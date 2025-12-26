package jobs

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"ai-things/manager-go/internal/db"
	"ai-things/manager-go/internal/slack"
	"ai-things/manager-go/internal/utils"
)

// SlackReviewPodcastJob posts a Slack thread with a watch link for the rendered podcast video.
// A separate Slack:Serve handler listens for thumbs-up/down reactions to approve/reject YouTube uploads.
type SlackReviewPodcastJob struct {
	BaseJob
	MaxWaiting int
}

func NewSlackReviewPodcastJob() SlackReviewPodcastJob {
	return SlackReviewPodcastJob{
		BaseJob: BaseJob{
			QueueInput:  "podcast_ready",
			QueueOutput: "youtube_review_requested",
		},
		MaxWaiting: 50,
	}
}

func (j SlackReviewPodcastJob) Run(ctx context.Context, jctx JobContext, opts JobOptions) error {
	if opts.Queue {
		return j.RunQueue(ctx, jctx, opts, func(ctx context.Context, contentID int64, hostname string) error {
			return j.processContent(ctx, jctx, contentID, opts.Regenerate)
		})
	}

	contentID := opts.ContentID
	if contentID == 0 {
		content, err := j.selectNext(ctx, jctx)
		if err != nil {
			return err
		}
		contentID = content.ID
	}

	return j.processContent(ctx, jctx, contentID, opts.Regenerate)
}

func (j SlackReviewPodcastJob) selectNext(ctx context.Context, jctx JobContext) (db.Content, error) {
	where := "WHERE type = 'gemini.payload'"
	ready := db.StatusTrueCondition([]string{"funfact_created", "wav_generated", "mp3_generated", "srt_generated", "thumbnail_generated", "podcast_ready"})
	notRequested := db.StatusNotTrueCondition([]string{j.QueueOutput})
	notUploaded := db.StatusNotTrueCondition([]string{"youtube_uploaded"})
	notApprovedOrRejected := db.StatusNotTrueCondition([]string{"youtube_approved", "youtube_rejected"})
	missing := db.MetaKeyMissingCondition([]string{"video_id.v1"})
	if ready != "" {
		where += " AND " + ready
	}
	if notRequested != "" {
		where += " AND " + notRequested
	}
	if notUploaded != "" {
		where += " AND " + notUploaded
	}
	if notApprovedOrRejected != "" {
		where += " AND " + notApprovedOrRejected
	}
	if missing != "" {
		where += " AND " + missing
	}

	content, err := jctx.Store.FindFirstContent(ctx, where)
	if err != nil {
		return db.Content{}, err
	}
	if content.ID == 0 {
		return db.Content{}, errors.New("no content to process")
	}
	return content, nil
}

func (j SlackReviewPodcastJob) processContent(ctx context.Context, jctx JobContext, contentID int64, regenerate bool) error {
	utils.Info("SlackReviewPodcast process", "content_id", contentID, "regenerate", regenerate)

	cfg := jctx.Config
	if strings.TrimSpace(cfg.PublicURL) == "" {
		return errors.New("missing app.public_url (needed to build watch links)")
	}
	if strings.TrimSpace(cfg.SlackSigningSecret) == "" {
		return errors.New("missing slack.signing_secret (needed to sign watch links)")
	}

	teamID := strings.TrimSpace(cfg.SlackTeamID)
	if teamID == "" {
		detectedTeamID, detectErr := jctx.Store.GetDefaultSlackTeamID(ctx)
		if detectErr != nil {
			return detectErr
		}
		teamID = detectedTeamID
		if teamID == "" {
			return errors.New("missing slack.team_id and no slack installation found in DB (run Slack:Serve install flow first, or set slack.team_id)")
		}
	}
	channelID := strings.TrimSpace(cfg.SlackImageChannel)
	if channelID == "" {
		dbChannel, chErr := jctx.Store.GetSlackImageChannel(ctx, teamID)
		if chErr != nil {
			return chErr
		}
		channelID = dbChannel
		if channelID == "" {
			return errors.New("missing slack.image_channel and no stored channel found in DB (run Slack:CreateImageChannel --name=ai-images or set slack.image_channel)")
		}
	}

	content, err := jctx.Store.GetContentByID(ctx, contentID)
	if err != nil {
		return err
	}
	meta, err := utils.DecodeMeta(content.Meta)
	if err != nil {
		return err
	}

	if !regenerate {
		if already, _ := utils.GetStatus(meta, j.QueueOutput); already {
			utils.Info("SlackReviewPodcast already requested; skipping", "content_id", contentID)
			return nil
		}
	}

	// Ensure we at least know how the watch endpoint can retrieve the file:
	// - Either it already exists locally, or we have a render host recorded in meta.podcast.hostname.
	filename := filepath.Join(cfg.BaseOutputFolder, "podcast", fmt.Sprintf("%010d.mp4", content.ID))
	if !utils.FileExists(filename) {
		podcast, _ := meta["podcast"].(map[string]any)
		if podcast == nil {
			return errors.New("podcast metadata missing")
		}
		if host, _ := podcast["hostname"].(string); strings.TrimSpace(host) == "" {
			return fmt.Errorf("podcast video missing locally and no render host recorded: %s", filename)
		}
	}

	watchURL := fmt.Sprintf("%s/watch/podcast/%d?token=%s", strings.TrimRight(cfg.PublicURL, "/"), content.ID, youtubeWatchToken(cfg.SlackSigningSecret, content.ID))

	token, err := jctx.Store.GetSlackBotToken(ctx, teamID)
	if err != nil || token == "" {
		return fmt.Errorf("missing slack bot token for team_id=%s (install the Slack app first)", teamID)
	}

	client := &http.Client{Timeout: 20 * time.Second}
	if err := slack.JoinChannel(ctx, client, token, channelID); err != nil {
		return fmt.Errorf("failed to join slack channel %s: %w", channelID, err)
	}

	label := fmt.Sprintf("%010d - %s", content.ID, strings.TrimSpace(content.Title))
	rootText := fmt.Sprintf("Review before YouTube upload:\n%s\n\nReact with :thumbsup: to approve or :thumbsdown: to reject.", label)
	threadTS, err := slack.PostMessageWithTS(ctx, client, token, channelID, rootText, "")
	if err != nil {
		return err
	}
	linkTS, err := slack.PostMessageWithTS(ctx, client, token, channelID, fmt.Sprintf("Watch: %s", watchURL), threadTS)
	if err != nil {
		return err
	}

	req := map[string]any{
		"team_id":     teamID,
		"channel_id":  channelID,
		"thread_ts":   threadTS,
		"watch_url":   watchURL,
		"link_ts":     linkTS,
		"hostname":    cfg.Hostname,
		"created_at":  time.Now().Format(time.RFC3339),
		"content_id":  content.ID,
		"content_tag": label,
	}

	history, _ := meta["slack_youtube_review_requests"].([]any)
	if history == nil {
		history = []any{}
	}
	if prev, ok := utils.GetMap(meta, "slack_youtube_review_request"); ok {
		prevThread, _ := prev["thread_ts"].(string)
		already := false
		for _, item := range history {
			m, _ := item.(map[string]any)
			if m == nil {
				continue
			}
			if ts, _ := m["thread_ts"].(string); ts != "" && ts == prevThread {
				already = true
				break
			}
		}
		if prevThread != "" && !already {
			history = append(history, prev)
		}
	}
	already := false
	for _, item := range history {
		m, _ := item.(map[string]any)
		if m == nil {
			continue
		}
		if ts, _ := m["thread_ts"].(string); ts != "" && ts == threadTS {
			already = true
			break
		}
	}
	if !already {
		history = append(history, req)
	}

	meta["slack_youtube_review_request"] = req
	meta["slack_youtube_review_requests"] = history
	utils.SetStatus(meta, j.QueueOutput, true)

	if err := jctx.Store.UpdateContentMetaStatus(ctx, content.ID, j.QueueOutput, meta); err != nil {
		return err
	}

	// Optional: publish to a queue for monitoring (approval is published later by Slack:Serve).
	if jctx.Queue != nil {
		payload, _ := json.Marshal(QueuePayload{ContentID: content.ID, Hostname: cfg.Hostname})
		_ = jctx.Queue.Publish(j.QueueOutput, payload)
	}
	return nil
}

func youtubeWatchToken(secret string, contentID int64) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(fmt.Sprintf("watch:%d", contentID)))
	return hex.EncodeToString(mac.Sum(nil))
}


