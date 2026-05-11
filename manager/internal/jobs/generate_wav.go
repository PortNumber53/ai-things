package jobs

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
		utils.Debug("GenerateWav waiting", "waiting", count, "max_waiting", j.MaxWaiting)
		if count >= j.MaxWaiting {
			utils.Warn("GenerateWav too many waiting; sleeping", "sleep_s", 60, "waiting", count, "max_waiting", j.MaxWaiting)
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
	notUploaded := db.StatusNotTrueCondition([]string{"youtube_uploaded"})
	missingVideoID := db.MetaKeyMissingCondition([]string{"video_id.v1"})
	if finishedTrue != "" {
		where += " AND " + finishedTrue
	}
	if finishedFalse != "" {
		where += " AND " + finishedFalse
	}
	if notUploaded != "" {
		where += " AND " + notUploaded
	}
	if missingVideoID != "" {
		where += " AND " + missingVideoID
	}
	return jctx.Store.CountContent(ctx, where)
}

func (j GenerateWavJob) selectNext(ctx context.Context, jctx JobContext) (db.Content, error) {
	where := "WHERE type = 'gemini.payload'"
	trueFlags := db.StatusTrueCondition([]string{"funfact_created"})
	falseFlags := db.StatusNotTrueCondition([]string{"wav_generated"})
	notUploaded := db.StatusNotTrueCondition([]string{"youtube_uploaded"})
	missingVideoID := db.MetaKeyMissingCondition([]string{"video_id.v1"})
	if trueFlags != "" {
		where += " AND " + trueFlags
	}
	if falseFlags != "" {
		where += " AND " + falseFlags
	}
	if notUploaded != "" {
		where += " AND " + notUploaded
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

func (j GenerateWavJob) processContent(ctx context.Context, jctx JobContext, contentID int64) error {
	utils.Info("GenerateWav process", "content_id", contentID)
	content, err := jctx.Store.GetContentByID(ctx, contentID)
	if err != nil {
		return err
	}
	meta, err := utils.DecodeMeta(content.Meta)
	if err != nil {
		return err
	}

	// If this content was already published to YouTube (or manually overridden with a YouTube video ID),
	// skip generating upstream assets.
	if status, ok := meta["status"].(map[string]any); ok {
		if raw, ok := status["youtube_uploaded"].(string); ok && raw == "true" {
			utils.Info("GenerateWav skip (already uploaded)", "content_id", contentID)
			return nil
		}
		if raw, ok := status["youtube_uploaded"].(bool); ok && raw {
			utils.Info("GenerateWav skip (already uploaded)", "content_id", contentID)
			return nil
		}
	}
	if _, ok := meta["video_id.v1"]; ok {
		utils.Info("GenerateWav skip (video_id.v1 present)", "content_id", contentID)
		return nil
	}

	text, err := utils.ExtractTextFromMeta(meta)
	if err != nil {
		return err
	}

	voice := jctx.Config.TTSVoice
	filename := fmt.Sprintf("%010d-%03d-%s-%s.wav", content.ID, 1, voice, utils.MD5String(text))
	outputFile := filepath.Join(jctx.Config.BaseOutputFolder, "waves", filename)
	preFile := filepath.Join(jctx.Config.BaseOutputFolder, "waves", "pre-"+filename)

	piperPath := "piper"
	// If PATH doesn't include the runtime venv (common when running jobs manually),
	// try the default deploy location.
	if jctx.Config.BaseAppFolder != "" {
		deployRoot := strings.TrimRight(jctx.Config.BaseAppFolder, "/")
		// If base_app_folder points at /deploy/ai-things/current, step up one directory.
		if filepath.Base(deployRoot) == "current" {
			deployRoot = filepath.Dir(deployRoot)
		}
		candidate := filepath.Join(deployRoot, "venvs", "runtime", "bin", "piper")
		if st, err := os.Stat(candidate); err == nil && !st.IsDir() {
			piperPath = candidate
		}
	}

	cmd := fmt.Sprintf(
		"echo %s | %s --debug --sentence-silence 0.7 --model %s -c %s --output_file %s && sox %s %s pad %d %d && rm %s",
		utils.ShellEscape(text),
		utils.ShellEscape(piperPath),
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

	sha256sum, err := utils.SHA256File(outputFile)
	if err != nil {
		return err
	}

	meta["wav"] = map[string]any{
		"filename":    filename,
		"sentence_id": 0,
		"hostname":    jctx.Config.Hostname,
		"sha256":      sha256sum,
	}
	utils.SetStatus(meta, j.QueueOutput, true)

	return jctx.Store.UpdateContentMetaStatus(ctx, content.ID, j.QueueOutput, meta)
}
