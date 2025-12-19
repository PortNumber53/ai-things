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

type FixSubtitlesJob struct {
	BaseJob
	MaxWaiting int
}

func NewFixSubtitlesJob() FixSubtitlesJob {
	return FixSubtitlesJob{
		BaseJob: BaseJob{
			QueueInput:  "srt_generated",
			QueueOutput: "srt_fixed",
		},
		MaxWaiting: 100,
	}
}

func (j FixSubtitlesJob) Run(ctx context.Context, jctx JobContext, opts JobOptions) error {
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
		utils.Logf("FixSubtitles: waiting=%d max=%d", count, j.MaxWaiting)
		if count >= j.MaxWaiting {
			utils.Logf("FixSubtitles: too many waiting, sleeping 60s")
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

func (j FixSubtitlesJob) countWaiting(ctx context.Context, jctx JobContext) (int, error) {
	where := "WHERE " + db.StatusTrueCondition([]string{"srt_generated"})
	notFixed := db.StatusNotTrueCondition([]string{"srt_fixed"})
	if notFixed != "" {
		where += " AND " + notFixed
	}
	return jctx.Store.CountContent(ctx, where)
}

func (j FixSubtitlesJob) selectNext(ctx context.Context, jctx JobContext) (db.Content, error) {
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

func (j FixSubtitlesJob) processContent(ctx context.Context, jctx JobContext, contentID int64) error {
	utils.Logf("FixSubtitles: process content_id=%d", contentID)
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

	captions := subtitles.ParseSRT(srt)
	for i := range captions {
		text := strings.ReplaceAll(captions[i].Text, "\n", " ")
		captions[i].Text = strings.Join(strings.Fields(text), " ")
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
