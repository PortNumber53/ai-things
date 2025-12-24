package jobs

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ai-things/manager-go/internal/db"
	"ai-things/manager-go/internal/utils"
)

type PromptForImageJob struct {
	BaseJob
	MaxWaiting int
}

func NewPromptForImageJob() PromptForImageJob {
	return PromptForImageJob{
		BaseJob: BaseJob{
			QueueInput:  "generate_image",
			QueueOutput: "thumbnail_generated",
		},
		MaxWaiting: 100,
	}
}

func (j PromptForImageJob) Run(ctx context.Context, jctx JobContext, opts JobOptions) error {
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
		utils.Logf("PromptForImage: waiting=%d max=%d", count, j.MaxWaiting)
		if count >= j.MaxWaiting {
			utils.Logf("PromptForImage: too many waiting, sleeping 60s")
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

func (j PromptForImageJob) countWaiting(ctx context.Context, jctx JobContext) (int, error) {
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

func (j PromptForImageJob) selectNext(ctx context.Context, jctx JobContext) (db.Content, error) {
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

func (j PromptForImageJob) processContent(ctx context.Context, jctx JobContext, contentID int64, regenerate bool) error {
	utils.Logf("PromptForImage: process content_id=%d regenerate=%t", contentID, regenerate)
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

	prompt := buildImagePrompt(text)
	if jctx.Config.Portnumber53APIKey == "" {
		return errors.New("missing PORTNUMBER53_API_KEY")
	}

	requestBody := map[string]any{
		"model":  "llama3.3",
		"stream": false,
		"prompt": prompt,
	}
	payload, _ := json.Marshal(requestBody)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://ollama.portnumber53.com/api/generate", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-key", jctx.Config.Portnumber53APIKey)

	timeout := jctx.Config.Portnumber53TimeoutSeconds
	if timeout <= 0 {
		timeout = 1000
	}
	client := &http.Client{Timeout: time.Duration(timeout) * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var response struct {
		Response string `json:"response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return err
	}
	bodyResponse := strings.Trim(response.Response, "\"")

	filename := fmt.Sprintf("%010d.jpg", content.ID)
	fullPath := filepath.Join(jctx.Config.BaseOutputFolder, "images", filename)

	if regenerate && utils.FileExists(fullPath) {
		_ = os.Remove(fullPath)
	}

	imageScript := filepath.Join(jctx.Config.BaseAppFolder, "imagegeneration", "image-flux.py")
	for !utils.FileExists(fullPath) {
		cmd := fmt.Sprintf("python %s %s %s", utils.ShellEscape(imageScript), utils.ShellEscape(fullPath), utils.ShellEscape(bodyResponse))
		if _, err := utils.RunCommand(cmd); err != nil {
			return err
		}
		time.Sleep(2 * time.Second)
	}

	meta["thumbnail"] = map[string]any{
		"filename": filename,
		"hostname": jctx.Config.Hostname,
	}
	utils.SetStatus(meta, j.QueueOutput, true)

	return jctx.Store.UpdateContentMetaStatus(ctx, content.ID, j.QueueOutput, meta)
}

func buildImagePrompt(text string) string {
	return fmt.Sprintf(`SYSTEM """
-You are an experience designer and artist.
-You are tasked with providing a prompt that will be used to generate an image representing the content of a text.
-The prompt should be a short sentence or two that captures the essence of the text.
- Do not include any preamble, or comments, or introduction, or explanation, or commentary, or any other additional text.
- Only output the prompt, nothing else.
- Make sure to include the name of the place, or subject to help the AI generate an accurate image.
"""
USER """
%s
"""`, text)
}
