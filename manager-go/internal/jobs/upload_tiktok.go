package jobs

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"time"

	"ai-things/manager-go/internal/db"
	"ai-things/manager-go/internal/utils"
)

type UploadTikTokJob struct {
	BaseJob
	MaxWaiting int
}

func NewUploadTikTokJob() UploadTikTokJob {
	return UploadTikTokJob{
		BaseJob: BaseJob{
			QueueInput:  "podcast_ready",
			QueueOutput: "upload.tiktok",
		},
		MaxWaiting: 100,
	}
}

func (j UploadTikTokJob) Run(ctx context.Context, jctx JobContext, opts JobOptions) error {
	if opts.Queue {
		return j.RunQueue(ctx, jctx, opts, func(ctx context.Context, contentID int64, hostname string) error {
			return j.processContent(ctx, jctx, contentID, opts.Info)
		})
	}

	contentID := opts.ContentID
	if contentID == 0 {
		count, err := j.countWaiting(ctx, jctx)
		if err != nil {
			return err
		}
		utils.Logf("UploadTikTok: waiting=%d max=%d", count, j.MaxWaiting)
		if count >= j.MaxWaiting {
			utils.Logf("UploadTikTok: too many waiting, sleeping 60s")
			time.Sleep(60 * time.Second)
			return nil
		}

		content, err := j.selectNext(ctx, jctx)
		if err != nil {
			return err
		}
		contentID = content.ID
	}

	return j.processContent(ctx, jctx, contentID, opts.Info)
}

func (j UploadTikTokJob) countWaiting(ctx context.Context, jctx JobContext) (int, error) {
	where := "WHERE type = 'gemini.payload'"
	trueFlags := db.StatusTrueCondition([]string{"funfact_created", "wav_generated", "mp3_generated", "srt_generated", "thumbnail_generated", "podcast_ready"})
	falseFlags := db.StatusFalseCondition([]string{"tiktok_uploaded"})
	missing := db.MetaKeyMissingCondition([]string{"tiktok_video_id"})
	if trueFlags != "" {
		where += " AND " + trueFlags
	}
	if falseFlags != "" {
		where += " AND " + falseFlags
	}
	if missing != "" {
		where += " AND " + missing
	}
	return jctx.Store.CountContent(ctx, where)
}

func (j UploadTikTokJob) selectNext(ctx context.Context, jctx JobContext) (db.Content, error) {
	where := "WHERE type = 'gemini.payload'"
	trueFlags := db.StatusTrueCondition([]string{"funfact_created", "wav_generated", "mp3_generated", "srt_generated", "thumbnail_generated", "podcast_ready"})
	notTrue := db.StatusNotTrueCondition([]string{"tiktok_uploaded"})
	missing := db.MetaKeyMissingCondition([]string{"tiktok_video_id"})
	if trueFlags != "" {
		where += " AND " + trueFlags
	}
	if notTrue != "" {
		where += " AND " + notTrue
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

func (j UploadTikTokJob) processContent(ctx context.Context, jctx JobContext, contentID int64, info bool) error {
	utils.Logf("UploadTikTok: process content_id=%d info=%t", contentID, info)
	content, err := jctx.Store.GetContentByID(ctx, contentID)
	if err != nil {
		return err
	}
	meta, err := utils.DecodeMeta(content.Meta)
	if err != nil {
		return err
	}

	description, err := utils.ExtractTextFromMeta(meta)
	if err != nil {
		return err
	}

	podcast, ok := meta["podcast"].(map[string]any)
	if !ok {
		return errors.New("podcast metadata missing")
	}
	podcastFilename, _ := podcast["filename"].(string)
	if podcastFilename == "" {
		return errors.New("podcast filename missing")
	}

	filename := filepath.Join(jctx.Config.BaseOutputFolder, "podcast", podcastFilename)
	caption := fmt.Sprintf("%07d - %s", content.ID, content.Title)

	if info {
		_, _ = fmt.Printf("Caption: %s\nDescription: %s\n", caption, description)
		videoID, err := utils.Prompt("Enter video ID")
		if err != nil {
			return err
		}
		meta["tiktok_video_id"] = videoID
		utils.SetStatus(meta, j.QueueOutput, true)
		utils.SetStatus(meta, "tiktok_uploaded", true)
		return jctx.Store.UpdateContentMetaStatus(ctx, content.ID, j.QueueOutput, meta)
	}

	utilityDir := filepath.Join(jctx.Config.BaseAppFolder, "utility")
	cmd := fmt.Sprintf("cd %s && %s %s %s 2>&1",
		utils.ShellEscape(utilityDir),
		jctx.Config.TikTokUploadScript,
		utils.ShellEscape(filename),
		utils.ShellEscape(caption),
	)
	output, err := utils.RunCommand(cmd)
	if err != nil {
		return err
	}

	pattern := regexp.MustCompile("Video id '([^']+)' was successfully uploaded")
	matches := pattern.FindStringSubmatch(output)
	if len(matches) < 2 {
		return errors.New("video ID not found in upload output")
	}

	meta["tiktok_video_id"] = matches[1]
	utils.SetStatus(meta, j.QueueOutput, true)
	utils.SetStatus(meta, "tiktok_uploaded", true)

	return jctx.Store.UpdateContentMetaStatus(ctx, content.ID, j.QueueOutput, meta)
}
