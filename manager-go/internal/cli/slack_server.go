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
	utils.ConfigureLogging(*verbose)

	cfg := jctx.Config
	if cfg.SlackClientID == "" || cfg.SlackClientSecret == "" {
		return errors.New("missing slack client credentials (set slack.client_id and slack.client_secret)")
	}
	if cfg.SlackSigningSecret == "" {
		return errors.New("missing slack signing secret (set slack.signing_secret)")
	}

	scopes := strings.TrimSpace(cfg.SlackScopes)
	if scopes == "" {
		// Include channels:history so we can receive and handle message.channels events for thread follow-ups.
		scopes = "chat:write,channels:read,channels:join,channels:history,app_mentions:read"
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

	client := &http.Client{
		Timeout:   20 * time.Second,
		Transport: loggingRoundTripper{base: http.DefaultTransport},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/slack/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("ok\n"))
	})
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
		utils.Debug("slack install redirect")
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

		utils.Info("slack installed", "team_id", resp.Team.ID, "team_name", resp.Team.Name)
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
			utils.Warn("slack events signature verify failed", "err", err)
			http.Error(w, "invalid signature", http.StatusUnauthorized)
			return
		}

		var envelope slackEventEnvelope
		if err := json.Unmarshal(body, &envelope); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}

		utils.Debug("slack events", "envelope_type", envelope.Type, "team_id", envelope.TeamID, "event_type", envelope.Event.Type)

		// Optional legacy token check.
		if cfg.SlackVerificationToken != "" && envelope.Token != "" && envelope.Token != cfg.SlackVerificationToken {
			utils.Warn("slack events legacy token mismatch", "team_id", envelope.TeamID)
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

			teamID := envelope.TeamID
			channel := envelope.Event.Channel
			original := envelope.Event.Text
			switch envelope.Event.Type {
			case "app_mention":
				// Always reply in-thread:
				// - If the mention happened inside an existing thread, Slack provides thread_ts (parent thread).
				// - Otherwise use the event ts to start a new thread off the mention.
				threadTS := envelope.Event.ThreadTS
				if threadTS == "" {
					threadTS = envelope.Event.TS
				}
				activatedBy := envelope.Event.User
				go func() {
					// Mark this thread active so subsequent replies in-thread don't need an explicit @mention.
					if err := jctx.Store.UpsertSlackThreadSession(context.Background(), teamID, channel, threadTS, activatedBy, 24*time.Hour); err != nil {
						// Migration might not be applied yet; degrade gracefully.
						utils.Warn("slack thread session upsert failed", "team_id", teamID, "channel", channel, "thread_ts", threadTS, "err", err)
					}

					token, err := jctx.Store.GetSlackBotToken(context.Background(), teamID)
					if err != nil || token == "" {
						utils.Warn("slack no bot token", "team_id", teamID, "err", err)
						return
					}

					clean := slackStripLeadingMention(original)
					words := slackCountWords(clean)
					chars := utf8.RuneCountInString(clean)
					reply := fmt.Sprintf("words=%d chars=%d", words, chars)
					if err := slack.PostMessage(context.Background(), client, token, channel, reply, threadTS); err != nil {
						utils.Warn("slack post message failed", "team_id", teamID, "channel", channel, "err", err)
					}
				}()
				return
			case "message":
				// Ignore message events that aren't threaded. This keeps behavior "sticky" to a thread once activated.
				if envelope.Event.ThreadTS == "" {
					return
				}
				// Ignore message subtypes (edits, joins, bot_message, etc.).
				if envelope.Event.Subtype != "" {
					return
				}
				// Ignore bot messages (including our own) to avoid loops.
				if envelope.Event.BotID != "" {
					return
				}

				threadTS := envelope.Event.ThreadTS
				go func() {
					active, err := jctx.Store.IsSlackThreadSessionActive(context.Background(), teamID, channel, threadTS)
					if err != nil {
						// Migration might not be applied yet; degrade gracefully.
						utils.Warn("slack thread session check failed", "team_id", teamID, "channel", channel, "thread_ts", threadTS, "err", err)
						return
					}
					if !active {
						return
					}
					// Extend TTL on activity.
					_ = jctx.Store.UpsertSlackThreadSession(context.Background(), teamID, channel, threadTS, "", 24*time.Hour)

					token, err := jctx.Store.GetSlackBotToken(context.Background(), teamID)
					if err != nil || token == "" {
						utils.Warn("slack no bot token", "team_id", teamID, "err", err)
						return
					}

					clean := strings.TrimSpace(original)
					words := slackCountWords(clean)
					chars := utf8.RuneCountInString(clean)
					reply := fmt.Sprintf("words=%d chars=%d", words, chars)
					if err := slack.PostMessage(context.Background(), client, token, channel, reply, threadTS); err != nil {
						utils.Warn("slack post message failed", "team_id", teamID, "channel", channel, "err", err)
					}
				}()
				return
			default:
				return
			}
		default:
			w.WriteHeader(http.StatusOK)
			return
		}
	})

	server := &http.Server{
		Addr:              *listen,
		Handler:           httpLoggingMiddleware(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		utils.Info("slack server listen", "listen", *listen, "redirect_url", redirectURL)
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
		Subtype string `json:"subtype"`
		Channel string `json:"channel"`
		User    string `json:"user"`
		BotID   string `json:"bot_id"`
		// ChannelType is present for message events (e.g. "channel", "group", "im").
		ChannelType string `json:"channel_type"`
		Text        string `json:"text"`
		TS          string `json:"ts"`
		ThreadTS    string `json:"thread_ts"`
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

type loggingResponseWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (w *loggingResponseWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *loggingResponseWriter) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(p)
	w.bytes += n
	return n, err
}

func httpLoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !utils.Verbose {
			next.ServeHTTP(w, r)
			return
		}
		start := time.Now()
		lrw := &loggingResponseWriter{ResponseWriter: w}
		next.ServeHTTP(lrw, r)
		dur := time.Since(start)
		status := lrw.status
		if status == 0 {
			status = http.StatusOK
		}
		utils.Debug(
			"http request",
			"method", r.Method,
			"path", r.URL.Path,
			"host", r.Host,
			"status", status,
			"bytes", lrw.bytes,
			"dur", dur.Truncate(time.Millisecond).String(),
			"remote", r.RemoteAddr,
			"ua", r.UserAgent(),
		)
	})
}

type loggingRoundTripper struct {
	base http.RoundTripper
}

func (t loggingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	if !utils.Verbose {
		return base.RoundTrip(req)
	}
	start := time.Now()
	resp, err := base.RoundTrip(req)
	dur := time.Since(start)
	if err != nil {
		utils.Warn("http outbound error", "method", req.Method, "url", req.URL.Redacted(), "dur", dur.Truncate(time.Millisecond).String(), "err", err)
		return nil, err
	}
	// Never log request headers/body (may contain secrets). Only method/url/status.
	utils.Debug("http outbound", "method", req.Method, "url", req.URL.Redacted(), "status", resp.StatusCode, "dur", dur.Truncate(time.Millisecond).String())
	return resp, nil
}
