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
			return j.processContent(ctx, jctx, contentID)
		})
	}

	contentID := opts.ContentID
	if contentID == 0 {
		count, err := j.countWaiting(ctx, jctx)
		if err != nil {
			return err
		}
		utils.Logf("GeneratePodcast: waiting=%d max=%d", count, j.MaxWaiting)
		if count >= j.MaxWaiting {
			utils.Logf("GeneratePodcast: too many waiting, sleeping 60s")
			time.Sleep(60 * time.Second)
			return nil
		}

		content, err := j.selectNext(ctx, jctx)
		if err != nil {
			return err
		}
		contentID = content.ID
	}

	return j.processContent(ctx, jctx, contentID)
}

func (j GeneratePodcastJob) countWaiting(ctx context.Context, jctx JobContext) (int, error) {
	where := "WHERE type = 'gemini.payload'"
	finishedTrue := db.StatusTrueCondition([]string{"funfact_created", "wav_generated", "mp3_generated", "srt_generated", "podcast_ready"})
	finishedFalse := db.StatusNotTrueCondition([]string{"youtube_uploaded"})
	if finishedTrue != "" {
		where += " AND " + finishedTrue
	}
	if finishedFalse != "" {
		where += " AND " + finishedFalse
	}
	return jctx.Store.CountContent(ctx, where)
}

func (j GeneratePodcastJob) selectNext(ctx context.Context, jctx JobContext) (db.Content, error) {
	where := "WHERE type = 'gemini.payload'"
	trueFlags := db.StatusTrueCondition([]string{"funfact_created", "wav_generated", "mp3_generated", "srt_generated"})
	falseFlags := db.StatusNotTrueCondition([]string{"podcast_ready"})
	if trueFlags != "" {
		where += " AND " + trueFlags
	}
	if falseFlags != "" {
		where += " AND " + falseFlags
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

func (j GeneratePodcastJob) processContent(ctx context.Context, jctx JobContext, contentID int64) error {
	utils.Logf("GeneratePodcast: process content_id=%d", contentID)
	content, err := jctx.Store.GetContentByID(ctx, contentID)
	if err != nil {
		return err
	}
	meta, err := utils.DecodeMeta(content.Meta)
	if err != nil {
		return err
	}

	mp3s, ok := meta["mp3s"].([]any)
	if !ok || len(mp3s) == 0 {
		utils.Logf("GeneratePodcast: mp3 metadata missing, resetting mp3_generated")
		_ = resetMp3Status(ctx, jctx, content.ID, meta)
		return nil
	}
	mp3Data, ok := mp3s[0].(map[string]any)
	if !ok {
		utils.Logf("GeneratePodcast: mp3 metadata invalid, resetting mp3_generated")
		_ = resetMp3Status(ctx, jctx, content.ID, meta)
		return nil
	}
	mp3Filename, _ := mp3Data["mp3"].(string)
	if mp3Filename == "" {
		utils.Logf("GeneratePodcast: mp3 filename missing, resetting mp3_generated")
		_ = resetMp3Status(ctx, jctx, content.ID, meta)
		return nil
	}
	mp3Path := filepath.Join(jctx.Config.BaseOutputFolder, "mp3", mp3Filename)

	if host, _ := mp3Data["hostname"].(string); host != "" && host != jctx.Config.Hostname {
		cmd := fmt.Sprintf("rsync -ravp --progress %s:%s %s", host, utils.ShellEscape(mp3Path), utils.ShellEscape(mp3Path))
		if output, err := utils.RunCommand(cmd); err != nil {
			if isMissingFileOutput(output) || !utils.FileExists(mp3Path) {
				utils.Logf("GeneratePodcast: mp3 missing on rsync, resetting mp3_generated")
				_ = resetMp3Status(ctx, jctx, content.ID, meta)
				return nil
			}
			return err
		}
	}

	if !utils.FileExists(mp3Path) {
		utils.Logf("GeneratePodcast: mp3 not found at %s, resetting mp3_generated", mp3Path)
		_ = resetMp3Status(ctx, jctx, content.ID, meta)
		return nil
	}

	thumbnail, ok := meta["thumbnail"].(map[string]any)
	if !ok {
		utils.Logf("GeneratePodcast: thumbnail metadata missing, resetting thumbnail_generated")
		_ = resetThumbnailStatus(ctx, jctx, content.ID, meta)
		return nil
	}
	imageFilename, _ := thumbnail["filename"].(string)
	if imageFilename == "" {
		utils.Logf("GeneratePodcast: thumbnail filename missing, resetting thumbnail_generated")
		_ = resetThumbnailStatus(ctx, jctx, content.ID, meta)
		return nil
	}
	imagePath := filepath.Join(jctx.Config.BaseOutputFolder, "images", imageFilename)

	if host, _ := thumbnail["hostname"].(string); host != "" && host != jctx.Config.Hostname {
		cmd := fmt.Sprintf("rsync -ravp --progress %s:%s %s", host, utils.ShellEscape(imagePath), utils.ShellEscape(imagePath))
		if output, err := utils.RunCommand(cmd); err != nil {
			if isMissingFileOutput(output) || !utils.FileExists(imagePath) {
				utils.Logf("GeneratePodcast: thumbnail missing on rsync, resetting thumbnail_generated")
				_ = resetThumbnailStatus(ctx, jctx, content.ID, meta)
				return nil
			}
			return err
		}
	}

	if !utils.FileExists(imagePath) {
		utils.Logf("GeneratePodcast: thumbnail not found at %s, resetting thumbnail_generated", imagePath)
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
		utils.Logf("GeneratePodcast: srt missing, resetting srt_generated")
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
		"__REPLACE_WITH_TITLE__":    fmt.Sprintf("%07d - %s", content.ID, content.Title),
		"__REPLACE_WITH_MP3__":      "audio.mp3",
		"__REPLACE_WITH_IMAGE__":    "image.jpg",
		"__REPLACE_WITH_SUBTITLES__": "podcast.srt",
		"__DURATION__":              strconv.Itoa(duration),
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
	if _, err := utils.RunCommand(cmd); err != nil {
		return err
	}

	podcastOut := filepath.Join(jctx.Config.BaseAppFolder, "podcast", "out", "video.mp4")
	podcastFilename := fmt.Sprintf("%010d.mp4", content.ID)
	podcastTarget := filepath.Join(jctx.Config.BaseOutputFolder, "podcast", podcastFilename)
	if err := utils.CopyFile(podcastOut, podcastTarget); err != nil {
		return err
	}

	meta["podcast"] = map[string]any{
		"filename": podcastFilename,
		"hostname": jctx.Config.Hostname,
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
	return strings.Contains(lower, "no such file") || strings.Contains(lower, "cannot stat")
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
