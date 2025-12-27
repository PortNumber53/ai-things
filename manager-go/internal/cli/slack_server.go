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
	"path/filepath"
	"strconv"
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
		// Include files:read so we can download images uploaded to threads (url_private).
		// Include reactions:read so we can receive reaction_added events used for approvals.
		scopes = "chat:write,channels:read,channels:join,channels:history,app_mentions:read,files:read,reactions:read"
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

	// Base URL used for watch links (prefer app.public_url, then --public-url, then infer from redirect URL).
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.PublicURL), "/")
	if baseURL == "" {
		baseURL = strings.TrimRight(strings.TrimSpace(*publicURL), "/")
	}
	if baseURL == "" && strings.HasSuffix(redirectURL, "/slack/oauth/callback") {
		baseURL = strings.TrimSuffix(redirectURL, "/slack/oauth/callback")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/watch/podcast/", func(w http.ResponseWriter, r *http.Request) {
		// Token-protected watch endpoint for reviewing rendered videos.
		// URL patterns:
		// - /watch/podcast/{id}?token=...
		// - /watch/podcast/{id}.mp4?token=...
		if cfg.SlackSigningSecret == "" {
			http.Error(w, "server missing slack signing secret", http.StatusInternalServerError)
			return
		}
		token := strings.TrimSpace(r.URL.Query().Get("token"))
		if token == "" {
			http.Error(w, "missing token", http.StatusUnauthorized)
			return
		}

		suffix := strings.TrimPrefix(r.URL.Path, "/watch/podcast/")
		if suffix == "" {
			http.Error(w, "missing id", http.StatusBadRequest)
			return
		}
		isMP4 := strings.HasSuffix(suffix, ".mp4")
		if isMP4 {
			suffix = strings.TrimSuffix(suffix, ".mp4")
		}
		id, err := strconv.ParseInt(suffix, 10, 64)
		if err != nil || id <= 0 {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		expected := youtubeWatchToken(cfg.SlackSigningSecret, id)
		if subtle.ConstantTimeCompare([]byte(expected), []byte(token)) != 1 {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}

		videoPath := filepath.Join(cfg.BaseOutputFolder, "podcast", fmt.Sprintf("%010d.mp4", id))
		ensureLocal := func() error {
			// Always consult DB meta so we can validate checksum (and fetch the correct artifact if local is stale).
			content, err := jctx.Store.GetContentByID(r.Context(), id)
			if err != nil {
				return err
			}
			meta, err := utils.DecodeMeta(content.Meta)
			if err != nil {
				return err
			}
			podcast, _ := meta["podcast"].(map[string]any)
			if podcast == nil {
				return errors.New("podcast meta missing")
			}
			remoteHost, _ := podcast["hostname"].(string)
			wantSHA, _ := podcast["sha256"].(string)

			// If we have a local file, validate it against the expected checksum (when recorded).
			if utils.FileExists(videoPath) {
				if strings.TrimSpace(wantSHA) == "" {
					return nil
				}
				haveSHA, err := utils.SHA256File(videoPath)
				if err != nil {
					return err
				}
				if haveSHA == wantSHA {
					return nil
				}
				// Local file exists but is not the expected artifact; keep it around for debugging and refetch.
				backupPath := fmt.Sprintf("%s.bad.%s", videoPath, haveSHA)
				_ = os.Rename(videoPath, backupPath)
			}

			// Try to fetch from the render host recorded in meta.podcast.hostname.
			if strings.TrimSpace(remoteHost) == "" {
				if strings.TrimSpace(wantSHA) != "" {
					return fmt.Errorf("podcast file missing or stale locally (want_sha=%s) and no render host recorded; cannot fetch", wantSHA)
				}
				return errors.New("podcast file missing locally and no render host recorded; cannot fetch")
			}

			// Ensure output dir exists.
			if err := utils.EnsureDir(filepath.Dir(videoPath)); err != nil {
				return err
			}

			cmd := fmt.Sprintf("rsync -ravp --progress %s:%s %s", remoteHost, utils.ShellEscape(videoPath), utils.ShellEscape(videoPath))
			output, err := utils.RunCommand(cmd)
			if err != nil {
				lower := strings.ToLower(output)
				if strings.Contains(lower, "no such file") ||
					strings.Contains(lower, "cannot stat") ||
					strings.Contains(lower, "could not resolve hostname") ||
					strings.Contains(lower, "connection unexpectedly closed") {
					return fmt.Errorf("remote podcast missing/unreachable: %s", strings.TrimSpace(output))
				}
				return err
			}
			if !utils.FileExists(videoPath) {
				return errors.New("fetch finished but file still missing")
			}
			if wantSHA != "" {
				haveSHA, err := utils.SHA256File(videoPath)
				if err != nil {
					return err
				}
				if haveSHA != wantSHA {
					return fmt.Errorf("checksum mismatch after fetch (want=%s have=%s)", wantSHA, haveSHA)
				}
			}
			return nil
		}

		if err := ensureLocal(); err != nil {
			utils.Warn("watch fetch failed", "content_id", id, "err", err)
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		if isMP4 {
			w.Header().Set("Content-Type", "video/mp4")
			w.Header().Set("Cache-Control", "no-store")
			http.ServeFile(w, r, videoPath)
			return
		}

		// Simple HTML wrapper so browsers show an inline player.
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		mp4URL := fmt.Sprintf("/watch/podcast/%d.mp4?token=%s", id, token)
		if strings.TrimSpace(baseURL) != "" {
			mp4URL = fmt.Sprintf("%s/watch/podcast/%d.mp4?token=%s", baseURL, id, token)
		}
		_, _ = w.Write([]byte(fmt.Sprintf(`<!doctype html>
<html>
  <head><meta charset="utf-8"><title>Podcast %010d</title></head>
  <body style="font-family: sans-serif; padding: 24px;">
    <h2>Podcast %010d</h2>
    <video controls style="width: min(960px, 100%%);">
      <source src="%s" type="video/mp4">
      Your browser does not support the video tag.
    </video>
    <p><a href="%s">Direct MP4 link</a></p>
  </body>
</html>`, id, id, mp4URL, mp4URL)))
	})

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

		utils.Debug(
			"slack events",
			"envelope_type", envelope.Type,
			"team_id", envelope.TeamID,
			"event_type", envelope.Event.Type,
			"subtype", envelope.Event.Subtype,
			"thread_ts", envelope.Event.ThreadTS,
			"files", len(envelope.Event.Files),
		)

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
				// Ignore bot messages (including our own) to avoid loops.
				if envelope.Event.BotID != "" {
					return
				}

				threadTS := envelope.Event.ThreadTS
				go func() {
					// First: handle Slack-driven image workflow (upload image to the thread).
					if len(envelope.Event.Files) > 0 {
						if handled := handleSlackImageUpload(
							context.Background(),
							jctx,
							client,
							teamID,
							channel,
							threadTS,
							envelope.Event.TS,
							envelope.Event.Files,
						); handled {
							return
						}
					}

					// Ignore message subtypes (edits, joins, file_share, etc.) for the non-image logic below.
					// File uploads are handled above.
					if envelope.Event.Subtype != "" {
						return
					}

					// Allow YouTube review approvals via plain text in the thread (fallback if reaction_added isn't delivered).
					if handled := handleSlackYouTubeReviewTextDecision(
						context.Background(),
						jctx,
						client,
						teamID,
						channel,
						threadTS,
						strings.TrimSpace(original),
						envelope.Event.User,
					); handled {
						return
					}

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
			case "reaction_added":
				// Handle approvals/rejections for YouTube review threads.
				itemTS := envelope.Event.Item.TS
				itemChannel := envelope.Event.Item.Channel
				reaction := envelope.Event.Reaction
				userID := envelope.Event.User
				if itemTS == "" || itemChannel == "" || reaction == "" {
					return
				}
				go func() {
					handleSlackYouTubeReviewReaction(
						context.Background(),
						jctx,
						client,
						teamID,
						itemChannel,
						itemTS,
						reaction,
						userID,
					)
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
		// Reaction events
		Reaction string `json:"reaction"`
		Item     struct {
			Type    string `json:"type"`
			Channel string `json:"channel"`
			TS      string `json:"ts"`
		} `json:"item"`
		Files       []struct {
			ID         string `json:"id"`
			Name       string `json:"name"`
			Mimetype   string `json:"mimetype"`
			Filetype   string `json:"filetype"`
			URLPrivate string `json:"url_private"`
		} `json:"files"`
	} `json:"event"`
}

func youtubeWatchToken(secret string, contentID int64) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(fmt.Sprintf("watch:%d", contentID)))
	return hex.EncodeToString(mac.Sum(nil))
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

func handleSlackYouTubeReviewReaction(
	ctx context.Context,
	jctx jobs.JobContext,
	client *http.Client,
	teamID string,
	channelID string,
	messageTS string,
	reaction string,
	userID string,
) {
	r := strings.TrimSpace(reaction)
	var decision string
	switch r {
	case "thumbsup", "+1":
		decision = "approved"
	case "thumbsdown", "-1":
		decision = "rejected"
	default:
		return
	}

	content, err := jctx.Store.FindContentBySlackYouTubeReviewThread(ctx, teamID, channelID, messageTS)
	if err != nil {
		utils.Warn("slack youtube review lookup failed", "team_id", teamID, "channel", channelID, "ts", messageTS, "err", err)
		return
	}
	if content.ID == 0 {
		return
	}

	token, err := jctx.Store.GetSlackBotToken(ctx, teamID)
	if err != nil || token == "" {
		utils.Warn("slack no bot token", "team_id", teamID, "err", err)
		return
	}

	meta, err := utils.DecodeMeta(content.Meta)
	if err != nil {
		utils.Warn("slack youtube review decode meta failed", "content_id", content.ID, "err", err)
		return
	}

	// Update request record (best-effort; tolerate missing structure).
	updateReq := func(m map[string]any) {
		m["decision"] = decision
		m["decided_at"] = time.Now().Format(time.RFC3339)
		m["decided_by_user_id"] = userID
		m["decided_reaction"] = r
	}
	if req, ok := utils.GetMap(meta, "slack_youtube_review_request"); ok {
		updateReq(req)
		meta["slack_youtube_review_request"] = req
	}
	if history, ok := meta["slack_youtube_review_requests"].([]any); ok && history != nil {
		for i := range history {
			m, _ := history[i].(map[string]any)
			if m == nil {
				continue
			}
			if ts, _ := m["thread_ts"].(string); ts != "" && ts == messageTS {
				updateReq(m)
				history[i] = m
				break
			}
			if ts, _ := m["link_ts"].(string); ts != "" && ts == messageTS {
				updateReq(m)
				history[i] = m
				break
			}
		}
		meta["slack_youtube_review_requests"] = history
	}

	var statusKey string
	var reply string
	switch decision {
	case "approved":
		statusKey = "youtube_approved"
		utils.SetStatus(meta, "youtube_approved", true)
		utils.SetStatus(meta, "youtube_rejected", false)
		reply = "Approved for YouTube upload. Queued."
	case "rejected":
		statusKey = "youtube_rejected"
		utils.SetStatus(meta, "youtube_approved", false)
		utils.SetStatus(meta, "youtube_rejected", true)
		reply = "Rejected for YouTube upload."
	default:
		return
	}

	if err := jctx.Store.UpdateContentMetaStatus(ctx, content.ID, statusKey, meta); err != nil {
		utils.Warn("slack youtube review update failed", "content_id", content.ID, "err", err)
		return
	}

	// Publish to approval queue so UploadYouTube workers can proceed (host-agnostic payload).
	if decision == "approved" && jctx.Queue != nil {
		payload, _ := json.Marshal(jobs.QueuePayload{ContentID: content.ID, Hostname: ""})
		if err := jctx.Queue.Publish("youtube_approved", payload); err != nil {
			utils.Warn("slack youtube approved publish failed", "content_id", content.ID, "err", err)
		}
	}

	// Best-effort confirmation message (in-thread). We try to reply to the root thread if we have it.
	threadTS := messageTS
	if req, ok := utils.GetMap(meta, "slack_youtube_review_request"); ok {
		if ts, _ := req["thread_ts"].(string); ts != "" {
			threadTS = ts
		}
	}
	if err := slack.PostMessage(ctx, client, token, channelID, reply, threadTS); err != nil {
		utils.Warn("slack youtube review reply failed", "content_id", content.ID, "channel", channelID, "err", err)
	}
}

func handleSlackYouTubeReviewTextDecision(
	ctx context.Context,
	jctx jobs.JobContext,
	client *http.Client,
	teamID string,
	channelID string,
	threadTS string,
	text string,
	userID string,
) bool {
	t := strings.ToLower(strings.TrimSpace(text))
	if t == "" {
		return false
	}

	approve := map[string]bool{
		"approve":  true,
		"approved": true,
		"yes":      true,
		"y":        true,
		"ship it":  true,
		"shipit":   true,
		"+1":       true,
		"👍":        true,
	}
	reject := map[string]bool{
		"reject":   true,
		"rejected": true,
		"no":       true,
		"n":        true,
		"redo":     true,
		"-1":       true,
		"👎":        true,
	}

	var reaction string
	switch {
	case approve[t]:
		reaction = "thumbsup"
	case reject[t]:
		reaction = "thumbsdown"
	default:
		return false
	}

	// Only handle if this thread corresponds to a review request.
	content, err := jctx.Store.FindContentBySlackYouTubeReviewThread(ctx, teamID, channelID, threadTS)
	if err != nil {
		utils.Warn("slack youtube review lookup failed", "team_id", teamID, "channel", channelID, "ts", threadTS, "err", err)
		return false
	}
	if content.ID == 0 {
		return false
	}

	// Apply the same logic as reaction approvals, using the parent thread ts as the item ts.
	handleSlackYouTubeReviewReaction(ctx, jctx, client, teamID, channelID, threadTS, reaction, userID)
	return true
}

func handleSlackImageUpload(
	ctx context.Context,
	jctx jobs.JobContext,
	client *http.Client,
	teamID, channelID, threadTS, messageTS string,
	files []struct {
		ID         string `json:"id"`
		Name       string `json:"name"`
		Mimetype   string `json:"mimetype"`
		Filetype   string `json:"filetype"`
		URLPrivate string `json:"url_private"`
	},
) bool {
	if teamID == "" || channelID == "" || threadTS == "" || len(files) == 0 {
		return false
	}

	// Only accept common image types.
	pick := func() (string, string, string, bool) {
		for _, f := range files {
			ft := strings.ToLower(strings.TrimSpace(f.Filetype))
			if ft == "" {
				ft = strings.ToLower(strings.TrimSpace(f.Mimetype))
			}
			switch ft {
			case "jpg", "jpeg", "image/jpeg":
				return "jpg", f.Name, f.URLPrivate, strings.TrimSpace(f.URLPrivate) != ""
			case "png", "image/png":
				return "png", f.Name, f.URLPrivate, strings.TrimSpace(f.URLPrivate) != ""
			case "webp", "image/webp":
				return "webp", f.Name, f.URLPrivate, strings.TrimSpace(f.URLPrivate) != ""
			}
		}
		return "", "", "", false
	}

	ext, origName, urlPrivate, ok := pick()
	if !ok {
		utils.Debug("slack image: files present but none supported", "team_id", teamID, "channel", channelID, "thread_ts", threadTS, "files", len(files))
		return false
	}

	content, err := jctx.Store.FindContentBySlackImageThread(ctx, teamID, channelID, threadTS)
	if err != nil {
		utils.Warn("slack image: lookup failed", "team_id", teamID, "channel", channelID, "thread_ts", threadTS, "err", err)
		return true
	}
	if content.ID == 0 {
		// Not for us; let other handlers handle this thread.
		utils.Debug("slack image: no linked content for thread", "team_id", teamID, "channel", channelID, "thread_ts", threadTS)
		return false
	}

	token, err := jctx.Store.GetSlackBotToken(ctx, teamID)
	if err != nil || token == "" {
		utils.Warn("slack image: missing bot token", "team_id", teamID, "err", err)
		return true
	}
	if client == nil {
		client = http.DefaultClient
	}

	imgBytes, err := slack.DownloadFile(ctx, client, token, urlPrivate)
	if err != nil {
		utils.Warn("slack image: download failed", "content_id", content.ID, "url", urlPrivate, "err", err)
		_ = slack.PostMessage(ctx, client, token, channelID, "I couldn't download that image (missing `files:read` scope or token?).", threadTS)
		return true
	}

	filename := fmt.Sprintf("%010d.%s", content.ID, ext)
	fullPath := filepath.Join(jctx.Config.BaseOutputFolder, "images", filename)
	if err := utils.EnsureDir(filepath.Dir(fullPath)); err != nil {
		utils.Warn("slack image: ensure dir failed", "content_id", content.ID, "err", err)
		return true
	}
	if err := os.WriteFile(fullPath, imgBytes, 0o644); err != nil {
		utils.Warn("slack image: write failed", "content_id", content.ID, "path", fullPath, "err", err)
		return true
	}

	meta, err := utils.DecodeMeta(content.Meta)
	if err != nil {
		utils.Warn("slack image: decode meta failed", "content_id", content.ID, "err", err)
		return true
	}
	meta["thumbnail"] = map[string]any{
		"filename": filename,
		"hostname": jctx.Config.Hostname,
		"sha256":   utils.SHA256Bytes(imgBytes),
		"source":   "slack",
		"original": origName,
	}
	utils.SetStatus(meta, "thumbnail_generated", true)
	utils.SetStatus(meta, "slack_image_requested", true)
	if req, ok := utils.GetMap(meta, "slack_image_request"); ok {
		req["completed"] = true
		req["completed_hostname"] = jctx.Config.Hostname
		req["completed_at"] = time.Now().Format(time.RFC3339)
		if strings.TrimSpace(messageTS) != "" {
			req["upload_ts"] = strings.TrimSpace(messageTS)
		}
		if len(files) > 0 && strings.TrimSpace(files[0].ID) != "" {
			req["file_id"] = strings.TrimSpace(files[0].ID)
		}
	} else {
		meta["slack_image_request"] = map[string]any{
			"team_id":            teamID,
			"channel_id":         channelID,
			"thread_ts":          threadTS,
			"completed":          true,
			"completed_hostname": jctx.Config.Hostname,
			"completed_at":       time.Now().Format(time.RFC3339),
			"upload_ts":          strings.TrimSpace(messageTS),
			"file_id":            strings.TrimSpace(files[0].ID),
		}
	}

	if err := jctx.Store.UpdateContentMetaStatus(ctx, content.ID, "thumbnail_generated", meta); err != nil {
		utils.Warn("slack image: db update failed", "content_id", content.ID, "err", err)
		return true
	}

	utils.Info("slack image saved", "content_id", content.ID, "path", fullPath)
	_ = slack.PostMessage(ctx, client, token, channelID, fmt.Sprintf("Saved image as %s and marked thumbnail_generated=true.", filename), threadTS)
	return true
}
