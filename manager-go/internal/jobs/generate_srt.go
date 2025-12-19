package jobs

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"ai-things/manager-go/internal/db"
	"ai-things/manager-go/internal/utils"
)

type GenerateSrtJob struct {
	BaseJob
	MaxWaiting int
}

func NewGenerateSrtJob() GenerateSrtJob {
	return GenerateSrtJob{
		BaseJob: BaseJob{
			QueueInput:  "wav_generated",
			QueueOutput: "srt_generated",
		},
		MaxWaiting: 100,
	}
}

func (j GenerateSrtJob) Run(ctx context.Context, jctx JobContext, opts JobOptions) error {
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
		utils.Logf("GenerateSrt: waiting=%d max=%d", count, j.MaxWaiting)
		if count >= j.MaxWaiting {
			utils.Logf("GenerateSrt: too many waiting, sleeping 60s")
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

func (j GenerateSrtJob) countWaiting(ctx context.Context, jctx JobContext) (int, error) {
	where := "WHERE type = 'gemini.payload'"
	finishedTrue := db.StatusTrueCondition([]string{"funfact_created", "wav_generated", "mp3_generated", "srt_generated"})
	finishedFalse := db.StatusNotTrueCondition([]string{"podcast_ready"})
	if finishedTrue != "" {
		where += " AND " + finishedTrue
	}
	if finishedFalse != "" {
		where += " AND " + finishedFalse
	}
	return jctx.Store.CountContent(ctx, where)
}

func (j GenerateSrtJob) selectNext(ctx context.Context, jctx JobContext) (db.Content, error) {
	where := "WHERE type = 'gemini.payload'"
	trueFlags := db.StatusTrueCondition([]string{"funfact_created", "wav_generated", "mp3_generated"})
	falseFlags := db.StatusNotTrueCondition([]string{"srt_generated"})
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

func (j GenerateSrtJob) processContent(ctx context.Context, jctx JobContext, contentID int64) error {
	utils.Logf("GenerateSrt: process content_id=%d", contentID)
	content, err := jctx.Store.GetContentByID(ctx, contentID)
	if err != nil {
		return err
	}
	meta, err := utils.DecodeMeta(content.Meta)
	if err != nil {
		return err
	}

	wavMeta, ok := utils.GetMap(meta, "wav")
	if !ok {
		return errors.New("wav metadata not found")
	}
	wavFilename, _ := wavMeta["filename"].(string)
	if wavFilename == "" {
		return errors.New("wav filename missing")
	}
	wavPath := filepath.Join(jctx.Config.BaseOutputFolder, "waves", wavFilename)

	if host, _ := wavMeta["hostname"].(string); host != "" && host != jctx.Config.Hostname {
		cmd := fmt.Sprintf("rsync -ravp --progress %s:%s %s", host, utils.ShellEscape(wavPath), utils.ShellEscape(wavPath))
		if _, err := utils.RunCommand(cmd); err != nil {
			return err
		}
	}

	cmd := fmt.Sprintf("%s %s %d", jctx.Config.SubtitleScript, utils.ShellEscape(wavPath), content.ID)
	if _, err := utils.RunCommand(cmd); err != nil {
		return err
	}

	srtPath := filepath.Join(jctx.Config.SubtitleFolder, fmt.Sprintf("transcription_%d.srt", content.ID))
	data, err := os.ReadFile(srtPath)
	if err != nil {
		return err
	}

	subtitles := map[string]any{
		"srt": string(data),
	}
	meta["subtitles"] = subtitles
	utils.SetStatus(meta, j.QueueOutput, true)

	return jctx.Store.UpdateContentMetaStatus(ctx, content.ID, j.QueueOutput, meta)
}
