package jobs

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"ai-things/manager-go/internal/db"
	"ai-things/manager-go/internal/utils"
)

type UploadYouTubeJob struct {
	BaseJob
	MaxWaiting int
}

func NewUploadYouTubeJob() UploadYouTubeJob {
	return UploadYouTubeJob{
		BaseJob: BaseJob{
			QueueInput:  "podcast_ready",
			QueueOutput: "upload.tiktok",
		},
		MaxWaiting: 100,
	}
}

func (j UploadYouTubeJob) Run(ctx context.Context, jctx JobContext, opts JobOptions) error {
	if opts.Queue {
		return j.RunQueue(ctx, jctx, opts, func(ctx context.Context, contentID int64, hostname string) error {
			return j.processContent(ctx, jctx, contentID, opts.Info, opts.EasyUpload)
		})
	}

	contentID := opts.ContentID
	if contentID == 0 {
		count, err := j.countWaiting(ctx, jctx)
		if err != nil {
			return err
		}
		utils.Logf("UploadYouTube: waiting=%d max=%d", count, j.MaxWaiting)
		if count >= j.MaxWaiting {
			utils.Logf("UploadYouTube: too many waiting, sleeping 60s")
			time.Sleep(60 * time.Second)
			return nil
		}

		content, err := j.selectNext(ctx, jctx)
		if err != nil {
			return err
		}
		contentID = content.ID
	}

	return j.processContent(ctx, jctx, contentID, opts.Info, opts.EasyUpload)
}

func (j UploadYouTubeJob) countWaiting(ctx context.Context, jctx JobContext) (int, error) {
	where := "WHERE type = 'gemini.payload'"
	trueFlags := db.StatusTrueCondition([]string{"funfact_created", "wav_generated", "mp3_generated", "srt_generated", "thumbnail_generated", "podcast_ready"})
	falseFlags := db.StatusFalseCondition([]string{"youtube_uploaded"})
	missing := db.MetaKeyMissingCondition([]string{"video_id.v1"})
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

func (j UploadYouTubeJob) selectNext(ctx context.Context, jctx JobContext) (db.Content, error) {
	where := "WHERE type = 'gemini.payload'"
	trueFlags := db.StatusTrueCondition([]string{"funfact_created", "wav_generated", "mp3_generated", "srt_generated", "thumbnail_generated", "podcast_ready"})
	notTrue := db.StatusNotTrueCondition([]string{"youtube_uploaded"})
	missing := db.MetaKeyMissingCondition([]string{"video_id.v1"})
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

func (j UploadYouTubeJob) processContent(ctx context.Context, jctx JobContext, contentID int64, info bool, easyUpload bool) error {
	utils.Logf("UploadYouTube: process content_id=%d info=%t easy_upload=%t", contentID, info, easyUpload)
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
	title := fmt.Sprintf("%07d - %s", content.ID, content.Title)
	category := "27"
	keywords := ""
	privacyStatus := "public"

	uploadDir := filepath.Join(jctx.Config.BaseOutputFolder, "upload")
	var hardlinkedFiles []string
	if easyUpload {
		if err := os.MkdirAll(uploadDir, 0o755); err != nil {
			return err
		}

		podcastLink := filepath.Join(uploadDir, filepath.Base(filename))
		_ = os.Remove(podcastLink)
		if err := os.Link(filename, podcastLink); err != nil {
			return err
		}
		hardlinkedFiles = append(hardlinkedFiles, podcastLink)

		thumbnail, ok := meta["thumbnail"].(map[string]any)
		if !ok {
			return errors.New("thumbnail metadata missing")
		}
		thumbFilename, _ := thumbnail["filename"].(string)
		if thumbFilename == "" {
			return errors.New("thumbnail filename missing")
		}
		thumbSource := filepath.Join(jctx.Config.BaseOutputFolder, "images", thumbFilename)
		thumbLink := filepath.Join(uploadDir, thumbFilename)
		_ = os.Remove(thumbLink)
		if err := os.Link(thumbSource, thumbLink); err != nil {
			return err
		}
		hardlinkedFiles = append(hardlinkedFiles, thumbLink)
	}

	cleanup := func() {
		for _, file := range hardlinkedFiles {
			_ = os.Remove(file)
		}
	}
	defer cleanup()

	if info {
		fmt.Printf("Title: %s\n", title)
		if originalText, _ := meta["original_text"].(string); originalText != "" {
			fmt.Printf("Original Text:\n%s\n", originalText)
		} else {
			fmt.Printf("Description: %s\n", description)
		}
		fmt.Printf("Category: %s\nKeywords: %s\nPrivacy status: %s\n", category, keywords, privacyStatus)

		input, err := utils.Prompt("Enter video ID or URL")
		if err != nil {
			return err
		}
		videoID := extractYouTubeID(input)
		if videoID == "" {
			return errors.New("invalid YouTube video ID or URL")
		}
		meta["video_id.v1"] = videoID
		utils.SetStatus(meta, j.QueueOutput, true)
		utils.SetStatus(meta, "youtube_uploaded", true)
		return jctx.Store.UpdateContentMetaStatus(ctx, content.ID, j.QueueOutput, meta)
	}

	command := fmt.Sprintf(
		"cd %s && %s --file=%s --title=%s --description=%s --category=%s --keywords=\"%s\" --privacyStatus=%s",
		utils.ShellEscape(filepath.Join(jctx.Config.BaseAppFolder, "auto-subtitles-generator")),
		jctx.Config.YoutubeUpload,
		utils.ShellEscape(filename),
		utils.ShellEscape(title),
		utils.ShellEscape(description),
		utils.ShellEscape(category),
		strings.ReplaceAll(keywords, "\"", "\\\""),
		utils.ShellEscape(privacyStatus),
	)

	output, err := utils.RunCommand(command)
	if err != nil {
		return err
	}

	pattern := regexp.MustCompile("Video id '([^']+)' was successfully uploaded")
	matches := pattern.FindStringSubmatch(output)
	if len(matches) < 2 {
		return errors.New("video ID not found in upload output")
	}

	meta["video_id.v1"] = matches[1]
	utils.SetStatus(meta, j.QueueOutput, true)
	utils.SetStatus(meta, "youtube_uploaded", true)

	return jctx.Store.UpdateContentMetaStatus(ctx, content.ID, j.QueueOutput, meta)
}

func extractYouTubeID(input string) string {
	input = strings.TrimSpace(input)
	if len(input) == 11 && regexp.MustCompile(`^[a-zA-Z0-9_-]{11}$`).MatchString(input) {
		return input
	}

	parsed, err := url.Parse(input)
	if err != nil {
		return ""
	}

	switch parsed.Host {
	case "youtu.be":
		return strings.TrimPrefix(parsed.Path, "/")
	case "www.youtube.com", "youtube.com":
		if strings.HasPrefix(parsed.Path, "/watch") {
			return parsed.Query().Get("v")
		}
		if strings.HasPrefix(parsed.Path, "/embed/") {
			return strings.TrimPrefix(parsed.Path, "/embed/")
		}
		if strings.HasPrefix(parsed.Path, "/shorts/") {
			return strings.TrimPrefix(parsed.Path, "/shorts/")
		}
	}

	return ""
}
