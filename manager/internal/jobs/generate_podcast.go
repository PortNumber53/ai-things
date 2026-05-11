package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"ai-things/manager-go/internal/db"
	"ai-things/manager-go/internal/utils"
)

type GeneratePodcastJob struct {
	BaseJob
	MaxWaiting int
}

func NewGeneratePodcastJob() GeneratePodcastJob {
	return GeneratePodcastJob{
		BaseJob: BaseJob{
			QueueInput:  "generate_podcast",
			QueueOutput: "podcast_ready",
		},
		MaxWaiting: 100,
	}
}

func (j GeneratePodcastJob) Run(ctx context.Context, jctx JobContext, opts JobOptions) error {
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
		utils.Debug("GeneratePodcast waiting", "waiting", count, "max_waiting", j.MaxWaiting)
		if count >= j.MaxWaiting {
			utils.Warn("GeneratePodcast too many waiting; sleeping", "sleep_s", 60, "waiting", count, "max_waiting", j.MaxWaiting)
			time.Sleep(60 * time.Second)
			return nil
		}

		content, err := j.selectNext(ctx, jctx)
		if err != nil {
			return err
		}
		contentID = content.ID
	}

	return j.processContent(ctx, jctx, contentID, opts.Regenerate)
}

func (j GeneratePodcastJob) countWaiting(ctx context.Context, jctx JobContext) (int, error) {
	where := "WHERE type = 'gemini.payload'"
	readyTrue := db.StatusTrueCondition([]string{"funfact_created", "wav_generated", "mp3_generated", "srt_generated", "thumbnail_generated"})
	notReady := db.StatusNotTrueCondition([]string{"podcast_ready"})
	// Once uploaded anywhere, treat it as terminal and do not re-render (unless forced by explicit content_id).
	notYoutubeUploaded := db.StatusNotTrueCondition([]string{"youtube_uploaded"})
	notTiktokUploaded := db.StatusNotTrueCondition([]string{"tiktok_uploaded"})
	missingVideoID := db.MetaKeyMissingCondition([]string{"video_id.v1"})
	if readyTrue != "" {
		where += " AND " + readyTrue
	}
	if notReady != "" {
		where += " AND " + notReady
	}
	if notYoutubeUploaded != "" {
		where += " AND " + notYoutubeUploaded
	}
	if notTiktokUploaded != "" {
		where += " AND " + notTiktokUploaded
	}
	if missingVideoID != "" {
		where += " AND " + missingVideoID
	}
	return jctx.Store.CountContent(ctx, where)
}

func (j GeneratePodcastJob) selectNext(ctx context.Context, jctx JobContext) (db.Content, error) {
	where := "WHERE type = 'gemini.payload'"
	trueFlags := db.StatusTrueCondition([]string{"funfact_created", "wav_generated", "mp3_generated", "srt_generated", "thumbnail_generated"})
	falseFlags := db.StatusNotTrueCondition([]string{"podcast_ready"})
	// Once uploaded anywhere, treat it as terminal and do not re-render (unless forced by explicit content_id).
	notYoutubeUploaded := db.StatusNotTrueCondition([]string{"youtube_uploaded"})
	notTiktokUploaded := db.StatusNotTrueCondition([]string{"tiktok_uploaded"})
	missingVideoID := db.MetaKeyMissingCondition([]string{"video_id.v1"})
	if trueFlags != "" {
		where += " AND " + trueFlags
	}
	if falseFlags != "" {
		where += " AND " + falseFlags
	}
	if notYoutubeUploaded != "" {
		where += " AND " + notYoutubeUploaded
	}
	if notTiktokUploaded != "" {
		where += " AND " + notTiktokUploaded
	}
	if missingVideoID != "" {
		where += " AND " + missingVideoID
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

func (j GeneratePodcastJob) processContent(ctx context.Context, jctx JobContext, contentID int64, force bool) error {
	utils.Info("GeneratePodcast process", "content_id", contentID, "force", force)
	content, err := jctx.Store.GetContentByID(ctx, contentID)
	if err != nil {
		return err
	}
	meta, err := utils.DecodeMeta(content.Meta)
	if err != nil {
		return err
	}

	// Terminal state: once uploaded to BOTH YouTube and TikTok (or manually overridden with a YouTube video ID + TikTok),
	// do not re-render unless forced.
	if !force {
		youtubeUploaded := false
		tiktokUploaded := false
		if status, ok := meta["status"].(map[string]any); ok {
			if raw, ok := status["youtube_uploaded"].(string); ok && raw == "true" {
				youtubeUploaded = true
			}
			if raw, ok := status["youtube_uploaded"].(bool); ok && raw {
				youtubeUploaded = true
			}
			if raw, ok := status["tiktok_uploaded"].(string); ok && raw == "true" {
				tiktokUploaded = true
			}
			if raw, ok := status["tiktok_uploaded"].(bool); ok && raw {
				tiktokUploaded = true
			}
		}
		_, hasVideoID := meta["video_id.v1"]
		if youtubeUploaded || tiktokUploaded || hasVideoID {
			utils.Info("GeneratePodcast skip (already uploaded)", "content_id", contentID, "youtube_uploaded", youtubeUploaded, "tiktok_uploaded", tiktokUploaded, "has_video_id", hasVideoID)
			// Avoid reprocessing loops if podcast_ready was reset after upload.
			utils.SetStatus(meta, "podcast_ready", true)
			_ = jctx.Store.UpdateContentMetaStatus(ctx, content.ID, "podcast_ready", meta)
			return nil
		}
	}

	mp3s, ok := meta["mp3s"].([]any)
	if !ok || len(mp3s) == 0 {
		utils.Warn("GeneratePodcast mp3 metadata missing; resetting mp3_generated", "content_id", contentID)
		_ = resetMp3Status(ctx, jctx, content.ID, meta)
		return nil
	}
	mp3Data, ok := mp3s[0].(map[string]any)
	if !ok {
		utils.Warn("GeneratePodcast mp3 metadata invalid; resetting mp3_generated", "content_id", contentID)
		_ = resetMp3Status(ctx, jctx, content.ID, meta)
		return nil
	}
	mp3Filename, _ := mp3Data["mp3"].(string)
	if mp3Filename == "" {
		utils.Warn("GeneratePodcast mp3 filename missing; resetting mp3_generated", "content_id", contentID)
		_ = resetMp3Status(ctx, jctx, content.ID, meta)
		return nil
	}
	mp3SHA, _ := mp3Data["sha256"].(string)
	mp3Path := filepath.Join(jctx.Config.BaseOutputFolder, "mp3", mp3Filename)

	if host, _ := mp3Data["hostname"].(string); host != "" && host != jctx.Config.Hostname {
		needRsync := true
		if utils.FileExists(mp3Path) {
			if mp3SHA == "" {
				needRsync = false
				utils.Debug("GeneratePodcast mp3 present locally; skipping rsync", "content_id", contentID, "path", mp3Path)
			} else {
				haveSHA, err := utils.SHA256File(mp3Path)
				if err != nil {
					return err
				}
				if haveSHA == mp3SHA {
					needRsync = false
					utils.Debug("GeneratePodcast mp3 checksum match; skipping rsync", "content_id", contentID, "path", mp3Path)
				} else {
					utils.Debug("GeneratePodcast mp3 checksum mismatch; will rsync", "content_id", contentID, "path", mp3Path)
				}
			}
		}
		if needRsync {
			cmd := fmt.Sprintf("rsync -ravp --progress %s:%s %s", host, utils.ShellEscape(mp3Path), utils.ShellEscape(mp3Path))
			output, err := utils.RunCommand(cmd)
			if err != nil {
				if isMissingFileOutput(output) || !utils.FileExists(mp3Path) {
					utils.Warn("GeneratePodcast mp3 missing on rsync; resetting mp3_generated", "content_id", contentID, "host", host)
					_ = resetMp3Status(ctx, jctx, content.ID, meta)
					return nil
				}
				return err
			}
			if mp3SHA != "" {
				haveSHA, err := utils.SHA256File(mp3Path)
				if err != nil {
					return err
				}
				if haveSHA != mp3SHA {
					return fmt.Errorf("mp3 checksum mismatch after rsync: %s", mp3Path)
				}
			}
		}
	}

	if !utils.FileExists(mp3Path) {
		utils.Warn("GeneratePodcast mp3 not found; resetting mp3_generated", "content_id", contentID, "path", mp3Path)
		_ = resetMp3Status(ctx, jctx, content.ID, meta)
		return nil
	}

	thumbnail, ok := meta["thumbnail"].(map[string]any)
	if !ok {
		utils.Warn("GeneratePodcast thumbnail metadata missing; resetting thumbnail_generated", "content_id", contentID)
		_ = resetThumbnailStatus(ctx, jctx, content.ID, meta)
		return nil
	}
	imageFilename, _ := thumbnail["filename"].(string)
	if imageFilename == "" {
		utils.Warn("GeneratePodcast thumbnail filename missing; resetting thumbnail_generated", "content_id", contentID)
		_ = resetThumbnailStatus(ctx, jctx, content.ID, meta)
		return nil
	}
	imageSHA, _ := thumbnail["sha256"].(string)
	imagePath := filepath.Join(jctx.Config.BaseOutputFolder, "images", imageFilename)

	if host, _ := thumbnail["hostname"].(string); host != "" && host != jctx.Config.Hostname {
		needRsync := true
		if utils.FileExists(imagePath) {
			if imageSHA == "" {
				needRsync = false
				utils.Debug("GeneratePodcast thumbnail present locally; skipping rsync", "content_id", contentID, "path", imagePath)
			} else {
				haveSHA, err := utils.SHA256File(imagePath)
				if err != nil {
					return err
				}
				if haveSHA == imageSHA {
					needRsync = false
					utils.Debug("GeneratePodcast thumbnail checksum match; skipping rsync", "content_id", contentID, "path", imagePath)
				} else {
					utils.Debug("GeneratePodcast thumbnail checksum mismatch; will rsync", "content_id", contentID, "path", imagePath)
				}
			}
		}
		if needRsync {
			cmd := fmt.Sprintf("rsync -ravp --progress %s:%s %s", host, utils.ShellEscape(imagePath), utils.ShellEscape(imagePath))
			output, err := utils.RunCommand(cmd)
			if err != nil {
				if isMissingFileOutput(output) || !utils.FileExists(imagePath) {
					utils.Warn("GeneratePodcast thumbnail missing on rsync; resetting thumbnail_generated", "content_id", contentID, "host", host)
					_ = resetThumbnailStatus(ctx, jctx, content.ID, meta)
					return nil
				}
				return err
			}
			if imageSHA != "" {
				haveSHA, err := utils.SHA256File(imagePath)
				if err != nil {
					return err
				}
				if haveSHA != imageSHA {
					return fmt.Errorf("thumbnail checksum mismatch after rsync: %s", imagePath)
				}
			}
		}
	}

	if !utils.FileExists(imagePath) {
		utils.Warn("GeneratePodcast thumbnail not found; resetting thumbnail_generated", "content_id", contentID, "path", imagePath)
		_ = resetThumbnailStatus(ctx, jctx, content.ID, meta)
		return nil
	}

	aiImagePath := filepath.Join(jctx.Config.BaseOutputFolder, "images-ai", imageFilename)
	if utils.FileExists(aiImagePath) {
		cmd := fmt.Sprintf("rsync -ravp --progress %s %s", utils.ShellEscape(aiImagePath), utils.ShellEscape(imagePath))
		if _, err := utils.RunCommand(cmd); err != nil {
			return err
		}
	}

	podcastPublic := filepath.Join(jctx.Config.BaseAppFolder, "podcast", "public")
	if err := utils.CopyFile(mp3Path, filepath.Join(podcastPublic, "audio.mp3")); err != nil {
		return err
	}
	if err := utils.CopyFile(imagePath, filepath.Join(podcastPublic, "image.jpg")); err != nil {
		return err
	}

	subtitles, ok := meta["subtitles"].(map[string]any)
	if !ok {
		return errors.New("subtitles missing")
	}
	srt, _ := subtitles["srt"].(string)
	if srt == "" {
		utils.Warn("GeneratePodcast srt missing; resetting srt_generated", "content_id", contentID)
		_ = resetSrtStatus(ctx, jctx, content.ID, meta)
		return nil
	}
	if err := os.WriteFile(filepath.Join(podcastPublic, "podcast.srt"), []byte(srt), 0o644); err != nil {
		return err
	}

	rootTemplate := filepath.Join(jctx.Config.BaseAppFolder, "podcast", "src", "Root_template.tsx")
	rootTarget := filepath.Join(jctx.Config.BaseAppFolder, "podcast", "src", "Root.tsx")
	templateData, err := os.ReadFile(rootTemplate)
	if err != nil {
		return err
	}

	duration := 0
	if rawDuration, ok := mp3Data["duration"].(float64); ok {
		duration = int(rawDuration)
	}

	replacements := map[string]string{
		"__REPLACE_WITH_TITLE__":     utils.EscapeJSSingleQuotedString(fmt.Sprintf("%07d - %s", content.ID, content.Title)),
		"__REPLACE_WITH_MP3__":       utils.EscapeJSSingleQuotedString("audio.mp3"),
		"__REPLACE_WITH_IMAGE__":     utils.EscapeJSSingleQuotedString("image.jpg"),
		"__REPLACE_WITH_SUBTITLES__": utils.EscapeJSSingleQuotedString("podcast.srt"),
		"__DURATION__":               strconv.Itoa(duration),
	}

	generated := string(templateData)
	for key, value := range replacements {
		generated = strings.ReplaceAll(generated, key, value)
	}

	if err := os.WriteFile(rootTarget, []byte(generated), 0o644); err != nil {
		return err
	}

	buildArgs := ""
	if utils.Verbose {
		// Make Remotion show the underlying Chromium stderr, which is often the real cause
		// (missing shared libraries, missing fonts, etc.).
		buildArgs = " -- --log=verbose"
	}
	cmd := fmt.Sprintf(
		"cd %s && npm run build%s",
		utils.ShellEscape(filepath.Join(jctx.Config.BaseAppFolder, "podcast")),
		buildArgs,
	)

	// Remotion renders to podcast/out/video.mp4. Ensure the output directory exists first.
	// Some environments (fresh deploys / rsynced folders) may not have created it yet.
	outDir := filepath.Join(jctx.Config.BaseAppFolder, "podcast", "out")
	if err := utils.EnsureDir(outDir); err != nil {
		return err
	}
	podcastOut := filepath.Join(outDir, "video.mp4")
	// Remove any stale output so we can reliably detect whether this run produced a file.
	_ = os.Remove(podcastOut)
	buildOutput, err := utils.RunCommand(cmd)
	if err != nil {
		return err
	}
	if !utils.FileExists(podcastOut) {
		trimmed := strings.TrimSpace(buildOutput)
		if len(trimmed) > 2000 {
			trimmed = trimmed[len(trimmed)-2000:]
		}
		if trimmed != "" {
			return fmt.Errorf("podcast build finished but output file missing: %s\n\nlast build output:\n%s", podcastOut, trimmed)
		}
		return fmt.Errorf("podcast build finished but output file missing: %s", podcastOut)
	}

	podcastFilename := fmt.Sprintf("%010d.mp4", content.ID)
	podcastTarget := filepath.Join(jctx.Config.BaseOutputFolder, "podcast", podcastFilename)
	if err := utils.CopyFile(podcastOut, podcastTarget); err != nil {
		return err
	}

	podcastSHA, err := utils.SHA256File(podcastTarget)
	if err != nil {
		return err
	}

	meta["podcast"] = map[string]any{
		"filename": podcastFilename,
		"hostname": jctx.Config.Hostname,
		"sha256":   podcastSHA,
	}
	utils.SetStatus(meta, j.QueueOutput, true)

	if err := jctx.Store.UpdateContentMetaStatus(ctx, content.ID, j.QueueOutput, meta); err != nil {
		return err
	}

	payload, _ := json.Marshal(QueuePayload{ContentID: content.ID, Hostname: jctx.Config.Hostname})
	return jctx.Queue.Publish(j.QueueOutput, payload)
}

func isMissingFileOutput(output string) bool {
	lower := strings.ToLower(output)
	return strings.Contains(lower, "no such file") ||
		strings.Contains(lower, "cannot stat") ||
		strings.Contains(lower, "could not resolve hostname") ||
		strings.Contains(lower, "connection unexpectedly closed")
}

func resetMp3Status(ctx context.Context, jctx JobContext, contentID int64, meta map[string]any) error {
	delete(meta, "mp3s")
	utils.SetStatus(meta, "mp3_generated", false)
	return jctx.Store.UpdateContentMetaStatus(ctx, contentID, "wav_generated", meta)
}

func resetThumbnailStatus(ctx context.Context, jctx JobContext, contentID int64, meta map[string]any) error {
	delete(meta, "thumbnail")
	utils.SetStatus(meta, "thumbnail_generated", false)
	return jctx.Store.UpdateContentMetaStatus(ctx, contentID, "srt_generated", meta)
}

func resetSrtStatus(ctx context.Context, jctx JobContext, contentID int64, meta map[string]any) error {
	if subtitlesMeta, ok := meta["subtitles"].(map[string]any); ok {
		delete(subtitlesMeta, "srt")
		meta["subtitles"] = subtitlesMeta
	}
	utils.SetStatus(meta, "srt_generated", false)
	return jctx.Store.UpdateContentMetaStatus(ctx, contentID, "mp3_generated", meta)
}
