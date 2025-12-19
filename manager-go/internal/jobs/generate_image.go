package jobs

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"ai-things/manager-go/internal/db"
	"ai-things/manager-go/internal/utils"
)

type GenerateImageJob struct {
	BaseJob
	MaxWaiting int
}

func NewGenerateImageJob() GenerateImageJob {
	return GenerateImageJob{
		BaseJob: BaseJob{
			QueueInput:      "generate_image",
			QueueOutput:     "thumbnail_generated",
			IgnoreHostCheck: true,
		},
		MaxWaiting: 100,
	}
}

func (j GenerateImageJob) Run(ctx context.Context, jctx JobContext, opts JobOptions) error {
	if opts.Queue {
		return j.RunQueue(ctx, jctx, opts, func(ctx context.Context, contentID int64, hostname string) error {
			return j.processContent(ctx, jctx, contentID, hostname)
		})
	}

	contentID := opts.ContentID
	if contentID == 0 {
		count, err := j.countWaiting(ctx, jctx)
		if err != nil {
			return err
		}
		utils.Logf("GenerateImage: waiting=%d max=%d", count, j.MaxWaiting)
		if count >= j.MaxWaiting {
			utils.Logf("GenerateImage: too many waiting, sleeping 60s")
			time.Sleep(60 * time.Second)
			return nil
		}

		content, err := j.selectNext(ctx, jctx)
		if err != nil {
			return err
		}
		contentID = content.ID
	}

	return j.processContent(ctx, jctx, contentID, "")
}

func (j GenerateImageJob) countWaiting(ctx context.Context, jctx JobContext) (int, error) {
	where := "WHERE type = 'gemini.payload'"
	finishedTrue := db.StatusTrueCondition([]string{"funfact_created", "wav_generated", "mp3_generated", "srt_generated", "thumbnail_generated"})
	finishedFalse := db.StatusNotTrueCondition([]string{"podcast_ready"})
	if finishedTrue != "" {
		where += " AND " + finishedTrue
	}
	if finishedFalse != "" {
		where += " AND " + finishedFalse
	}
	return jctx.Store.CountContent(ctx, where)
}

func (j GenerateImageJob) selectNext(ctx context.Context, jctx JobContext) (db.Content, error) {
	where := "WHERE type = 'gemini.payload'"
	trueFlags := db.StatusTrueCondition([]string{"funfact_created"})
	falseFlags := db.StatusNotTrueCondition([]string{"thumbnail_generated"})
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

func (j GenerateImageJob) processContent(ctx context.Context, jctx JobContext, contentID int64, targetHostname string) error {
	utils.Logf("GenerateImage: process content_id=%d target_host=%s", contentID, targetHostname)
	content, err := jctx.Store.GetContentByID(ctx, contentID)
	if err != nil {
		return err
	}

	payload := map[string]any{
		"prompt":          content.Title,
		"steps":           32,
		"width":           800,
		"height":          600,
		"negative_prompt": "lowres, bad anatomy, bad hands, text, error, missing fingers, extra digit, fewer digits, cropped, worst quality, low quality, normal quality, jpeg artifacts, signature, watermark, username, blurry",
		"enable_hr":       true,
		"restore_faces":   true,
		"hr_upscaler":     "Nearest",
		"denoising_strength": 0.7,
	}
	body, _ := json.Marshal(payload)

	client := &http.Client{Timeout: 300 * time.Second}
	resp, err := client.Post("http://192.168.68.70:7860/sdapi/v1/txt2img", "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var response struct {
		Images []string `json:"images"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return err
	}
	if len(response.Images) == 0 {
		return errors.New("no images returned")
	}

	imageData, err := base64.StdEncoding.DecodeString(response.Images[0])
	if err != nil {
		return err
	}

	filename := fmt.Sprintf("%010d.jpg", content.ID)
	fullPath := filepath.Join(jctx.Config.BaseOutputFolder, "images", filename)
	if err := utils.EnsureDir(filepath.Dir(fullPath)); err != nil {
		return err
	}
	if err := os.WriteFile(fullPath, imageData, 0o644); err != nil {
		return err
	}

	if targetHostname != "" && targetHostname != jctx.Config.Hostname {
		cmd := fmt.Sprintf("scp -v %s %s:%s", utils.ShellEscape(fullPath), targetHostname, utils.ShellEscape(fullPath))
		if _, err := utils.RunCommand(cmd); err != nil {
			return err
		}
	}

	meta, err := utils.DecodeMeta(content.Meta)
	if err != nil {
		return err
	}
	meta["thumbnail"] = map[string]any{
		"filename": filename,
		"hostname": jctx.Config.Hostname,
	}
	utils.SetStatus(meta, j.QueueOutput, true)

	if err := jctx.Store.UpdateContentMetaStatus(ctx, content.ID, j.QueueOutput, meta); err != nil {
		return err
	}

	payloadMsg, _ := json.Marshal(QueuePayload{ContentID: content.ID, Hostname: jctx.Config.Hostname})
	return jctx.Queue.Publish(j.QueueOutput, payloadMsg)
}
