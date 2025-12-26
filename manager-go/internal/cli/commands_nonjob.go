package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode"

	"ai-things/manager-go/internal/db"
	"ai-things/manager-go/internal/jobs"
	"ai-things/manager-go/internal/utils"
)

func runAiGenerateFunFacts(ctx context.Context, jctx jobs.JobContext, args []string) error {
	fs := flag.NewFlagSet("Ai:GenerateFunFacts", flag.ContinueOnError)
	sleep := fs.Int("sleep", 30, "Sleep duration in seconds")
	queueFlag := fs.Bool("queue", false, "Process queue messages")
	verbose := fs.Bool("verbose", utils.Verbose, "Verbose logging")
	if err := fs.Parse(args); err != nil {
		return err
	}
	utils.ConfigureLogging(*verbose)
	_ = sleep
	_ = queueFlag

	contentID, err := parseContentID(fs.Args())
	if err != nil {
		return err
	}
	utils.Info(
		"Ai:GenerateFunFacts start",
		"content_id", contentID,
		"host", jctx.Config.OllamaHostname,
		"port", jctx.Config.OllamaPort,
		"model", jctx.Config.OllamaModel,
	)

	text, title, paragraphs, count, err := generateOllamaFunFact(ctx, jctx.Config.OllamaHostname, jctx.Config.OllamaPort, jctx.Config.OllamaModel)
	if err != nil {
		return err
	}
	_ = text
	utils.Debug("Ai:GenerateFunFacts response", "title_len", len(title), "paragraphs", len(paragraphs), "count", count)

	metaJSON, err := json.Marshal(map[string]any{})
	if err != nil {
		return err
	}
	sentencesJSON, err := json.Marshal(paragraphs)
	if err != nil {
		return err
	}

	if contentID == 0 {
		status := "new"
		contentType := "text-to-tts"
		_, err := jctx.Store.CreateContent(ctx, db.Content{
			Title:     title,
			Status:    &status,
			Type:      &contentType,
			Sentences: sentencesJSON,
			Count:     count,
			Meta:      metaJSON,
		})
		return err
	}

	return jctx.Store.UpdateContentText(ctx, contentID, title, sentencesJSON, count, metaJSON)
}

func generateOllamaFunFact(ctx context.Context, ollamaHostname string, ollamaPort int, ollamaModel string) (string, string, []map[string]any, int, error) {
	prompt := strings.TrimSpace(`Write 6 to 10 paragraphs about a single unique random fact about Earth's Rotation,
make the explanation engaging while keeping it simple.
Your response must be in format structured exactly like this, no extra formatting required:
TITLE: The title for the subject comes here
CONTENT: Your entire fun fact goes here.`)

	if ollamaHostname == "" {
		return "", "", nil, 0, errors.New("missing ollama hostname (set ollama.hostname in config.ini)")
	}
	if ollamaPort == 0 {
		ollamaPort = 11434
	}
	if strings.TrimSpace(ollamaModel) == "" {
		ollamaModel = "llama3.2"
	}
	utils.Debug(
		"ollama generate",
		"url", fmt.Sprintf("http://%s:%d/api/generate", ollamaHostname, ollamaPort),
		"model", ollamaModel,
		"prompt_len", len(prompt),
	)

	payload := map[string]any{
		"model":      ollamaModel,
		"keep_alive": 300,
		"prompt":     prompt,
		"stream":     false,
		"options": map[string]any{
			"seed":        time.Now().Unix(),
			"temperature": 1,
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", "", nil, 0, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("http://%s:%d/api/generate", ollamaHostname, ollamaPort), bytes.NewReader(body))
	if err != nil {
		return "", "", nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 600 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", nil, 0, err
	}
	defer resp.Body.Close()
	utils.Debug("ollama response", "status", resp.StatusCode)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "", nil, 0, fmt.Errorf("ollama response status %d", resp.StatusCode)
	}

	var decoded struct {
		Response string `json:"response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return "", "", nil, 0, err
	}
	utils.Debug("ollama decoded", "response_len", len(decoded.Response))

	title := ""
	paragraphs := []map[string]any{}
	count := 0
	prevSpacer := false
	lines := strings.Split(decoded.Response, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "TITLE:") {
			title = strings.TrimSpace(strings.TrimPrefix(line, "TITLE:"))
		} else if line != "" {
			line = strings.TrimSpace(strings.TrimPrefix(line, "CONTENT:"))
			for _, sentence := range splitSentences(line, ".!?") {
				if strings.TrimSpace(sentence) == "" {
					continue
				}
				count++
				paragraphs = append(paragraphs, map[string]any{
					"count":   count,
					"content": strings.TrimSpace(sentence),
				})
			}
			prevSpacer = false
		}
		if !prevSpacer {
			count++
			paragraphs = append(paragraphs, map[string]any{
				"count":   count,
				"content": "<spacer>",
			})
			prevSpacer = true
		}
	}

	return decoded.Response, title, paragraphs, count, nil
}

func runAiSplitText(ctx context.Context, jctx jobs.JobContext, args []string) error {
	fs := flag.NewFlagSet("Ai:SplitText", flag.ContinueOnError)
	verbose := fs.Bool("verbose", utils.Verbose, "Verbose logging")
	if err := fs.Parse(args); err != nil {
		return err
	}
	utils.ConfigureLogging(*verbose)

	if jctx.Queue == nil {
		return errors.New("queue client is not configured")
	}

	payload, _ := json.Marshal(map[string]any{
		"payload": "",
	})
	return jctx.Queue.Publish("tts_split_text", payload)
}

func runBackfillResponseDataToSentences(ctx context.Context, jctx jobs.JobContext, args []string) error {
	fs := flag.NewFlagSet("Backfill:ResponseDataToSentences", flag.ContinueOnError)
	logOnly := fs.Bool("log", false, "Log only, suppress console output")
	verbose := fs.Bool("verbose", utils.Verbose, "Verbose logging")
	if err := fs.Parse(args); err != nil {
		return err
	}
	utils.ConfigureLogging(*verbose)

	contentID, err := parseContentID(fs.Args())
	if err != nil {
		return err
	}

	if contentID != 0 {
		return backfillContentOriginalText(ctx, jctx, contentID, *logOnly)
	}

	lastID := int64(0)
	for {
		contents, err := listContentBatch(ctx, jctx.Store, "WHERE (meta->>'original_text' IS NULL OR meta->>'original_text' = '')", nil, lastID, 100)
		if err != nil {
			return err
		}
		if len(contents) == 0 {
			break
		}
		for _, content := range contents {
			if err := backfillContentOriginalText(ctx, jctx, content.ID, *logOnly); err != nil {
				return err
			}
			lastID = content.ID
		}
	}
	return nil
}

func backfillContentOriginalText(ctx context.Context, jctx jobs.JobContext, contentID int64, logOnly bool) error {
	content, err := jctx.Store.GetContentByID(ctx, contentID)
	if err != nil {
		return err
	}
	meta, err := utils.DecodeMeta(content.Meta)
	if err != nil {
		return err
	}

	originalText := extractOriginalText(meta)
	if originalText == "" {
		return nil
	}

	if !logOnly {
		fmt.Printf("content_id=%d original_text=%q\n", contentID, originalText)
	}

	meta["original_text"] = originalText
	return jctx.Store.UpdateContentMeta(ctx, contentID, meta)
}

func runCheckImageIsGenerated(ctx context.Context, jctx jobs.JobContext, args []string) error {
	return checkGeneratedFiles(ctx, jctx, args, "thumbnail_generated", func(content db.Content, meta map[string]any) (bool, string, error) {
		thumb, ok := meta["thumbnail"].(map[string]any)
		if !ok {
			return true, "thumbnail meta missing", nil
		}
		filename, _ := thumb["filename"].(string)
		if filename == "" {
			return true, "thumbnail filename missing", nil
		}
		imgPath := filepath.Join(jctx.Config.BaseOutputFolder, "images", filename)
		if !utils.FileExists(imgPath) {
			return true, "thumbnail file missing", nil
		}

		if wantSHA, _ := thumb["sha256"].(string); wantSHA != "" {
			haveSHA, err := utils.SHA256File(imgPath)
			if err != nil {
				return false, "", err
			}
			if haveSHA != wantSHA {
				return true, "thumbnail checksum mismatch", nil
			}
		}
		return false, "thumbnail ok", nil
	}, func(meta map[string]any) {
		if status, ok := meta["status"].(map[string]any); ok {
			delete(status, "thumbnail")
		}
		delete(meta, "thumbnail")
		utils.SetStatus(meta, "thumbnail_generated", false)
		utils.SetStatus(meta, "podcast_ready", false)
	})
}

func runCheckMp3IsGenerated(ctx context.Context, jctx jobs.JobContext, args []string) error {
	return checkGeneratedFiles(ctx, jctx, args, "mp3_generated", func(content db.Content, meta map[string]any) (bool, string, error) {
		mp3s, ok := meta["mp3s"].([]any)
		if !ok || len(mp3s) == 0 {
			return true, "mp3s meta missing/empty", nil
		}
		for _, entry := range mp3s {
			mp3Meta, ok := entry.(map[string]any)
			if !ok {
				return true, "mp3s meta invalid", nil
			}
			filename, _ := mp3Meta["mp3"].(string)
			if filename == "" {
				return true, "mp3 filename missing", nil
			}
			mp3Path := filepath.Join(jctx.Config.BaseOutputFolder, "mp3", filename)
			if !utils.FileExists(mp3Path) {
				return true, "mp3 file missing", nil
			}
			if wantSHA, _ := mp3Meta["sha256"].(string); wantSHA != "" {
				haveSHA, err := utils.SHA256File(mp3Path)
				if err != nil {
					return false, "", err
				}
				if haveSHA != wantSHA {
					return true, "mp3 checksum mismatch", nil
				}
			}
		}
		return false, "mp3 ok", nil
	}, func(meta map[string]any) {
		delete(meta, "mp3s")
		utils.SetStatus(meta, "mp3_generated", false)
		utils.SetStatus(meta, "podcast_ready", false)
	})
}

func runCheckPodcastIsGenerated(ctx context.Context, jctx jobs.JobContext, args []string) error {
	return checkGeneratedFiles(ctx, jctx, args, "podcast_ready", func(content db.Content, meta map[string]any) (bool, string, error) {
		podcast, ok := meta["podcast"].(map[string]any)
		if !ok {
			return true, "podcast meta missing", nil
		}
		filename, _ := podcast["filename"].(string)
		if filename == "" {
			return true, "podcast filename missing", nil
		}
		podcastPath := filepath.Join(jctx.Config.BaseOutputFolder, "podcast", filename)
		if !utils.FileExists(podcastPath) {
			return true, "podcast file missing", nil
		}
		if wantSHA, _ := podcast["sha256"].(string); wantSHA != "" {
			haveSHA, err := utils.SHA256File(podcastPath)
			if err != nil {
				return false, "", err
			}
			if haveSHA != wantSHA {
				return true, "podcast checksum mismatch", nil
			}
		}
		return false, "podcast ok", nil
	}, func(meta map[string]any) {
		delete(meta, "podcast")
		utils.SetStatus(meta, "podcast_ready", false)
	})
}

func runCheckYoutubeIsUploadable(ctx context.Context, jctx jobs.JobContext, args []string) error {
	where := "WHERE type = 'gemini.payload'"
	trueFlags := db.StatusTrueCondition([]string{"funfact_created", "wav_generated", "mp3_generated", "srt_generated", "thumbnail_generated", "podcast_ready", "youtube_approved"})
	notTrue := db.StatusNotTrueCondition([]string{"youtube_uploaded"})
	notRejected := db.StatusNotTrueCondition([]string{"youtube_rejected"})
	missing := db.MetaKeyMissingCondition([]string{"video_id.v1"})
	if trueFlags != "" {
		where += " AND " + trueFlags
	}
	if notTrue != "" {
		where += " AND " + notTrue
	}
	if notRejected != "" {
		where += " AND " + notRejected
	}
	if missing != "" {
		where += " AND " + missing
	}

	// Validate the same artifact UploadPodcastToYoutube needs: local podcast mp4 is present (and matches sha256 if recorded).
	return checkGeneratedFilesWhere(ctx, jctx, args, "YoutubeIsUploadable", where, func(content db.Content, meta map[string]any) (bool, string, error) {
		if approved, ok := utils.GetStatus(meta, "youtube_approved"); !ok || !approved {
			if rejected, _ := utils.GetStatus(meta, "youtube_rejected"); rejected {
				return true, "youtube rejected (youtube_rejected=true)", nil
			}
			return true, "awaiting slack approval (youtube_approved!=true)", nil
		}
		podcast, ok := meta["podcast"].(map[string]any)
		if !ok {
			return true, "podcast meta missing", nil
		}
		filename, _ := podcast["filename"].(string)
		if filename == "" {
			return true, "podcast filename missing", nil
		}
		// If the podcast was rendered on another host, treat it as uploadable even if it's not
		// present locally yet (the upload job will rsync it).
		if host, _ := podcast["hostname"].(string); host != "" && host != jctx.Config.Hostname {
			return false, "podcast on remote host (will rsync on upload)", nil
		}
		podcastPath := filepath.Join(jctx.Config.BaseOutputFolder, "podcast", filename)
		if !utils.FileExists(podcastPath) {
			return true, "podcast file missing", nil
		}
		if wantSHA, _ := podcast["sha256"].(string); wantSHA != "" {
			haveSHA, err := utils.SHA256File(podcastPath)
			if err != nil {
				return false, "", err
			}
			if haveSHA != wantSHA {
				return true, "podcast checksum mismatch", nil
			}
		}
		return false, "podcast ok", nil
	}, func(meta map[string]any) {
		delete(meta, "podcast")
		utils.SetStatus(meta, "podcast_ready", false)
	})
}

func runCheckYoutubeUploadEligibility(ctx context.Context, jctx jobs.JobContext, args []string) error {
	fs := flag.NewFlagSet("Check:YoutubeUploadEligibility", flag.ContinueOnError)
	verbose := fs.Bool("verbose", utils.Verbose, "Verbose logging")
	if err := fs.Parse(args); err != nil {
		return err
	}
	utils.ConfigureLogging(*verbose)

	contentID, err := parseContentID(fs.Args())
	if err != nil {
		return err
	}

	// Evaluate eligibility for a single content_id if provided.
	if contentID != 0 {
		content, err := jctx.Store.GetContentByID(ctx, contentID)
		if err != nil {
			return err
		}
		meta, err := utils.DecodeMeta(content.Meta)
		if err != nil {
			return err
		}
		logYoutubeUploadEligibility(jctx, content, meta)
		return nil
	}

	// Otherwise, scan podcast_ready rows and explain why each is (not) pending upload.
	where := "WHERE type = 'gemini.payload'"
	ready := db.StatusTrueCondition([]string{"podcast_ready"})
	if ready != "" {
		where += " AND " + ready
	}

	checked := 0
	lastID := int64(0)
	for {
		contents, err := listContentBatch(ctx, jctx.Store, where, nil, lastID, 500)
		if err != nil {
			return err
		}
		if len(contents) == 0 {
			break
		}
		for _, content := range contents {
			meta, err := utils.DecodeMeta(content.Meta)
			if err != nil {
				return err
			}
			logYoutubeUploadEligibility(jctx, content, meta)
			checked++
			lastID = content.ID
		}
	}
	utils.Info("check summary", "check", "YoutubeUploadEligibility", "checked", checked)
	return nil
}

func logYoutubeUploadEligibility(jctx jobs.JobContext, content db.Content, meta map[string]any) {
	// Mirrors UploadPodcastToYoutube selection logic.
	videoID, hasVideoID := meta["video_id.v1"]
	youtubeUploaded := false
	if status, ok := meta["status"].(map[string]any); ok {
		if raw, ok := status["youtube_uploaded"].(string); ok && raw == "true" {
			youtubeUploaded = true
		}
		if raw, ok := status["youtube_uploaded"].(bool); ok && raw {
			youtubeUploaded = true
		}
	}

	if youtubeUploaded {
		utils.Info("youtube upload eligibility", "content_id", content.ID, "decision", "skip", "reason", "youtube_uploaded=true")
		return
	}
	if hasVideoID && videoID != nil {
		utils.Info("youtube upload eligibility", "content_id", content.ID, "decision", "skip", "reason", "video_id.v1 present (manual override)")
		return
	}

	// Ensure required upstream flags are true (same as UploadPodcastToYoutube).
	required := []string{"funfact_created", "wav_generated", "mp3_generated", "srt_generated", "thumbnail_generated", "podcast_ready"}
	var missing []string
	if status, ok := meta["status"].(map[string]any); ok {
		for _, k := range required {
			v, ok := status[k]
			if !ok {
				missing = append(missing, k)
				continue
			}
			switch vv := v.(type) {
			case string:
				if vv != "true" {
					missing = append(missing, k)
				}
			case bool:
				if !vv {
					missing = append(missing, k)
				}
			default:
				missing = append(missing, k)
			}
		}
	} else {
		missing = required
	}
	if len(missing) > 0 {
		utils.Warn("youtube upload eligibility", "content_id", content.ID, "decision", "skip", "reason", "missing required flags", "missing", strings.Join(missing, ","))
		return
	}

	// Validate local file presence (and checksum if present).
	podcast, ok := meta["podcast"].(map[string]any)
	if !ok {
		utils.Warn("youtube upload eligibility", "content_id", content.ID, "decision", "flagged", "reason", "podcast meta missing")
		return
	}
	filename, _ := podcast["filename"].(string)
	if filename == "" {
		utils.Warn("youtube upload eligibility", "content_id", content.ID, "decision", "flagged", "reason", "podcast filename missing")
		return
	}
	if host, _ := podcast["hostname"].(string); host != "" && host != jctx.Config.Hostname {
		utils.Info(
			"youtube upload eligibility",
			"content_id", content.ID,
			"decision", "pending",
			"reason", "podcast on remote host (will rsync on upload)",
			"host", host,
			"filename", filename,
		)
		return
	}
	podcastPath := filepath.Join(jctx.Config.BaseOutputFolder, "podcast", filename)
	if !utils.FileExists(podcastPath) {
		utils.Warn("youtube upload eligibility", "content_id", content.ID, "decision", "flagged", "reason", "podcast file missing", "path", podcastPath)
		return
	}
	if wantSHA, _ := podcast["sha256"].(string); wantSHA != "" {
		haveSHA, err := utils.SHA256File(podcastPath)
		if err != nil {
			utils.Warn("youtube upload eligibility", "content_id", content.ID, "decision", "flagged", "reason", "sha256 read failed", "err", err)
			return
		}
		if haveSHA != wantSHA {
			utils.Warn("youtube upload eligibility", "content_id", content.ID, "decision", "flagged", "reason", "podcast checksum mismatch", "path", podcastPath, "want_sha256", wantSHA, "have_sha256", haveSHA)
			return
		}
	}

	utils.Info("youtube upload eligibility", "content_id", content.ID, "decision", "pending", "reason", "eligible for upload")
}

func runCheckSrtIsGenerated(ctx context.Context, jctx jobs.JobContext, args []string) error {
	return checkGeneratedFiles(ctx, jctx, args, "srt_generated", func(content db.Content, meta map[string]any) (bool, string, error) {
		srtPath := filepath.Join(jctx.Config.SubtitleFolder, fmt.Sprintf("transcription_%d.srt", content.ID))
		if utils.FileExists(srtPath) {
			return false, "srt ok", nil
		}
		return true, "srt file missing", nil
	}, func(meta map[string]any) {
		delete(meta, "subtitles")
		utils.SetStatus(meta, "srt_generated", false)
		utils.SetStatus(meta, "srt_fixed", false)
		utils.SetStatus(meta, "podcast_ready", false)
	})
}

func runCheckWavIsGenerated(ctx context.Context, jctx jobs.JobContext, args []string) error {
	return checkGeneratedFiles(ctx, jctx, args, "wav_generated", func(content db.Content, meta map[string]any) (bool, string, error) {
		wav, ok := meta["wav"].(map[string]any)
		if !ok {
			return true, "wav meta missing", nil
		}
		filename, _ := wav["filename"].(string)
		if filename == "" {
			return true, "wav filename missing", nil
		}
		wavPath := filepath.Join(jctx.Config.BaseOutputFolder, "waves", filename)
		if !utils.FileExists(wavPath) {
			return true, "wav file missing", nil
		}
		if wantSHA, _ := wav["sha256"].(string); wantSHA != "" {
			haveSHA, err := utils.SHA256File(wavPath)
			if err != nil {
				return false, "", err
			}
			if haveSHA != wantSHA {
				return true, "wav checksum mismatch", nil
			}
		}
		return false, "wav ok", nil
	}, func(meta map[string]any) {
		delete(meta, "wav")
		utils.SetStatus(meta, "wav_generated", false)
		utils.SetStatus(meta, "podcast_ready", false)
	})
}

type checkResetter func(meta map[string]any)
type checkPredicate func(content db.Content, meta map[string]any) (bool, string, error)

func checkGeneratedFiles(ctx context.Context, jctx jobs.JobContext, args []string, statusKey string, predicate checkPredicate, reset checkResetter) error {
	where := ""
	if statusKey != "" {
		cond := db.StatusTrueCondition([]string{statusKey})
		if cond != "" {
			where = "WHERE " + cond
		}
	}
	return checkGeneratedFilesWhere(ctx, jctx, args, statusKey, where, predicate, reset)
}

func checkGeneratedFilesWhere(ctx context.Context, jctx jobs.JobContext, args []string, checkName string, where string, predicate checkPredicate, reset checkResetter) error {
	fs := flag.NewFlagSet("Check:"+checkName, flag.ContinueOnError)
	verbose := fs.Bool("verbose", utils.Verbose, "Verbose logging")
	if err := fs.Parse(args); err != nil {
		return err
	}
	utils.ConfigureLogging(*verbose)

	checked := 0
	valid := 0
	flagged := 0
	fixed := 0
	lastID := int64(0)
	for {
		contents, err := listContentBatch(ctx, jctx.Store, where, nil, lastID, 500)
		if err != nil {
			return err
		}
		if len(contents) == 0 {
			break
		}
		for _, content := range contents {
			meta, err := utils.DecodeMeta(content.Meta)
			if err != nil {
				return err
			}
			needsReset, reason, err := predicate(content, meta)
			if err != nil {
				return err
			}
			checked++
			if needsReset {
				flagged++
				utils.Warn("check row", "check", checkName, "content_id", content.ID, "decision", "flagged", "reason", reason)
				reset(meta)
				if err := jctx.Store.UpdateContentMeta(ctx, content.ID, meta); err != nil {
					return err
				}
				fixed++
			} else {
				valid++
				if utils.Verbose {
					utils.Debug("check row", "check", checkName, "content_id", content.ID, "decision", "valid", "reason", reason)
				}
			}
			lastID = content.ID
		}
	}

	utils.Info("check summary", "check", checkName, "checked", checked, "valid", valid, "flagged", flagged, "updated", fixed)
	fmt.Printf("Fixed %d rows\n", fixed)
	return nil
}

func runContentFindDuplicateTitles(ctx context.Context, jctx jobs.JobContext, args []string) error {
	fs := flag.NewFlagSet("Content:FindDuplicateTitles", flag.ContinueOnError)
	verbose := fs.Bool("verbose", utils.Verbose, "Verbose logging")
	if err := fs.Parse(args); err != nil {
		return err
	}
	utils.ConfigureLogging(*verbose)

	rows, err := jctx.Store.QueryContents(ctx, `
		SELECT id, title, status, type, sentences, count, meta, archive, created_at, updated_at
		FROM contents
		WHERE title IN (
			SELECT title
			FROM contents
			GROUP BY title
			HAVING COUNT(id) > 1
		)
		ORDER BY title, id
	`)
	if err != nil {
		return err
	}

	byTitle := map[string][]db.Content{}
	for _, content := range rows {
		byTitle[content.Title] = append(byTitle[content.Title], content)
	}

	for title, contents := range byTitle {
		bestID := pickBestDuplicate(contents)
		for _, content := range contents {
			if content.ID == bestID {
				continue
			}
			if err := archiveContentAndRegenerate(ctx, jctx, content); err != nil {
				return fmt.Errorf("title %q id %d: %w", title, content.ID, err)
			}
		}
	}
	return nil
}

func pickBestDuplicate(contents []db.Content) int64 {
	if len(contents) == 0 {
		return 0
	}
	bestID := contents[0].ID
	maxViews := -1
	maxComments := -1
	for _, content := range contents {
		meta, err := utils.DecodeMeta(content.Meta)
		if err != nil {
			continue
		}
		viewCount := metaInt(meta, "view_count")
		comments := metaInt(meta, "comments")
		if viewCount > maxViews || (viewCount == maxViews && comments > maxComments) {
			maxViews = viewCount
			maxComments = comments
			bestID = content.ID
		}
	}
	return bestID
}

func archiveContentAndRegenerate(ctx context.Context, jctx jobs.JobContext, content db.Content) error {
	archiveEntry := map[string]any{
		"data": map[string]any{
			"title":      content.Title,
			"status":     content.Status,
			"type":       content.Type,
			"sentences":  string(content.Sentences),
			"count":      content.Count,
			"meta":       string(content.Meta),
			"created_at": content.CreatedAt,
			"updated_at": content.UpdatedAt,
		},
	}

	var archive map[string]any
	if len(content.Archive) > 0 {
		if err := json.Unmarshal(content.Archive, &archive); err != nil {
			return err
		}
	}
	if archive == nil {
		archive = map[string]any{}
	}

	if isArchiveDuplicate(archive, archiveEntry) {
		return runGeminiGenerateFunFact(ctx, jctx, []string{fmt.Sprintf("%d", content.ID)})
	}

	timestamp := time.Now().Format("20060102150405")
	for {
		if _, exists := archive[timestamp]; !exists {
			break
		}
		timestamp += "1"
	}
	archive[timestamp] = archiveEntry

	archiveJSON, err := json.Marshal(archive)
	if err != nil {
		return err
	}
	if err := jctx.Store.UpdateContentArchive(ctx, content.ID, archiveJSON); err != nil {
		return err
	}

	return runGeminiGenerateFunFact(ctx, jctx, []string{fmt.Sprintf("%d", content.ID)})
}

func isArchiveDuplicate(archive map[string]any, entry map[string]any) bool {
	entryData, _ := json.Marshal(entry["data"])
	for _, value := range archive {
		item, ok := value.(map[string]any)
		if !ok {
			continue
		}
		data, ok := item["data"]
		if !ok {
			continue
		}
		encoded, err := json.Marshal(data)
		if err != nil {
			continue
		}
		if bytes.Equal(encoded, entryData) {
			return true
		}
	}
	return false
}

func metaInt(meta map[string]any, key string) int {
	value, ok := meta[key]
	if !ok {
		return 0
	}
	switch v := value.(type) {
	case float64:
		return int(v)
	case int:
		return v
	case string:
		i, _ := strconv.Atoi(v)
		return i
	default:
		return 0
	}
}

func runContentIdentifySubject(ctx context.Context, jctx jobs.JobContext, args []string) error {
	fs := flag.NewFlagSet("Content:IdentifySubject", flag.ContinueOnError)
	contentID := fs.Int64("content-id", 0, "The ID of the content to analyze")
	verbose := fs.Bool("verbose", utils.Verbose, "Verbose logging")
	if err := fs.Parse(args); err != nil {
		return err
	}
	utils.ConfigureLogging(*verbose)

	if *contentID == 0 {
		return errors.New("content-id is required")
	}
	content, err := jctx.Store.GetContentByID(ctx, *contentID)
	if err != nil {
		return err
	}

	meta, err := utils.DecodeMeta(content.Meta)
	if err != nil {
		return err
	}
	meta["subject"] = "pending_ai_analysis"
	return jctx.Store.UpdateContentMeta(ctx, content.ID, meta)
}

func runContentQuery(ctx context.Context, jctx jobs.JobContext, args []string) error {
	fs := flag.NewFlagSet("content:query", flag.ContinueOnError)
	verbose := fs.Bool("verbose", utils.Verbose, "Verbose logging")
	if err := fs.Parse(args); err != nil {
		return err
	}
	utils.ConfigureLogging(*verbose)

	startID, endID, err := parseOptionalRange(fs.Args())
	if err != nil {
		return err
	}

	if jctx.Queue == nil {
		return errors.New("queue client is not configured")
	}

	lastID := int64(0)
	for {
		query := `
			SELECT id, title, status, type, sentences, count, meta, archive, created_at, updated_at
			FROM contents
			WHERE status = $1 AND type = $2 AND id > $3
		`
		args := []any{"funfact.created", "gemini.payload", lastID}
		if startID != nil && endID != nil {
			query += " AND id BETWEEN $4 AND $5"
			args = append(args, *startID, *endID)
		}
		query += " ORDER BY id LIMIT 10"
		contents, err := jctx.Store.QueryContents(ctx, query, args...)
		if err != nil {
			return err
		}
		if len(contents) == 0 {
			break
		}
		for _, content := range contents {
			payload, _ := json.Marshal(map[string]any{"content_id": content.ID})
			if err := jctx.Queue.Publish("funfact.created", payload); err != nil {
				return err
			}
			lastID = content.ID
		}
	}
	return nil
}

func runGeminiGenerateFunFact(ctx context.Context, jctx jobs.JobContext, args []string) error {
	fs := flag.NewFlagSet("Gemini:GenerateFunFact", flag.ContinueOnError)
	verbose := fs.Bool("verbose", utils.Verbose, "Verbose logging")
	if err := fs.Parse(args); err != nil {
		return err
	}
	utils.ConfigureLogging(*verbose)

	contentID, err := parseContentID(fs.Args())
	if err != nil {
		return err
	}

	if contentID == 0 {
		count, err := jctx.Store.CountContent(ctx, "WHERE "+db.StatusTrueCondition([]string{"funfact_created"}))
		if err != nil {
			return err
		}
		if count >= 100 {
			time.Sleep(60 * time.Second)
			return nil
		}
		ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)
		defer stop()
		for {
			if ctx.Err() != nil {
				return nil
			}
			if err := generateGeminiFunFact(ctx, jctx, 0); err != nil {
				return err
			}
			time.Sleep(2 * time.Second)
		}
	}

	return generateGeminiFunFact(ctx, jctx, contentID)
}

func generateGeminiFunFact(ctx context.Context, jctx jobs.JobContext, contentID int64) error {
	apiKey := jctx.Config.GeminiAPIKey
	if apiKey == "" {
		return errors.New("missing gemini api key (set gemini.api_key in config.ini)")
	}

	subject, err := jctx.Store.FindRandomSubject(ctx)
	if err != nil {
		return err
	}
	if subject.ID == 0 {
		return errors.New("no available subjects found")
	}

	prompt := fmt.Sprintf(`# INSTRUCTIONS
Write 10 to 15 paragraphs about a single unique random fact of any topic that you can think of about %s,
make the explanation engaging while keeping it simple. You must use the specified output format.

# SAMPLE OUTPUT FORMAT:
TITLE: The title for the subject
CONTENT: The content about the fun fact`, subject.Subject)

	response, err := callGemini(ctx, apiKey, prompt)
	if err != nil {
		return err
	}

	text := geminiExtractText(response)
	if text == "" {
		return errors.New("gemini response text missing")
	}

	text = strings.ReplaceAll(text, "\n\n", "\n")
	text = strings.ReplaceAll(text, "***", "")
	text = strings.ReplaceAll(text, "**", "")

	title := ""
	paragraphs := []map[string]any{}
	count := 0
	prevSpacer := false
	punctuationSpacers := map[rune]int{
		'.': 3,
		'!': 3,
		'?': 3,
		';': 2,
		',': 1,
	}

	lines := strings.Split(text, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "TITLE:") {
			title = strings.TrimSpace(strings.TrimPrefix(line, "TITLE:"))
		} else if line != "" {
			line = strings.TrimSpace(strings.TrimPrefix(line, "CONTENT:"))
			for _, sentence := range splitSentences(line, ".!?;,") {
				sentence = strings.TrimSpace(sentence)
				if sentence == "" {
					continue
				}
				last := rune(sentence[len(sentence)-1])
				spacer := punctuationSpacers[last]
				if spacer == 0 {
					spacer = 2
				}
				count++
				paragraphs = append(paragraphs, map[string]any{
					"count":   count,
					"content": sentence,
				})
				count++
				paragraphs = append(paragraphs, map[string]any{
					"count":   count,
					"content": fmt.Sprintf("<spacer %d>", spacer),
				})
			}
			prevSpacer = false
		}
		if !prevSpacer {
			count++
			paragraphs = append(paragraphs, map[string]any{
				"count":   count,
				"content": "<spacer 3>",
			})
			prevSpacer = true
		}
	}

	metaPayload := map[string]any{
		"status": map[string]any{
			"funfact_created":     true,
			"wav_generated":       false,
			"mp3_generated":       false,
			"podcast_ready":       false,
			"youtube_uploaded":    false,
			"srt_generated":       false,
			"thumbnail_generated": false,
		},
		"sentences":       paragraphs,
		"gemini_response": response,
	}

	metaJSON, err := json.Marshal(metaPayload)
	if err != nil {
		return err
	}
	sentencesJSON, err := json.Marshal(paragraphs)
	if err != nil {
		return err
	}

	status := "funfact_created"
	contentType := "gemini.payload"
	content := db.Content{
		ID:        contentID,
		Title:     title,
		Status:    &status,
		Type:      &contentType,
		Sentences: sentencesJSON,
		Count:     count,
		Meta:      metaJSON,
	}

	if contentID == 0 {
		if _, err := jctx.Store.CreateContent(ctx, content); err != nil {
			return err
		}
	} else {
		if err := jctx.Store.UpsertContentByID(ctx, content); err != nil {
			return err
		}
	}

	return jctx.Store.IncrementSubjectPodcasts(ctx, subject.ID)
}

func runRssSubscribe(ctx context.Context, jctx jobs.JobContext, args []string) error {
	fs := flag.NewFlagSet("Rss:Subscribe", flag.ContinueOnError)
	verbose := fs.Bool("verbose", utils.Verbose, "Verbose logging")
	if err := fs.Parse(args); err != nil {
		return err
	}
	utils.ConfigureLogging(*verbose)

	if len(fs.Args()) == 0 {
		return errors.New("url is required")
	}
	rawURL := fs.Args()[0]
	parsed, err := url.ParseRequestURI(rawURL)
	if err != nil || parsed.Scheme == "" {
		return fmt.Errorf("invalid url: %s", rawURL)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(rawURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("rss fetch failed status=%d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	channel, err := parseRSSChannel(body)
	if err != nil {
		return err
	}

	now := time.Now()
	sub := db.Subscription{
		FeedURL:       rawURL,
		Title:         channel.Title,
		Description:   channel.Description,
		SiteURL:       channel.Link,
		LastFetchedAt: &now,
		IsActive:      true,
	}
	return jctx.Store.InsertSubscription(ctx, sub)
}

func runRssFetchHtml(ctx context.Context, jctx jobs.JobContext, args []string) error {
	fs := flag.NewFlagSet("Rss:FetchHtml", flag.ContinueOnError)
	verbose := fs.Bool("verbose", utils.Verbose, "Verbose logging")
	if err := fs.Parse(args); err != nil {
		return err
	}
	utils.ConfigureLogging(*verbose)

	subscriptions, err := jctx.Store.ListActiveSubscriptions(ctx)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 30 * time.Second}
	for _, sub := range subscriptions {
		resp, err := client.Get(sub.FeedURL)
		if err != nil {
			return err
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return err
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("rss fetch failed status=%d url=%s", resp.StatusCode, sub.FeedURL)
		}

		items, err := parseTrendingItems(body)
		if err != nil {
			return err
		}
		for _, item := range items {
			for _, news := range item.NewsItems {
				if news.URL == "" {
					continue
				}
				existing, err := jctx.Store.GetCollectionByURL(ctx, news.URL)
				if err != nil {
					return err
				}
				if existing.ID != 0 && existing.HTMLContent != "" {
					continue
				}
				html, err := fetchHTML(news.URL)
				if err != nil {
					return err
				}
				if html == "" {
					continue
				}
				if existing.ID != 0 {
					if err := jctx.Store.UpdateCollectionHTML(ctx, existing.ID, html); err != nil {
						return err
					}
					continue
				}
				coll := db.Collection{
					URL:         news.URL,
					Title:       news.Title,
					Language:    "en",
					HTMLContent: html,
					FetchedAt:   time.Now(),
				}
				if err := jctx.Store.InsertCollection(ctx, coll); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func runSubjectProcessCollections(ctx context.Context, jctx jobs.JobContext, args []string) error {
	fs := flag.NewFlagSet("Subject:ProcessCollections", flag.ContinueOnError)
	verbose := fs.Bool("verbose", utils.Verbose, "Verbose logging")
	if err := fs.Parse(args); err != nil {
		return err
	}
	utils.ConfigureLogging(*verbose)

	apiKey := jctx.Config.GeminiAPIKey
	if apiKey == "" {
		return errors.New("missing gemini api key (set gemini.api_key in config.ini)")
	}

	lastID := int64(0)
	for {
		collections, err := jctx.Store.ListCollectionsUnprocessed(ctx, lastID, 100)
		if err != nil {
			return err
		}
		if len(collections) == 0 {
			break
		}
		for _, collection := range collections {
			subjects, err := extractSubjects(ctx, apiKey, collection.HTMLContent)
			if err != nil {
				fmt.Fprintf(os.Stderr, "collection %d error: %v\n", collection.ID, err)
				time.Sleep(30 * time.Second)
				continue
			}
			for _, subject := range subjects {
				subject = strings.TrimSpace(strings.ToLower(subject))
				if subject == "" {
					continue
				}
				existing, err := jctx.Store.GetSubjectByName(ctx, subject)
				if err != nil {
					return err
				}
				if existing.ID == 0 {
					if err := jctx.Store.InsertSubject(ctx, subject); err != nil {
						return err
					}
				}
			}
			if err := jctx.Store.MarkCollectionProcessed(ctx, collection.ID); err != nil {
				return err
			}
			lastID = collection.ID
		}
	}
	return nil
}

func runYoutubeUpdateMeta(ctx context.Context, jctx jobs.JobContext, args []string) error {
	fs := flag.NewFlagSet("Youtube:UpdateMeta", flag.ContinueOnError)
	verbose := fs.Bool("verbose", utils.Verbose, "Verbose logging")
	if err := fs.Parse(args); err != nil {
		return err
	}
	utils.ConfigureLogging(*verbose)

	lastID := int64(0)
	for {
		contents, err := listContentBatch(ctx, jctx.Store, "WHERE meta->>'video_id.v1' IS NOT NULL", nil, lastID, 100)
		if err != nil {
			return err
		}
		if len(contents) == 0 {
			break
		}
		for _, content := range contents {
			if err := updateYoutubeMeta(ctx, jctx, content); err != nil {
				return err
			}
			lastID = content.ID
		}
	}
	return nil
}

func updateYoutubeMeta(ctx context.Context, jctx jobs.JobContext, content db.Content) error {
	meta, err := utils.DecodeMeta(content.Meta)
	if err != nil {
		return err
	}

	if youtubeMeta, ok := meta["youtube"].(map[string]any); ok {
		if raw, ok := youtubeMeta["meta_last_updated_at"].(string); ok && raw != "" {
			if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
				if time.Since(parsed) < 24*time.Hour {
					return nil
				}
			}
		}
	}

	videoID, _ := meta["video_id.v1"].(string)
	if videoID == "" {
		return errors.New("video id missing in meta")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://www.youtube.com/watch?v="+videoID, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("youtube returned status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if len(body) == 0 {
		return errors.New("empty youtube response")
	}

	if meta["youtube"] == nil {
		meta["youtube"] = map[string]any{}
	}
	youtubeMeta, _ := meta["youtube"].(map[string]any)
	youtubeMeta["meta_last_updated_at"] = time.Now().Format(time.RFC3339)
	meta["youtube"] = youtubeMeta

	return jctx.Store.UpdateContentMeta(ctx, content.ID, meta)
}

func runAppFabricExtractWisdom(ctx context.Context, jctx jobs.JobContext, args []string) error {
	fs := flag.NewFlagSet("app:fabric-extract-wisdom", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	fmt.Println("fabric-extract-wisdom: not implemented")
	return nil
}

func runChatHiennaGPT(ctx context.Context, jctx jobs.JobContext, args []string) error {
	fs := flag.NewFlagSet("chat:HiennaGPT", flag.ContinueOnError)
	verbose := fs.Bool("verbose", utils.Verbose, "Verbose logging")
	if err := fs.Parse(args); err != nil {
		return err
	}
	utils.ConfigureLogging(*verbose)

	query := strings.TrimSpace(strings.Join(fs.Args(), " "))
	if query == "" {
		return errors.New("query is required")
	}

	apiKey := jctx.Config.Portnumber53APIKey
	if apiKey == "" {
		return errors.New("missing portnumber53 api key (set portnumber53.api_key in config.ini)")
	}

	payload := map[string]any{
		"model":  "llama3.1:8b",
		"stream": false,
		"prompt": fmt.Sprintf(`SYSTEM """
You are a politician, that answers questions always trying to avoid giving a real, or even correct answer. You also add a lot of generic definitions and circular logic to your speech. Keep your answers around 100 words. Some examples:
-When people ask you about fixing the education system, you may praise the color of the buses and be excited about their color.
-When asked about how you're going to fix the economy, you bring into the conversation social rights that have nothing to do with economics.
-When asked about inflation, you will say prices have gone up, and prices being up, makes things cost more, because inflation is high.
"""
USER """
%s
"""`, query),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://ollama.portnumber53.com/api/generate", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-key", apiKey)

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
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("chat response status %d", resp.StatusCode)
	}

	var decoded struct {
		Response string `json:"response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return err
	}
	fmt.Println(decoded.Response)
	return nil
}

func runSentencesCheck(ctx context.Context, jctx jobs.JobContext, args []string) error {
	fs := flag.NewFlagSet("sentences:check", flag.ContinueOnError)
	verbose := fs.Bool("verbose", utils.Verbose, "Verbose logging")
	if err := fs.Parse(args); err != nil {
		return err
	}
	utils.ConfigureLogging(*verbose)

	contentID, err := parseContentID(fs.Args())
	if err != nil {
		return err
	}

	var contents []db.Content
	if contentID != 0 {
		content, err := jctx.Store.GetContentByID(ctx, contentID)
		if err != nil {
			return err
		}
		contents = append(contents, content)
	} else {
		all, err := jctx.Store.QueryContents(ctx, `
			SELECT id, title, status, type, sentences, count, meta, archive, created_at, updated_at
			FROM contents
			WHERE type = 'gemini.payload'
			ORDER BY id
		`)
		if err != nil {
			return err
		}
		contents = all
	}

	modified := 0
	for _, content := range contents {
		meta, err := utils.DecodeMeta(content.Meta)
		if err != nil {
			return err
		}
		sentences, err := decodeSentences(content.Sentences)
		if err != nil {
			return err
		}
		filtered := make([]map[string]any, 0, len(sentences))
		for _, sentence := range sentences {
			text, _ := sentence["content"].(string)
			if strings.HasPrefix(text, "<spacer") {
				continue
			}
			filtered = append(filtered, sentence)
		}

		filenames := []any{}
		if metaValue, ok := meta["filenames"]; ok {
			if raw, ok := metaValue.([]any); ok {
				filenames = raw
			}
		}

		allMatch := true
		for _, sentence := range filtered {
			sentenceID := metaInt(sentence, "count")
			if !filenamesContainSentence(filenames, sentenceID) {
				fmt.Printf("Record with ID %d sentence %d has no filename.\n", content.ID, sentenceID)
				allMatch = false
				break
			}
		}

		titlePrefix := fmt.Sprintf("%010d-%03d-", content.ID, 0)
		if !filenamesContainPrefix(filenames, titlePrefix) {
			fmt.Printf("Record with ID %d has no title filename.\n", content.ID)
			allMatch = false
		}

		if allMatch && (content.Type == nil || *content.Type != "gemini.wav_ready") {
			if err := jctx.Store.UpdateContentType(ctx, content.ID, "gemini.wav_ready"); err != nil {
				return err
			}
			modified++
		}
	}

	if modified > 0 {
		fmt.Printf("%d record(s) were updated successfully.\n", modified)
	} else {
		fmt.Println("No records were updated.")
	}
	return nil
}

func filenamesContainSentence(filenames []any, sentenceID int) bool {
	for _, item := range filenames {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		rawID, ok := entry["sentence_id"]
		if !ok {
			continue
		}
		switch v := rawID.(type) {
		case float64:
			if int(v) == sentenceID {
				return true
			}
		case int:
			if v == sentenceID {
				return true
			}
		case string:
			if parsed, _ := strconv.Atoi(v); parsed == sentenceID {
				return true
			}
		}
	}
	return false
}

func filenamesContainPrefix(filenames []any, prefix string) bool {
	for _, item := range filenames {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		filename, _ := entry["filename"].(string)
		if strings.Contains(filename, prefix) {
			return true
		}
	}
	return false
}

func runTiktokPublish(ctx context.Context, jctx jobs.JobContext, args []string) error {
	fs := flag.NewFlagSet("tiktok:publish", flag.ContinueOnError)
	accessToken := fs.String("access-token", jctx.Config.TikTokAccessToken, "TikTok access token")
	videoPath := fs.String("file", jctx.Config.TikTokVideoPath, "Video file to upload")
	verbose := fs.Bool("verbose", utils.Verbose, "Verbose logging")
	if err := fs.Parse(args); err != nil {
		return err
	}
	utils.ConfigureLogging(*verbose)

	if *accessToken == "" {
		return errors.New("missing access token (set tiktok.access_token in config.ini or --access-token)")
	}
	if *videoPath == "" {
		return errors.New("missing video file (set tiktok.video_path in config.ini or --file)")
	}
	info, err := os.Stat(*videoPath)
	if err != nil {
		return err
	}

	payload := map[string]any{
		"source_info": map[string]any{
			"source":            *videoPath,
			"video_size":        info.Size(),
			"chunk_size":        info.Size(),
			"total_chunk_count": 1,
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://open.tiktokapis.com/v2/post/publish/inbox/video/init/", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+*accessToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("tiktok publish failed status=%d", resp.StatusCode)
	}
	fmt.Println("Video published successfully.")
	return nil
}

func runTTSSplitJobs(ctx context.Context, jctx jobs.JobContext, args []string) error {
	fs := flag.NewFlagSet("tts:SplitJobs", flag.ContinueOnError)
	verbose := fs.Bool("verbose", utils.Verbose, "Verbose logging")
	if err := fs.Parse(args); err != nil {
		return err
	}
	utils.ConfigureLogging(*verbose)

	contentID, sentenceID, err := parseContentSentenceArgs(fs.Args())
	if err != nil {
		return err
	}
	if contentID == 0 {
		return errors.New("invalid content ID")
	}
	if jctx.Queue == nil {
		return errors.New("queue client is not configured")
	}

	content, err := jctx.Store.GetContentByID(ctx, contentID)
	if err != nil {
		return err
	}

	meta, err := utils.DecodeMeta(content.Meta)
	if err != nil {
		return err
	}

	if sentenceID != nil {
		filenames := []any{}
		if raw, ok := meta["filenames"].([]any); ok {
			filenames = raw
		}
		filtered := make([]any, 0, len(filenames))
		for _, item := range filenames {
			entry, ok := item.(map[string]any)
			if !ok {
				filtered = append(filtered, item)
				continue
			}
			idValue, ok := entry["sentence_id"]
			if !ok || !equalSentenceID(idValue, *sentenceID) {
				filtered = append(filtered, entry)
			}
		}
		meta["filenames"] = filtered
		if err := jctx.Store.UpdateContentMeta(ctx, content.ID, meta); err != nil {
			return err
		}
	}

	if content.Title != "" {
		if err := enqueueTTSJob(jctx, content, content.Title, 0); err != nil {
			return err
		}
	}

	sentences, err := decodeSentences(content.Sentences)
	if err != nil {
		return err
	}
	for idx, sentence := range sentences {
		text, _ := sentence["content"].(string)
		if strings.HasPrefix(text, "<spacer ") {
			continue
		}
		if err := enqueueTTSJob(jctx, content, text, idx); err != nil {
			return err
		}
	}
	return nil
}

func enqueueTTSJob(jctx jobs.JobContext, content db.Content, text string, index int) error {
	payload := map[string]any{
		"text":        text,
		"voice":       "jenny",
		"filename":    ttsFilename(content.ID, text, index),
		"content_id":  content.ID,
		"sentence_id": index,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if err := jctx.Queue.Publish("tts_wave", body); err != nil {
		return err
	}
	fmt.Printf("%d : %s\n", index, text)
	return nil
}

func ttsFilename(contentID int64, text string, index int) string {
	return fmt.Sprintf("%010d-%03d-%s-%s.wav", contentID, index, "jenny", utils.MD5String(text))
}

type rssChannel struct {
	Title       *string `xml:"channel>title"`
	Description *string `xml:"channel>description"`
	Link        *string `xml:"channel>link"`
}

func parseRSSChannel(data []byte) (rssChannel, error) {
	var channel rssChannel
	if err := xml.Unmarshal(data, &channel); err != nil {
		return rssChannel{}, err
	}
	return channel, nil
}

type rssFeed struct {
	Channel *rssChannelItems `xml:"channel"`
	Items   []rssItem        `xml:"item"`
}

type rssChannelItems struct {
	Items []rssItem `xml:"item"`
}

type rssItem struct {
	Title     string    `xml:"title"`
	NewsItems []rssNews `xml:"http://trends.google.com/trending/rss news_item"`
}

type rssNews struct {
	Title string `xml:"http://trends.google.com/trending/rss news_item_title"`
	URL   string `xml:"http://trends.google.com/trending/rss news_item_url"`
}

func parseTrendingItems(data []byte) ([]rssItem, error) {
	var feed rssFeed
	if err := xml.Unmarshal(data, &feed); err != nil {
		return nil, err
	}
	if feed.Channel != nil {
		return feed.Channel.Items, nil
	}
	return feed.Items, nil
}

func fetchHTML(rawURL string) (string, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("fetch %s status=%d", rawURL, resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func callGemini(ctx context.Context, apiKey, prompt string) (map[string]any, error) {
	payload := map[string]any{
		"contents": []any{
			map[string]any{
				"parts": []any{
					map[string]any{
						"text": prompt,
					},
				},
			},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/gemini-1.5-flash:generateContent?key=%s", apiKey), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 600 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("gemini response status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var decoded map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, err
	}
	return decoded, nil
}

func geminiExtractText(response map[string]any) string {
	if response == nil {
		return ""
	}
	if text, ok := utils.GetString(response, "candidates", "0", "content", "parts", "0", "text"); ok {
		return text
	}
	return ""
}

func extractSubjects(ctx context.Context, apiKey, htmlContent string) ([]string, error) {
	prompt := "Given the following HTML content, create a list of subjects, things, events, etc. that are mentioned or implied. Format the response as a simple comma-separated list of subjects in lowercase:\n\n" + htmlContent
	response, err := callGemini(ctx, apiKey, prompt)
	if err != nil {
		return nil, err
	}
	text := geminiExtractText(response)
	if text == "" {
		return nil, errors.New("empty gemini subject response")
	}
	parts := strings.Split(text, ",")
	for i, part := range parts {
		parts[i] = strings.TrimSpace(part)
	}
	return parts, nil
}

func decodeSentences(data []byte) ([]map[string]any, error) {
	if len(data) == 0 {
		return nil, nil
	}
	var out []map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func splitSentences(text, punctuation string) []string {
	if text == "" {
		return nil
	}
	var sentences []string
	runes := []rune(text)
	start := 0
	for i := 0; i < len(runes); i++ {
		if strings.ContainsRune(punctuation, runes[i]) {
			if i+1 < len(runes) && unicode.IsSpace(runes[i+1]) {
				sentence := strings.TrimSpace(string(runes[start : i+1]))
				if sentence != "" {
					sentences = append(sentences, sentence)
				}
				j := i + 1
				for j < len(runes) && unicode.IsSpace(runes[j]) {
					j++
				}
				start = j
				i = j - 1
			}
		}
	}
	if start < len(runes) {
		sentence := strings.TrimSpace(string(runes[start:]))
		if sentence != "" {
			sentences = append(sentences, sentence)
		}
	}
	return sentences
}

func parseOptionalRange(args []string) (*int64, *int64, error) {
	if len(args) == 0 {
		return nil, nil, nil
	}
	if len(args) < 2 {
		return nil, nil, errors.New("start and end must be provided together")
	}
	start, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return nil, nil, err
	}
	end, err := strconv.ParseInt(args[1], 10, 64)
	if err != nil {
		return nil, nil, err
	}
	return &start, &end, nil
}

func parseContentSentenceArgs(args []string) (int64, *int, error) {
	if len(args) == 0 {
		return 0, nil, nil
	}
	contentID, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return 0, nil, err
	}
	if len(args) < 2 {
		return contentID, nil, nil
	}
	sentenceID, err := strconv.Atoi(args[1])
	if err != nil {
		return 0, nil, err
	}
	return contentID, &sentenceID, nil
}

func equalSentenceID(value any, target int) bool {
	switch v := value.(type) {
	case float64:
		return int(v) == target
	case int:
		return v == target
	case string:
		parsed, _ := strconv.Atoi(v)
		return parsed == target
	default:
		return false
	}
}

func listContentBatch(ctx context.Context, store *db.Store, where string, args []any, lastID int64, limit int) ([]db.Content, error) {
	cond := strings.TrimSpace(where)
	queryArgs := append([]any{}, args...)
	if cond == "" {
		cond = "WHERE id > $1 ORDER BY id LIMIT $2"
		queryArgs = append(queryArgs, lastID, limit)
	} else {
		if !strings.HasPrefix(strings.ToUpper(cond), "WHERE") {
			cond = "WHERE " + cond
		}
		cond += fmt.Sprintf(" AND id > $%d", len(queryArgs)+1)
		queryArgs = append(queryArgs, lastID)
		cond += fmt.Sprintf(" ORDER BY id LIMIT $%d", len(queryArgs)+1)
		queryArgs = append(queryArgs, limit)
	}
	query := `
		SELECT id, title, status, type, sentences, count, meta, archive, created_at, updated_at
		FROM contents
		` + cond
	return store.QueryContents(ctx, query, queryArgs...)
}

func extractOriginalText(meta map[string]any) string {
	var payload string
	if text, ok := utils.GetString(meta, "ollama_response", "response"); ok && text != "" {
		payload = text
	}
	if text, ok := utils.GetString(meta, "gemini_response", "candidates", "0", "content", "parts", "0", "text"); ok && text != "" {
		payload = text
	}
	if payload == "" {
		return ""
	}

	payload = strings.ReplaceAll(payload, "\n\n", "\n")
	payload = strings.ReplaceAll(payload, "***", "")
	payload = strings.ReplaceAll(payload, "**", "")

	var original strings.Builder
	lines := strings.Split(payload, "\n")
	prevSpacer := false
	punctuationSpacers := map[rune]int{
		'.': 3,
		'!': 3,
		'?': 3,
		';': 2,
		',': 1,
	}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "TITLE:") {
			continue
		}
		if line != "" {
			line = strings.TrimSpace(strings.TrimPrefix(line, "CONTENT:"))
			original.WriteString(line)
			original.WriteString("\n")
			for _, sentence := range splitSentences(line, ".!?;,") {
				sentence = strings.TrimSpace(sentence)
				if sentence == "" {
					continue
				}
				last := rune(sentence[len(sentence)-1])
				_ = punctuationSpacers[last]
			}
			prevSpacer = false
		}
		if !prevSpacer {
			original.WriteString("\n")
			prevSpacer = true
		}
	}
	return strings.TrimSpace(original.String())
}
