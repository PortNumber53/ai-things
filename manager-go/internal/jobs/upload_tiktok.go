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
		utils.Debug("UploadTikTok waiting", "waiting", count, "max_waiting", j.MaxWaiting)
		if count >= j.MaxWaiting {
			utils.Warn("UploadTikTok too many waiting; sleeping", "sleep_s", 60, "waiting", count, "max_waiting", j.MaxWaiting)
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
	// Treat "not uploaded yet" as "not true" (NULL or anything other than 'true'),
	// matching selectNext(). Using StatusFalseCondition would require an explicit 'false' value.
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
	utils.Info("UploadTikTok process", "content_id", contentID, "info", info)
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

	if host, _ := podcast["hostname"].(string); host != "" && host != jctx.Config.Hostname {
		wantSHA, _ := podcast["sha256"].(string)
		needRsync := true
		if utils.FileExists(filename) {
			if wantSHA == "" {
				needRsync = false
				utils.Debug("UploadTikTok podcast present locally; skipping rsync", "content_id", contentID, "path", filename)
			} else {
				haveSHA, err := utils.SHA256File(filename)
				if err != nil {
					return err
				}
				if haveSHA == wantSHA {
					needRsync = false
					utils.Debug("UploadTikTok podcast checksum match; skipping rsync", "content_id", contentID, "path", filename)
				} else {
					utils.Debug("UploadTikTok podcast checksum mismatch; will rsync", "content_id", contentID, "path", filename)
				}
			}
		}

		if needRsync {
			if err := utils.EnsureDir(filepath.Dir(filename)); err != nil {
				return err
			}
			cmd := fmt.Sprintf("rsync -ravp --progress %s:%s %s", host, utils.ShellEscape(filename), utils.ShellEscape(filename))
			output, err := utils.RunCommand(cmd)
			if err != nil {
				if isMissingFileOutput(output) || !utils.FileExists(filename) {
					utils.Warn("UploadTikTok podcast missing/unreachable; resetting podcast_ready", "content_id", contentID, "host", host, "path", filename)
					_ = resetPodcastStatus(ctx, jctx, contentID, meta)
					return nil
				}
				return err
			}
			if wantSHA != "" {
				haveSHA, err := utils.SHA256File(filename)
				if err != nil {
					return err
				}
				if haveSHA != wantSHA {
					utils.Warn("UploadTikTok podcast checksum mismatch after rsync; resetting podcast_ready", "content_id", contentID, "host", host, "path", filename)
					_ = resetPodcastStatus(ctx, jctx, contentID, meta)
					return nil
				}
			}
		}
	}
	if !utils.FileExists(filename) {
		utils.Warn("UploadTikTok podcast missing; resetting podcast_ready", "content_id", contentID, "path", filename)
		_ = resetPodcastStatus(ctx, jctx, contentID, meta)
		return nil
	}

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
		utils.ShellEscape(resolveWorkDir([]string{
			utilityDir,
			filepath.Join("..", "utility"),
			"utility",
		}, jctx.Config.TikTokUploadScript)),
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
