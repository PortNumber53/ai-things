package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"strings"
	"time"

	"ai-things/manager-go/internal/jobs"
	"ai-things/manager-go/internal/slack"
	"ai-things/manager-go/internal/utils"
)

func runSlackPruneImageThreads(ctx context.Context, jctx jobs.JobContext, args []string) error {
	fs := flag.NewFlagSet("Slack:PruneImageThreads", flag.ContinueOnError)
	days := fs.Int("days", 7, "Prune completed image threads older than N days")
	limit := fs.Int("limit", 200, "Maximum threads to prune in one run")
	dryRun := fs.Bool("dry-run", true, "Show what would be pruned without deleting")
	verbose := fs.Bool("verbose", utils.Verbose, "Verbose logging")
	if err := fs.Parse(args); err != nil {
		return err
	}
	utils.ConfigureLogging(*verbose)

	if *days <= 0 {
		return errors.New("--days must be > 0")
	}
	olderThan := time.Now().Add(-time.Duration(*days) * 24 * time.Hour)
	threads, err := jctx.Store.ListCompletedSlackImageThreadsToPrune(ctx, olderThan, *limit)
	if err != nil {
		return err
	}
	if len(threads) == 0 {
		fmt.Println("No threads to prune.")
		return nil
	}

	client := &http.Client{Timeout: 20 * time.Second}
	pruned := 0
	for _, t := range threads {
		if strings.TrimSpace(t.TeamID) == "" || strings.TrimSpace(t.ChannelID) == "" || strings.TrimSpace(t.ThreadTS) == "" {
			continue
		}
		if *dryRun {
			fmt.Printf("would_prune content_id=%d team_id=%s channel_id=%s thread_ts=%s completed_at=%s\n",
				t.ContentID, t.TeamID, t.ChannelID, t.ThreadTS, t.CompletedAt.Format(time.RFC3339))
			continue
		}

		token, err := jctx.Store.GetSlackBotToken(ctx, t.TeamID)
		if err != nil || token == "" {
			utils.Warn("prune: missing bot token", "team_id", t.TeamID, "content_id", t.ContentID, "err", err)
			continue
		}

		// Delete prompt reply first (if we have it), then delete the root thread message.
		// Best-effort: also attempt to delete the upload message and Slack file, if we have IDs.
		if strings.TrimSpace(t.PromptTS) != "" {
			if err := slack.DeleteMessage(ctx, client, token, t.ChannelID, t.PromptTS); err != nil {
				utils.Warn("prune: delete prompt failed", "content_id", t.ContentID, "channel", t.ChannelID, "ts", t.PromptTS, "err", err)
			}
		}
		if strings.TrimSpace(t.UploadTS) != "" {
			if err := slack.DeleteMessage(ctx, client, token, t.ChannelID, t.UploadTS); err != nil {
				utils.Warn("prune: delete upload message failed", "content_id", t.ContentID, "channel", t.ChannelID, "ts", t.UploadTS, "err", err)
			}
		}
		if strings.TrimSpace(t.FileID) != "" {
			if err := slack.DeleteFile(ctx, client, token, t.FileID); err != nil {
				utils.Warn("prune: delete slack file failed", "content_id", t.ContentID, "file_id", t.FileID, "err", err)
			}
		}
		if err := slack.DeleteMessage(ctx, client, token, t.ChannelID, t.ThreadTS); err != nil {
			utils.Warn("prune: delete thread failed", "content_id", t.ContentID, "channel", t.ChannelID, "ts", t.ThreadTS, "err", err)
			continue
		}

		// Mark pruned in meta so we don't keep trying.
		content, err := jctx.Store.GetContentByID(ctx, t.ContentID)
		if err != nil {
			utils.Warn("prune: load content failed", "content_id", t.ContentID, "err", err)
			continue
		}
		meta, err := utils.DecodeMeta(content.Meta)
		if err != nil {
			utils.Warn("prune: decode meta failed", "content_id", t.ContentID, "err", err)
			continue
		}
		if req, ok := utils.GetMap(meta, "slack_image_request"); ok {
			req["pruned"] = true
			req["pruned_at"] = time.Now().Format(time.RFC3339)
		}
		if err := jctx.Store.UpdateContentMeta(ctx, t.ContentID, meta); err != nil {
			utils.Warn("prune: update meta failed", "content_id", t.ContentID, "err", err)
			continue
		}
		pruned++
	}

	if *dryRun {
		fmt.Printf("dry_run_done candidates=%d\n", len(threads))
		return nil
	}
	fmt.Printf("pruned=%d\n", pruned)
	return nil
}
