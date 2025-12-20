package cli

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	"ai-things/manager-go/internal/db"
	"ai-things/manager-go/internal/jobs"
	"ai-things/manager-go/internal/slack"
	"ai-things/manager-go/internal/utils"
)

func runSlackServe(ctx context.Context, jctx jobs.JobContext, args []string) error {
	fs := flag.NewFlagSet("Slack:Serve", flag.ContinueOnError)
	listen := fs.String("listen", "", "Listen address (host:port). Default is :{slack.port}.")
	publicURL := fs.String("public-url", "", "Public base URL (used to compute redirect if slack.redirect_url not set)")
	verbose := fs.Bool("verbose", utils.Verbose, "Verbose logging")
	if err := fs.Parse(args); err != nil {
		return err
	}
	utils.Verbose = *verbose

	cfg := jctx.Config
	if cfg.SlackClientID == "" || cfg.SlackClientSecret == "" {
		return errors.New("missing slack client credentials (set slack.client_id and slack.client_secret)")
	}
	if cfg.SlackSigningSecret == "" {
		return errors.New("missing slack signing secret (set slack.signing_secret)")
	}

	scopes := strings.TrimSpace(cfg.SlackScopes)
	if scopes == "" {
		scopes = "chat:write,channels:read,channels:join,app_mentions:read"
	}

	redirectURL := strings.TrimSpace(cfg.SlackRedirectURL)
	if redirectURL == "" && strings.TrimSpace(*publicURL) != "" {
		redirectURL = strings.TrimRight(strings.TrimSpace(*publicURL), "/") + "/slack/oauth/callback"
	}
	if redirectURL == "" {
		return errors.New("missing slack redirect URL (set slack.redirect_url or pass --public-url)")
	}

	if *listen == "" {
		port := cfg.SlackPort
		if port == 0 {
			port = 8085
		}
		*listen = fmt.Sprintf(":%d", port)
	}

	client := &http.Client{Timeout: 20 * time.Second}

	mux := http.NewServeMux()
	mux.HandleFunc("/slack/install", func(w http.ResponseWriter, r *http.Request) {
		state, err := slackMakeState(cfg.SlackSigningSecret)
		if err != nil {
			http.Error(w, "failed to create state", http.StatusInternalServerError)
			return
		}
		authURL, err := slack.BuildOAuthAuthorizeURL(cfg.SlackClientID, redirectURL, scopes, state)
		if err != nil {
			http.Error(w, "failed to build install url", http.StatusInternalServerError)
			return
		}
		utils.Logf("slack: install redirect")
		http.Redirect(w, r, authURL, http.StatusFound)
	})

	mux.HandleFunc("/slack/oauth/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		state := r.URL.Query().Get("state")
		if code == "" {
			http.Error(w, "missing code", http.StatusBadRequest)
			return
		}
		if err := slackVerifyState(cfg.SlackSigningSecret, state); err != nil {
			http.Error(w, "invalid state", http.StatusUnauthorized)
			return
		}

		resp, err := slack.ExchangeOAuthCode(r.Context(), client, cfg.SlackClientID, cfg.SlackClientSecret, code, redirectURL)
		if err != nil {
			http.Error(w, "oauth failed", http.StatusBadRequest)
			return
		}

		if err := jctx.Store.UpsertSlackInstallation(r.Context(), db.SlackInstallation{
			TeamID:    resp.Team.ID,
			TeamName:  resp.Team.Name,
			BotUserID: resp.BotUserID,
			BotToken:  resp.AccessToken,
			Scope:     resp.Scope,
		}); err != nil {
			http.Error(w, "failed to store installation", http.StatusInternalServerError)
			return
		}

		utils.Logf("slack: installed team_id=%s team_name=%s", resp.Team.ID, resp.Team.Name)
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("Slack installed successfully. You can close this window.\n"))
	})

	mux.HandleFunc("/slack/events", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			http.Error(w, "read body failed", http.StatusBadRequest)
			return
		}

		if err := slack.VerifySignature(cfg.SlackSigningSecret, r.Header, body, time.Now()); err != nil {
			http.Error(w, "invalid signature", http.StatusUnauthorized)
			return
		}

		var envelope slackEventEnvelope
		if err := json.Unmarshal(body, &envelope); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}

		// Optional legacy token check.
		if cfg.SlackVerificationToken != "" && envelope.Token != "" && envelope.Token != cfg.SlackVerificationToken {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}

		switch envelope.Type {
		case "url_verification":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"challenge": envelope.Challenge})
			return
		case "event_callback":
			// ACK quickly to avoid Slack retries.
			w.WriteHeader(http.StatusOK)

			if envelope.TeamID == "" || envelope.Event.Type == "" {
				return
			}
			if envelope.Event.Type != "app_mention" {
				return
			}

			teamID := envelope.TeamID
			channel := envelope.Event.Channel
			threadTS := envelope.Event.TS
			original := envelope.Event.Text
			go func() {
				token, err := jctx.Store.GetSlackBotToken(context.Background(), teamID)
				if err != nil || token == "" {
					utils.Logf("slack: no bot token for team_id=%s", teamID)
					return
				}

				clean := slackStripLeadingMention(original)
				words := slackCountWords(clean)
				chars := utf8.RuneCountInString(clean)
				reply := fmt.Sprintf("words=%d chars=%d", words, chars)
				_ = slack.PostMessage(context.Background(), client, token, channel, reply, threadTS)
			}()
			return
		default:
			w.WriteHeader(http.StatusOK)
			return
		}
	})

	server := &http.Server{
		Addr:              *listen,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		utils.Logf("Slack:Serve: listen=%s redirect_url=%s", *listen, redirectURL)
		errCh <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
		return nil
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

type slackEventEnvelope struct {
	Type      string `json:"type"`
	Token     string `json:"token"`
	Challenge string `json:"challenge"`
	TeamID    string `json:"team_id"`
	Event     struct {
		Type    string `json:"type"`
		Channel string `json:"channel"`
		Text    string `json:"text"`
		TS      string `json:"ts"`
	} `json:"event"`
}

func slackMakeState(secret string) (string, error) {
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	payload, err := json.Marshal(map[string]any{
		"ts":    time.Now().Unix(),
		"nonce": hex.EncodeToString(nonce),
	})
	if err != nil {
		return "", err
	}
	payloadB64 := base64.RawURLEncoding.EncodeToString(payload)

	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(payloadB64))
	sig := hex.EncodeToString(mac.Sum(nil))
	return payloadB64 + "." + sig, nil
}

func slackVerifyState(secret, state string) error {
	parts := strings.Split(state, ".")
	if len(parts) != 2 {
		return errors.New("invalid state format")
	}
	payloadB64, sig := parts[0], parts[1]

	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(payloadB64))
	expected := hex.EncodeToString(mac.Sum(nil))
	if subtle.ConstantTimeCompare([]byte(expected), []byte(sig)) != 1 {
		return errors.New("state signature mismatch")
	}

	raw, err := base64.RawURLEncoding.DecodeString(payloadB64)
	if err != nil {
		return errors.New("invalid state payload")
	}
	var decoded struct {
		TS int64 `json:"ts"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return errors.New("invalid state json")
	}
	ts := time.Unix(decoded.TS, 0)
	if time.Since(ts) > 15*time.Minute {
		return errors.New("state expired")
	}
	return nil
}

func slackStripLeadingMention(text string) string {
	// Slack app_mention events include something like: "<@U123ABC> hello world"
	// Remove the first mention token if present.
	s := strings.TrimSpace(text)
	if strings.HasPrefix(s, "<@") {
		if end := strings.Index(s, ">"); end > 0 {
			s = strings.TrimSpace(s[end+1:])
			s = strings.TrimLeft(s, " :,-\t")
		}
	}
	return strings.TrimSpace(s)
}

func slackCountWords(text string) int {
	// Use Fields to split on Unicode whitespace.
	return len(strings.Fields(strings.TrimSpace(text)))
}


