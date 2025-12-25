package slack

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	oauthAuthorizeURL      = "https://slack.com/oauth/v2/authorize"
	oauthAccessURL         = "https://slack.com/api/oauth.v2.access"
	chatPostMessageURL     = "https://slack.com/api/chat.postMessage"
	conversationsCreateURL = "https://slack.com/api/conversations.create"
	conversationsJoinURL   = "https://slack.com/api/conversations.join"
	conversationsListURL   = "https://slack.com/api/conversations.list"

	signatureVersion = "v0"
	maxClockSkew     = 5 * time.Minute
)

func FindChannelByName(ctx context.Context, client *http.Client, botToken, name string) (string, error) {
	if client == nil {
		client = http.DefaultClient
	}
	if strings.TrimSpace(botToken) == "" {
		return "", errors.New("bot token missing")
	}
	name = strings.TrimSpace(strings.TrimPrefix(name, "#"))
	if name == "" {
		return "", errors.New("channel name missing")
	}

	cursor := ""
	for i := 0; i < 20; i++ {
		u, _ := url.Parse(conversationsListURL)
		q := u.Query()
		q.Set("exclude_archived", "true")
		q.Set("limit", "200")
		// Most workspaces will allow listing public channels with channels:read.
		// If you want private channels too, the token must also have groups:read and we can add private_channel.
		q.Set("types", "public_channel")
		if cursor != "" {
			q.Set("cursor", cursor)
		}
		u.RawQuery = q.Encode()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("Authorization", "Bearer "+botToken)

		resp, err := client.Do(req)
		if err != nil {
			return "", err
		}
		respBody, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return "", fmt.Errorf("slack conversations.list status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(respBody)))
		}

		var decoded struct {
			OK       bool   `json:"ok"`
			Error    string `json:"error"`
			Needed   string `json:"needed"`
			Provided string `json:"provided"`
			Channels []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"channels"`
			ResponseMetadata struct {
				NextCursor string `json:"next_cursor"`
			} `json:"response_metadata"`
		}
		if err := json.Unmarshal(respBody, &decoded); err != nil {
			return "", err
		}
		if !decoded.OK {
			if decoded.Error == "" {
				decoded.Error = "conversations.list failed"
			}
			if strings.TrimSpace(decoded.Needed) != "" || strings.TrimSpace(decoded.Provided) != "" {
				return "", fmt.Errorf("%s (needed=%s provided=%s)", decoded.Error, strings.TrimSpace(decoded.Needed), strings.TrimSpace(decoded.Provided))
			}
			return "", errors.New(decoded.Error)
		}

		for _, ch := range decoded.Channels {
			if strings.EqualFold(ch.Name, name) && ch.ID != "" {
				return ch.ID, nil
			}
		}

		cursor = strings.TrimSpace(decoded.ResponseMetadata.NextCursor)
		if cursor == "" {
			break
		}
	}
	return "", nil
}

func CreateChannel(ctx context.Context, client *http.Client, botToken, name string, isPrivate bool) (string, error) {
	if client == nil {
		client = http.DefaultClient
	}
	if strings.TrimSpace(botToken) == "" {
		return "", errors.New("bot token missing")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errors.New("channel name missing")
	}

	payload := map[string]any{
		"name":       name,
		"is_private": isPrivate,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, conversationsCreateURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+botToken)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("slack conversations.create status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var decoded struct {
		OK       bool   `json:"ok"`
		Error    string `json:"error"`
		Needed   string `json:"needed"`
		Provided string `json:"provided"`
		Channel  struct {
			ID string `json:"id"`
		} `json:"channel"`
	}
	if err := json.Unmarshal(respBody, &decoded); err != nil {
		return "", err
	}
	if !decoded.OK {
		if decoded.Error == "" {
			decoded.Error = "conversations.create failed"
		}
		// Slack includes helpful fields like "needed" and "provided" for missing_scope.
		if strings.TrimSpace(decoded.Needed) != "" || strings.TrimSpace(decoded.Provided) != "" {
			return "", fmt.Errorf("%s (needed=%s provided=%s)", decoded.Error, strings.TrimSpace(decoded.Needed), strings.TrimSpace(decoded.Provided))
		}
		return "", errors.New(decoded.Error)
	}
	if decoded.Channel.ID == "" {
		return "", errors.New("conversations.create missing channel.id")
	}
	return decoded.Channel.ID, nil
}

func JoinChannel(ctx context.Context, client *http.Client, botToken, channelID string) error {
	if client == nil {
		client = http.DefaultClient
	}
	if strings.TrimSpace(botToken) == "" {
		return errors.New("bot token missing")
	}
	if strings.TrimSpace(channelID) == "" {
		return errors.New("channel id missing")
	}

	payload := map[string]any{
		"channel": channelID,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, conversationsJoinURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+botToken)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("slack conversations.join status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	var decoded struct {
		OK       bool   `json:"ok"`
		Error    string `json:"error"`
		Needed   string `json:"needed"`
		Provided string `json:"provided"`
	}
	if err := json.Unmarshal(respBody, &decoded); err == nil {
		if !decoded.OK {
			if decoded.Error == "" {
				decoded.Error = "conversations.join failed"
			}
			if strings.TrimSpace(decoded.Needed) != "" || strings.TrimSpace(decoded.Provided) != "" {
				return fmt.Errorf("%s (needed=%s provided=%s)", decoded.Error, strings.TrimSpace(decoded.Needed), strings.TrimSpace(decoded.Provided))
			}
			return errors.New(decoded.Error)
		}
	}
	return nil
}

func DownloadFile(ctx context.Context, client *http.Client, botToken, url string) ([]byte, error) {
	if client == nil {
		client = http.DefaultClient
	}
	if strings.TrimSpace(botToken) == "" {
		return nil, errors.New("bot token missing")
	}
	if strings.TrimSpace(url) == "" {
		return nil, errors.New("url missing")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+botToken)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("slack file download status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return body, nil
}

type OAuthAccessResponse struct {
	OK          bool   `json:"ok"`
	Error       string `json:"error"`
	AccessToken string `json:"access_token"`
	Scope       string `json:"scope"`
	BotUserID   string `json:"bot_user_id"`
	Team        struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"team"`
}

func BuildOAuthAuthorizeURL(clientID, redirectURL, scopes, state string) (string, error) {
	if clientID == "" {
		return "", errors.New("clientID is required")
	}
	u, err := url.Parse(oauthAuthorizeURL)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("client_id", clientID)
	if scopes != "" {
		q.Set("scope", scopes)
	}
	if redirectURL != "" {
		q.Set("redirect_uri", redirectURL)
	}
	if state != "" {
		q.Set("state", state)
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func ExchangeOAuthCode(ctx context.Context, client *http.Client, clientID, clientSecret, code, redirectURL string) (OAuthAccessResponse, error) {
	if client == nil {
		client = http.DefaultClient
	}
	if clientID == "" || clientSecret == "" {
		return OAuthAccessResponse{}, errors.New("client_id and client_secret are required")
	}
	if code == "" {
		return OAuthAccessResponse{}, errors.New("code is required")
	}

	values := url.Values{}
	values.Set("client_id", clientID)
	values.Set("client_secret", clientSecret)
	values.Set("code", code)
	if redirectURL != "" {
		values.Set("redirect_uri", redirectURL)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, oauthAccessURL, strings.NewReader(values.Encode()))
	if err != nil {
		return OAuthAccessResponse{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return OAuthAccessResponse{}, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return OAuthAccessResponse{}, fmt.Errorf("slack oauth status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var decoded OAuthAccessResponse
	if err := json.Unmarshal(body, &decoded); err != nil {
		return OAuthAccessResponse{}, err
	}
	if !decoded.OK {
		if decoded.Error == "" {
			decoded.Error = "oauth failed"
		}
		return decoded, errors.New(decoded.Error)
	}
	if decoded.AccessToken == "" {
		return decoded, errors.New("oauth response missing access_token")
	}
	if decoded.Team.ID == "" {
		return decoded, errors.New("oauth response missing team.id")
	}
	return decoded, nil
}

func VerifySignature(signingSecret string, headers http.Header, body []byte, now time.Time) error {
	if strings.TrimSpace(signingSecret) == "" {
		return errors.New("slack signing secret missing")
	}

	ts := headers.Get("X-Slack-Request-Timestamp")
	sig := headers.Get("X-Slack-Signature")
	if ts == "" || sig == "" {
		return errors.New("missing slack signature headers")
	}

	parsedTS, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return errors.New("invalid slack timestamp")
	}
	t := time.Unix(parsedTS, 0)
	if now.IsZero() {
		now = time.Now()
	}
	if now.Sub(t) > maxClockSkew || t.Sub(now) > maxClockSkew {
		return errors.New("slack timestamp outside allowed window")
	}

	base := fmt.Sprintf("%s:%s:%s", signatureVersion, ts, string(body))
	mac := hmac.New(sha256.New, []byte(signingSecret))
	_, _ = mac.Write([]byte(base))
	expected := signatureVersion + "=" + hex.EncodeToString(mac.Sum(nil))

	if subtle.ConstantTimeCompare([]byte(expected), []byte(sig)) != 1 {
		return errors.New("invalid slack signature")
	}
	return nil
}

func PostMessage(ctx context.Context, client *http.Client, botToken, channel, text, threadTS string) error {
	if client == nil {
		client = http.DefaultClient
	}
	if botToken == "" {
		return errors.New("bot token missing")
	}
	if channel == "" {
		return errors.New("channel missing")
	}
	if text == "" {
		return errors.New("text missing")
	}

	payload := map[string]any{
		"channel": channel,
		"text":    text,
	}
	if threadTS != "" {
		payload["thread_ts"] = threadTS
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, chatPostMessageURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+botToken)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("slack chat.postMessage status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var decoded struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(respBody, &decoded); err == nil {
		if !decoded.OK {
			if decoded.Error == "" {
				decoded.Error = "chat.postMessage failed"
			}
			return errors.New(decoded.Error)
		}
	}
	return nil
}

// PostMessageWithTS posts a message and returns the created message ts.
// If threadTS is empty, the returned ts can be used as a thread root.
func PostMessageWithTS(ctx context.Context, client *http.Client, botToken, channel, text, threadTS string) (string, error) {
	if client == nil {
		client = http.DefaultClient
	}
	if strings.TrimSpace(botToken) == "" {
		return "", errors.New("bot token missing")
	}
	if strings.TrimSpace(channel) == "" {
		return "", errors.New("channel missing")
	}
	if strings.TrimSpace(text) == "" {
		return "", errors.New("text missing")
	}

	payload := map[string]any{
		"channel": channel,
		"text":    text,
	}
	if threadTS != "" {
		payload["thread_ts"] = threadTS
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, chatPostMessageURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+botToken)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("slack chat.postMessage status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var decoded struct {
		OK      bool   `json:"ok"`
		Error   string `json:"error"`
		TS      string `json:"ts"`
		Message struct {
			TS string `json:"ts"`
		} `json:"message"`
	}
	if err := json.Unmarshal(respBody, &decoded); err != nil {
		return "", err
	}
	if !decoded.OK {
		if decoded.Error == "" {
			decoded.Error = "chat.postMessage failed"
		}
		return "", errors.New(decoded.Error)
	}
	if decoded.TS != "" {
		return decoded.TS, nil
	}
	if decoded.Message.TS != "" {
		return decoded.Message.TS, nil
	}
	return "", nil
}
