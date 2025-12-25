package jobs

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"ai-things/manager-go/internal/db"
	"ai-things/manager-go/internal/slack"
	"ai-things/manager-go/internal/utils"
)

// SlackPromptForImageJob requests an image via Slack by posting a message and replying with a generated prompt.
// A separate Slack:Serve handler listens for an uploaded image in the thread and finalizes thumbnail generation.
type SlackPromptForImageJob struct {
	BaseJob
	MaxWaiting int
}

func NewSlackPromptForImageJob() SlackPromptForImageJob {
	return SlackPromptForImageJob{
		BaseJob: BaseJob{
			QueueInput:      "generate_image", // same queue as PromptForImage
			QueueOutput:     "slack_image_requested",
			IgnoreHostCheck: true,
		},
		MaxWaiting: 100,
	}
}

func (j SlackPromptForImageJob) Run(ctx context.Context, jctx JobContext, opts JobOptions) error {
	if opts.Queue {
		return j.RunQueue(ctx, jctx, opts, func(ctx context.Context, contentID int64, hostname string) error {
			return j.processContent(ctx, jctx, contentID, opts.Regenerate)
		})
	}

	contentID := opts.ContentID
	if contentID == 0 {
		count, err := j.countWaiting(ctx, jctx)
		if err != nil {
			return err
		}
		// Unlike the pipeline jobs, this Slack job is user-interactive and rate-limited by humans.
		// We process at most ONE item per invocation, so a large backlog shouldn't cause us to sleep.
		utils.Debug("SlackPromptForImage backlog", "waiting", count)

		content, err := j.selectNext(ctx, jctx)
		if err != nil {
			return err
		}
		contentID = content.ID
	}

	return j.processContent(ctx, jctx, contentID, opts.Regenerate)
}

func (j SlackPromptForImageJob) countWaiting(ctx context.Context, jctx JobContext) (int, error) {
	where := "WHERE type = 'gemini.payload'"
	trueFlags := db.StatusTrueCondition([]string{"funfact_created"})
	notThumb := db.StatusNotTrueCondition([]string{"thumbnail_generated"})
	notRequested := db.StatusNotTrueCondition([]string{j.QueueOutput})
	if trueFlags != "" {
		where += " AND " + trueFlags
	}
	if notThumb != "" {
		where += " AND " + notThumb
	}
	if notRequested != "" {
		where += " AND " + notRequested
	}
	return jctx.Store.CountContent(ctx, where)
}

func (j SlackPromptForImageJob) selectNext(ctx context.Context, jctx JobContext) (db.Content, error) {
	where := "WHERE type = 'gemini.payload'"
	trueFlags := db.StatusTrueCondition([]string{"funfact_created"})
	notThumb := db.StatusNotTrueCondition([]string{"thumbnail_generated"})
	notRequested := db.StatusNotTrueCondition([]string{j.QueueOutput})
	if trueFlags != "" {
		where += " AND " + trueFlags
	}
	if notThumb != "" {
		where += " AND " + notThumb
	}
	if notRequested != "" {
		where += " AND " + notRequested
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

func (j SlackPromptForImageJob) processContent(ctx context.Context, jctx JobContext, contentID int64, regenerate bool) error {
	utils.Info("SlackPromptForImage process", "content_id", contentID, "regenerate", regenerate)

	cfg := jctx.Config
	teamID := strings.TrimSpace(cfg.SlackTeamID)
	if teamID == "" {
		// Prefer the team_id from the most recent Slack installation stored in the DB.
		detectedTeamID, detectErr := jctx.Store.GetDefaultSlackTeamID(ctx)
		if detectErr != nil {
			return detectErr
		}
		teamID = detectedTeamID
		if teamID == "" {
			return errors.New("missing slack.team_id and no slack installation found in DB (run Slack:Serve and install the app, or set slack.team_id)")
		}
	}
	channelID := strings.TrimSpace(cfg.SlackImageChannel)
	if channelID == "" {
		// Prefer DB-stored image channel created by Slack:CreateImageChannel.
		dbChannel, chErr := jctx.Store.GetSlackImageChannel(ctx, teamID)
		if chErr != nil {
			return chErr
		}
		channelID = dbChannel
		if channelID == "" {
			return errors.New("missing slack.image_channel and no stored image channel found in DB (run Slack:CreateImageChannel --name=ai-images or set slack.image_channel)")
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
			utils.Info("SlackPromptForImage already requested; skipping", "content_id", contentID)
			return nil
		}
	}

	token, err := jctx.Store.GetSlackBotToken(ctx, teamID)
	if err != nil || token == "" {
		return fmt.Errorf("missing slack bot token for team_id=%s (install the Slack app first)", teamID)
	}

	// Compose the user-facing “work item” message.
	label := fmt.Sprintf("%010d - %s", content.ID, strings.TrimSpace(content.Title))

	// Build the prompt we want the human to use when generating an image.
	text, err := utils.ExtractTextFromMeta(meta)
	if err != nil {
		return err
	}
	prompt := buildImagePrompt(text)

	client := &http.Client{Timeout: 20 * time.Second}

	// Ensure the bot is in the channel before posting (some workspaces require membership to post).
	// If this fails due to missing scopes or restrictions, fail loudly so we don't set slack_image_requested incorrectly.
	if err := slack.JoinChannel(ctx, client, token, channelID); err != nil {
		return fmt.Errorf("failed to join slack channel %s: %w", channelID, err)
	}

	// Post the main message, and capture its ts so we can reply in-thread.
	threadTS, err := slack.PostMessageWithTS(ctx, client, token, channelID, label, "")
	if err != nil {
		return err
	}

	if err := slack.PostMessage(ctx, client, token, channelID, prompt, threadTS); err != nil {
		return err
	}
	utils.Info("SlackPromptForImage posted", "team_id", teamID, "channel_id", channelID, "thread_ts", threadTS, "content_id", content.ID)

	newReq := map[string]any{
		"team_id":    teamID,
		"channel_id": channelID,
		"thread_ts":  threadTS,
		"prompt":     prompt,
		"hostname":   cfg.Hostname,
	}
	// Preserve prior requests so uploads to older threads can still be matched.
	// Keep the latest request in slack_image_request for convenience, but also maintain an append-only history list.
	history, _ := meta["slack_image_requests"].([]any)
	if history == nil {
		history = []any{}
	}
	// Ensure the previous slack_image_request is also in history (if present).
	if prev, ok := utils.GetMap(meta, "slack_image_request"); ok {
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
	// Add the new request to history if not already present.
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
		history = append(history, newReq)
	}

	meta["slack_image_request"] = newReq
	meta["slack_image_requests"] = history
	utils.SetStatus(meta, j.QueueOutput, true)

	// Don’t claim thumbnail_generated yet — Slack:Serve will finalize when an image is uploaded.
	return jctx.Store.UpdateContentMetaStatus(ctx, content.ID, j.QueueOutput, meta)
}
