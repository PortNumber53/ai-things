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
	if cfg.SlackTeamID == "" {
		return errors.New("missing slack.team_id (set slack.team_id in config.ini or SLACK_TEAM_ID)")
	}
	if cfg.SlackImageChannel == "" {
		return errors.New("missing slack.image_channel (set slack.image_channel in config.ini or SLACK_IMAGE_CHANNEL)")
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

	token, err := jctx.Store.GetSlackBotToken(ctx, cfg.SlackTeamID)
	if err != nil || token == "" {
		return fmt.Errorf("missing slack bot token for team_id=%s (install the Slack app first)", cfg.SlackTeamID)
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

	// Post the main message, and capture its ts so we can reply in-thread.
	threadTS, err := slack.PostMessageWithTS(ctx, client, token, cfg.SlackImageChannel, label, "")
	if err != nil {
		return err
	}

	if err := slack.PostMessage(ctx, client, token, cfg.SlackImageChannel, prompt, threadTS); err != nil {
		return err
	}

	meta["slack_image_request"] = map[string]any{
		"team_id":    cfg.SlackTeamID,
		"channel_id": cfg.SlackImageChannel,
		"thread_ts":  threadTS,
		"prompt":     prompt,
		"hostname":   cfg.Hostname,
	}
	utils.SetStatus(meta, j.QueueOutput, true)

	// Don’t claim thumbnail_generated yet — Slack:Serve will finalize when an image is uploaded.
	return jctx.Store.UpdateContentMetaStatus(ctx, content.ID, j.QueueOutput, meta)
}
