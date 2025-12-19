package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"ai-things/manager-go/internal/db"
	"ai-things/manager-go/internal/subtitles"
	"ai-things/manager-go/internal/utils"
)

type CorrectSubtitlesJob struct {
	BaseJob
	MaxWaiting int
}

func NewCorrectSubtitlesJob() CorrectSubtitlesJob {
	return CorrectSubtitlesJob{
		BaseJob: BaseJob{
			QueueInput:  "srt_generated",
			QueueOutput: "srt_fixed",
		},
		MaxWaiting: 100,
	}
}

func (j CorrectSubtitlesJob) Run(ctx context.Context, jctx JobContext, opts JobOptions) error {
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
		utils.Logf("CorrectSubtitles: waiting=%d max=%d", count, j.MaxWaiting)
		if count >= j.MaxWaiting {
			utils.Logf("CorrectSubtitles: too many waiting, sleeping 60s")
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

func (j CorrectSubtitlesJob) countWaiting(ctx context.Context, jctx JobContext) (int, error) {
	where := "WHERE " + db.StatusTrueCondition([]string{"srt_generated"})
	notFixed := db.StatusNotTrueCondition([]string{"srt_fixed"})
	if notFixed != "" {
		where += " AND " + notFixed
	}
	return jctx.Store.CountContent(ctx, where)
}

func (j CorrectSubtitlesJob) selectNext(ctx context.Context, jctx JobContext) (db.Content, error) {
	where := "WHERE " + db.StatusTrueCondition([]string{"srt_generated"})
	notFixed := db.StatusNotTrueCondition([]string{"srt_fixed"})
	if notFixed != "" {
		where += " AND " + notFixed
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

func (j CorrectSubtitlesJob) processContent(ctx context.Context, jctx JobContext, contentID int64) error {
	utils.Logf("CorrectSubtitles: process content_id=%d", contentID)
	content, err := jctx.Store.GetContentByID(ctx, contentID)
	if err != nil {
		return err
	}
	meta, err := utils.DecodeMeta(content.Meta)
	if err != nil {
		return err
	}

	subtitlesMeta, ok := meta["subtitles"].(map[string]any)
	if !ok {
		return errors.New("subtitles missing")
	}
	srt, _ := subtitlesMeta["srt"].(string)
	if srt == "" {
		return errors.New("srt content missing")
	}

	originalText, err := utils.ExtractTextFromMeta(meta)
	if err != nil {
		return err
	}
	origWords := strings.Fields(originalText)

	captions := subtitles.ParseSRT(srt)
	if len(captions) == 0 {
		return errors.New("no captions parsed")
	}

	wordIndex := 0
	for i := range captions {
		words := strings.Fields(strings.ReplaceAll(captions[i].Text, "\n", " "))
		for j := range words {
			if wordIndex >= len(origWords) {
				break
			}
			words[j] = origWords[wordIndex]
			wordIndex++
		}
		captions[i].Text = strings.Join(words, " ")
	}

	if wordIndex < len(origWords) {
		captions[len(captions)-1].Text = strings.TrimSpace(captions[len(captions)-1].Text + " " + strings.Join(origWords[wordIndex:], " "))
	}

	fixed := subtitles.SerializeSRT(captions)
	subtitlesMeta["srt"] = fixed
	meta["subtitles"] = subtitlesMeta
	utils.SetStatus(meta, j.QueueOutput, true)

	if err := jctx.Store.UpdateContentMetaStatus(ctx, content.ID, j.QueueOutput, meta); err != nil {
		return err
	}

	payload, _ := json.Marshal(QueuePayload{ContentID: content.ID, Hostname: jctx.Config.Hostname})
	return jctx.Queue.Publish(j.QueueOutput, payload)
}
