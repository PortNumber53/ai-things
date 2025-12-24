package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"ai-things/manager-go/internal/db"
	"ai-things/manager-go/internal/utils"
)

type GenerateMp3Job struct {
	BaseJob
	MaxWaiting int
}

func NewGenerateMp3Job() GenerateMp3Job {
	return GenerateMp3Job{
		BaseJob: BaseJob{
			QueueInput:  "wav_generated",
			QueueOutput: "mp3_generated",
		},
		MaxWaiting: 100,
	}
}

func (j GenerateMp3Job) Run(ctx context.Context, jctx JobContext, opts JobOptions) error {
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
		utils.Debug("GenerateMp3 waiting", "waiting", count, "max_waiting", j.MaxWaiting)
		if count >= j.MaxWaiting {
			utils.Warn("GenerateMp3 too many waiting; sleeping", "sleep_s", 60, "waiting", count, "max_waiting", j.MaxWaiting)
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

func (j GenerateMp3Job) countWaiting(ctx context.Context, jctx JobContext) (int, error) {
	where := "WHERE type = 'gemini.payload'"
	finishedTrue := db.StatusTrueCondition([]string{"funfact_created", "wav_generated", "mp3_generated"})
	finishedFalse := db.StatusNotTrueCondition([]string{"podcast_ready"})
	if finishedTrue != "" {
		where += " AND " + finishedTrue
	}
	if finishedFalse != "" {
		where += " AND " + finishedFalse
	}
	return jctx.Store.CountContent(ctx, where)
}

func (j GenerateMp3Job) selectNext(ctx context.Context, jctx JobContext) (db.Content, error) {
	where := "WHERE type = 'gemini.payload'"
	trueFlags := db.StatusTrueCondition([]string{"funfact_created", "wav_generated"})
	falseFlags := db.StatusNotTrueCondition([]string{"mp3_generated"})
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

func (j GenerateMp3Job) processContent(ctx context.Context, jctx JobContext, contentID int64) error {
	utils.Info("GenerateMp3 process", "content_id", contentID)
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
		if output, err := utils.RunCommand(cmd); err != nil {
			if !utils.FileExists(wavPath) {
				utils.Warn("GenerateMp3 wav missing on rsync; resetting wav_generated", "content_id", contentID, "host", host)
				_ = resetWavStatus(ctx, jctx, content.ID, meta)
				return nil
			}
			utils.Warn(
				"GenerateMp3 rsync failed but wav exists",
				"content_id", contentID,
				"host", host,
				"output", strings.TrimSpace(output),
				"err", err,
			)
			return err
		}
	}

	if !utils.FileExists(wavPath) {
		utils.Warn("GenerateMp3 wav not found; resetting wav_generated", "content_id", contentID, "path", wavPath)
		_ = resetWavStatus(ctx, jctx, content.ID, meta)
		return nil
	}

	if err := utils.EnsureDir(filepath.Join(jctx.Config.BaseOutputFolder, "mp3")); err != nil {
		return err
	}

	outputFile := strings.TrimSuffix(filepath.Base(wavFilename), filepath.Ext(wavFilename)) + ".mp3"
	outputPath := filepath.Join(jctx.Config.BaseOutputFolder, "mp3", outputFile)
	cmd := fmt.Sprintf("ffmpeg -y -i %s -acodec libmp3lame %s", utils.ShellEscape(wavPath), utils.ShellEscape(outputPath))
	if _, err := utils.RunCommand(cmd); err != nil {
		return err
	}

	info, err := os.Stat(outputPath)
	if err != nil {
		return err
	}
	if time.Since(info.ModTime()) > time.Minute {
		return fmt.Errorf("mp3 file is stale: %s", outputPath)
	}

	duration, err := probeDuration(outputPath)
	if err != nil {
		return err
	}

	sentenceID, _ := wavMeta["sentence_id"].(float64)
	converted := []map[string]any{
		{
			"mp3":         outputFile,
			"sentence_id": int(sentenceID),
			"duration":    duration,
			"hostname":    jctx.Config.Hostname,
		},
	}
	meta["mp3s"] = converted
	utils.SetStatus(meta, j.QueueOutput, true)

	if err := jctx.Store.UpdateContentMetaStatus(ctx, content.ID, j.QueueOutput, meta); err != nil {
		return err
	}

	payload, _ := json.Marshal(QueuePayload{ContentID: content.ID, Hostname: jctx.Config.Hostname})
	return jctx.Queue.Publish(j.QueueOutput, payload)
}

func resetWavStatus(ctx context.Context, jctx JobContext, contentID int64, meta map[string]any) error {
	delete(meta, "wav")
	utils.SetStatus(meta, "wav_generated", false)
	return jctx.Store.UpdateContentMetaStatus(ctx, contentID, "funfact_created", meta)
}

func probeDuration(path string) (float64, error) {
	cmd := fmt.Sprintf("ffmpeg -i %s 2>&1 | grep Duration", utils.ShellEscape(path))
	output, err := utils.RunCommand(cmd)
	if err != nil {
		return 0, err
	}

	re := regexp.MustCompile(`Duration: (\d+):(\d+):(\d+\.\d+)`)
	matches := re.FindStringSubmatch(output)
	if len(matches) < 4 {
		return 0, errors.New("duration not found")
	}

	hours, _ := strconv.Atoi(matches[1])
	minutes, _ := strconv.Atoi(matches[2])
	seconds, _ := strconv.ParseFloat(matches[3], 64)
	return float64(hours*3600+minutes*60) + seconds, nil
}
