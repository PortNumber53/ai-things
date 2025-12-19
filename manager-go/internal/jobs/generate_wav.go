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

type GenerateWavJob struct {
	BaseJob
	MaxWaiting int
}

func NewGenerateWavJob() GenerateWavJob {
	return GenerateWavJob{
		BaseJob: BaseJob{
			QueueInput:      "funfact_created",
			QueueOutput:     "wav_generated",
			IgnoreHostCheck: true,
		},
		MaxWaiting: 100,
	}
}

func (j GenerateWavJob) Run(ctx context.Context, jctx JobContext, opts JobOptions) error {
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
		utils.Logf("GenerateWav: waiting=%d max=%d", count, j.MaxWaiting)
		if count >= j.MaxWaiting {
			utils.Logf("GenerateWav: too many waiting, sleeping 60s")
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

func (j GenerateWavJob) countWaiting(ctx context.Context, jctx JobContext) (int, error) {
	where := "WHERE type = 'gemini.payload'"
	finishedTrue := db.StatusTrueCondition([]string{"funfact_created", "wav_generated"})
	finishedFalse := db.StatusNotTrueCondition([]string{"podcast_ready"})
	if finishedTrue != "" {
		where += " AND " + finishedTrue
	}
	if finishedFalse != "" {
		where += " AND " + finishedFalse
	}
	return jctx.Store.CountContent(ctx, where)
}

func (j GenerateWavJob) selectNext(ctx context.Context, jctx JobContext) (db.Content, error) {
	where := "WHERE type = 'gemini.payload'"
	trueFlags := db.StatusTrueCondition([]string{"funfact_created"})
	falseFlags := db.StatusNotTrueCondition([]string{"wav_generated"})
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

func (j GenerateWavJob) processContent(ctx context.Context, jctx JobContext, contentID int64) error {
	utils.Logf("GenerateWav: process content_id=%d", contentID)
	content, err := jctx.Store.GetContentByID(ctx, contentID)
	if err != nil {
		return err
	}
	meta, err := utils.DecodeMeta(content.Meta)
	if err != nil {
		return err
	}

	text, err := utils.ExtractTextFromMeta(meta)
	if err != nil {
		return err
	}

	voice := jctx.Config.TTSVoice
	filename := fmt.Sprintf("%010d-%03d-%s-%s.wav", content.ID, 1, voice, utils.MD5String(text))
	outputFile := filepath.Join(jctx.Config.BaseOutputFolder, "waves", filename)
	preFile := filepath.Join(jctx.Config.BaseOutputFolder, "waves", "pre-"+filename)

	cmd := fmt.Sprintf(
		"echo %s | piper --debug --sentence-silence 0.7 --model %s -c %s --output_file %s && sox %s %s pad %d %d && rm %s",
		utils.ShellEscape(text),
		utils.ShellEscape(jctx.Config.TTSOnnxModel),
		utils.ShellEscape(jctx.Config.TTSConfig),
		utils.ShellEscape(preFile),
		utils.ShellEscape(preFile),
		utils.ShellEscape(outputFile),
		2,
		5,
		utils.ShellEscape(preFile),
	)
	_, err = utils.RunCommand(cmd)
	if err != nil {
		return err
	}

	info, err := os.Stat(outputFile)
	if err != nil {
		return err
	}
	if time.Since(info.ModTime()) > time.Minute {
		return fmt.Errorf("output file is stale: %s", outputFile)
	}

	meta["wav"] = map[string]any{
		"filename":    filename,
		"sentence_id": 0,
		"hostname":    jctx.Config.Hostname,
	}
	utils.SetStatus(meta, j.QueueOutput, true)

	return jctx.Store.UpdateContentMetaStatus(ctx, content.ID, j.QueueOutput, meta)
}
