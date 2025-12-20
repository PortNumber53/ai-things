package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"ai-things/manager-go/internal/config"
	"ai-things/manager-go/internal/db"
	"ai-things/manager-go/internal/jobs"
	"ai-things/manager-go/internal/queue"
	"ai-things/manager-go/internal/utils"
)

func Run(args []string) int {
	// Support a global --verbose flag anywhere in the argv (before or after the command).
	// This is helpful because the stdlib flag parser stops at the first non-flag argument.
	args, globalVerbose := extractGlobalVerbose(args)
	if globalVerbose {
		utils.Verbose = true
	}

	if len(args) < 2 {
		printUsage()
		return 1
	}
	if args[1] == "-h" || args[1] == "--help" || args[1] == "help" {
		printUsage()
		return 0
	}

	ctx := context.Background()
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		return 1
	}
	utils.Logf("manager: config loaded env=%s hostname=%s", cfg.AppEnv, cfg.Hostname)

	store, err := db.NewStore(ctx, cfg.DBConnString())
	if err != nil {
		fmt.Fprintf(os.Stderr, "db error: %v\n", err)
		return 1
	}
	defer store.Close()
	utils.Logf("manager: db connected")

	queueClient, err := queue.New(cfg.RabbitMQURL())
	if err != nil {
		fmt.Fprintf(os.Stderr, "queue error: %v\n", err)
		return 1
	}
	defer queueClient.Close()
	utils.Logf("manager: queue connected")

	jctx := jobs.JobContext{
		Config: cfg,
		Store:  store,
		Queue:  queueClient,
	}

	cmd := args[1]
	cmdArgs := args[2:]
	utils.Logf("manager: cmd=%s args=%v", cmd, cmdArgs)

	var runErr error
	switch cmd {
	case "Ai:GenerateFunFacts":
		runErr = runAiGenerateFunFacts(ctx, jctx, cmdArgs)
	case "Ai:SplitText":
		runErr = runAiSplitText(ctx, jctx, cmdArgs)
	case "Backfill:ResponseDataToSentences":
		runErr = runBackfillResponseDataToSentences(ctx, jctx, cmdArgs)
	case "Check:ImageIsGenerated":
		runErr = runCheckImageIsGenerated(ctx, jctx, cmdArgs)
	case "Check:Mp3IsGenerated":
		runErr = runCheckMp3IsGenerated(ctx, jctx, cmdArgs)
	case "Check:PodcastIsGenerated":
		runErr = runCheckPodcastIsGenerated(ctx, jctx, cmdArgs)
	case "Check:SrtIsGenerated":
		runErr = runCheckSrtIsGenerated(ctx, jctx, cmdArgs)
	case "Check:WavIsGenerated":
		runErr = runCheckWavIsGenerated(ctx, jctx, cmdArgs)
	case "Content:FindDuplicateTitles":
		runErr = runContentFindDuplicateTitles(ctx, jctx, cmdArgs)
	case "Content:IdentifySubject":
		runErr = runContentIdentifySubject(ctx, jctx, cmdArgs)
	case "content:query":
		runErr = runContentQuery(ctx, jctx, cmdArgs)
	case "Gemini:GenerateFunFact":
		runErr = runGeminiGenerateFunFact(ctx, jctx, cmdArgs)
	case "job:GenerateWav":
		runErr = runGenerateWav(ctx, jctx, cmdArgs)
	case "job:GenerateSrt":
		runErr = runGenerateSrt(ctx, jctx, cmdArgs)
	case "job:GenerateMp3":
		runErr = runGenerateMp3(ctx, jctx, cmdArgs)
	case "job:PromptForImage":
		runErr = runPromptForImage(ctx, jctx, cmdArgs)
	case "job:GenerateImage":
		runErr = runGenerateImage(ctx, jctx, cmdArgs)
	case "job:GeneratePodcast":
		runErr = runGeneratePodcast(ctx, jctx, cmdArgs)
	case "job:FixSubtitles":
		runErr = runFixSubtitles(ctx, jctx, cmdArgs)
	case "job:CorrectSubtitles":
		runErr = runCorrectSubtitles(ctx, jctx, cmdArgs)
	case "job:SetupPodcast":
		runErr = runSetupPodcast(ctx, jctx, cmdArgs)
	case "job:UploadPodcastToTikTok":
		runErr = runUploadTikTok(ctx, jctx, cmdArgs)
	case "job:UploadPodcastToYoutube":
		runErr = runUploadYouTube(ctx, jctx, cmdArgs)
	case "Rss:FetchHtml":
		runErr = runRssFetchHtml(ctx, jctx, cmdArgs)
	case "Rss:Subscribe":
		runErr = runRssSubscribe(ctx, jctx, cmdArgs)
	case "Subject:ProcessCollections":
		runErr = runSubjectProcessCollections(ctx, jctx, cmdArgs)
	case "Youtube:UpdateMeta":
		runErr = runYoutubeUpdateMeta(ctx, jctx, cmdArgs)
	case "app:fabric-extract-wisdom":
		runErr = runAppFabricExtractWisdom(ctx, jctx, cmdArgs)
	case "chat:HiennaGPT":
		runErr = runChatHiennaGPT(ctx, jctx, cmdArgs)
	case "sentences:check":
		runErr = runSentencesCheck(ctx, jctx, cmdArgs)
	case "tiktok:publish":
		runErr = runTiktokPublish(ctx, jctx, cmdArgs)
	case "tts:SplitJobs":
		runErr = runTTSSplitJobs(ctx, jctx, cmdArgs)
	case "Slack:Serve":
		runErr = runSlackServe(ctx, jctx, cmdArgs)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		printUsage()
		return 1
	}

	if runErr != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", runErr)
		return 1
	}

	return 0
}

func extractGlobalVerbose(args []string) ([]string, bool) {
	if len(args) == 0 {
		return args, false
	}
	verbose := false
	out := make([]string, 0, len(args))
	for _, arg := range args {
		switch {
		case arg == "--verbose" || arg == "-verbose":
			verbose = true
			continue
		case strings.HasPrefix(arg, "--verbose="):
			raw := strings.TrimPrefix(arg, "--verbose=")
			if parsed, err := strconv.ParseBool(raw); err == nil {
				verbose = parsed
			}
			continue
		case strings.HasPrefix(arg, "-verbose="):
			raw := strings.TrimPrefix(arg, "-verbose=")
			if parsed, err := strconv.ParseBool(raw); err == nil {
				verbose = parsed
			}
			continue
		default:
			out = append(out, arg)
		}
	}
	return out, verbose
}

func parseContentID(args []string) (int64, error) {
	if len(args) == 0 {
		return 0, nil
	}
	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid content_id: %s", args[0])
	}
	return id, nil
}

func runGenerateWav(ctx context.Context, jctx jobs.JobContext, args []string) error {
	fs := flag.NewFlagSet("job:GenerateWav", flag.ContinueOnError)
	sleep := fs.Int("sleep", 30, "Sleep time in seconds")
	queueFlag := fs.Bool("queue", false, "Process queue messages")
	verbose := fs.Bool("verbose", utils.Verbose, "Verbose logging")
	if err := fs.Parse(args); err != nil {
		return err
	}
	utils.Verbose = *verbose
	contentID, err := parseContentID(fs.Args())
	if err != nil {
		return err
	}
	opts := jobs.JobOptions{ContentID: contentID, Sleep: *sleep, Queue: *queueFlag}
	logJobStart("job:GenerateWav", opts)

	job := jobs.NewGenerateWavJob()
	return job.Run(ctx, jctx, opts)
}

func runGenerateSrt(ctx context.Context, jctx jobs.JobContext, args []string) error {
	fs := flag.NewFlagSet("job:GenerateSrt", flag.ContinueOnError)
	sleep := fs.Int("sleep", 30, "Sleep time in seconds")
	queueFlag := fs.Bool("queue", false, "Process queue messages")
	verbose := fs.Bool("verbose", utils.Verbose, "Verbose logging")
	if err := fs.Parse(args); err != nil {
		return err
	}
	utils.Verbose = *verbose
	contentID, err := parseContentID(fs.Args())
	if err != nil {
		return err
	}
	opts := jobs.JobOptions{ContentID: contentID, Sleep: *sleep, Queue: *queueFlag}
	logJobStart("job:GenerateSrt", opts)

	job := jobs.NewGenerateSrtJob()
	return job.Run(ctx, jctx, opts)
}

func runGenerateMp3(ctx context.Context, jctx jobs.JobContext, args []string) error {
	fs := flag.NewFlagSet("job:GenerateMp3", flag.ContinueOnError)
	sleep := fs.Int("sleep", 30, "Sleep time in seconds")
	queueFlag := fs.Bool("queue", false, "Process queue messages")
	verbose := fs.Bool("verbose", utils.Verbose, "Verbose logging")
	if err := fs.Parse(args); err != nil {
		return err
	}
	utils.Verbose = *verbose
	contentID, err := parseContentID(fs.Args())
	if err != nil {
		return err
	}
	opts := jobs.JobOptions{ContentID: contentID, Sleep: *sleep, Queue: *queueFlag}
	logJobStart("job:GenerateMp3", opts)

	job := jobs.NewGenerateMp3Job()
	return job.Run(ctx, jctx, opts)
}

func runPromptForImage(ctx context.Context, jctx jobs.JobContext, args []string) error {
	fs := flag.NewFlagSet("job:PromptForImage", flag.ContinueOnError)
	sleep := fs.Int("sleep", 30, "Sleep time in seconds")
	queueFlag := fs.Bool("queue", false, "Process queue messages")
	regenerate := fs.Bool("regenerate", false, "Regenerate the image")
	verbose := fs.Bool("verbose", utils.Verbose, "Verbose logging")
	if err := fs.Parse(args); err != nil {
		return err
	}
	utils.Verbose = *verbose
	contentID, err := parseContentID(fs.Args())
	if err != nil {
		return err
	}
	opts := jobs.JobOptions{ContentID: contentID, Sleep: *sleep, Queue: *queueFlag, Regenerate: *regenerate}
	logJobStart("job:PromptForImage", opts)

	job := jobs.NewPromptForImageJob()
	return job.Run(ctx, jctx, opts)
}

func runGenerateImage(ctx context.Context, jctx jobs.JobContext, args []string) error {
	fs := flag.NewFlagSet("job:GenerateImage", flag.ContinueOnError)
	sleep := fs.Int("sleep", 30, "Sleep time in seconds")
	queueFlag := fs.Bool("queue", false, "Process queue messages")
	verbose := fs.Bool("verbose", utils.Verbose, "Verbose logging")
	if err := fs.Parse(args); err != nil {
		return err
	}
	utils.Verbose = *verbose
	contentID, err := parseContentID(fs.Args())
	if err != nil {
		return err
	}
	opts := jobs.JobOptions{ContentID: contentID, Sleep: *sleep, Queue: *queueFlag}
	logJobStart("job:GenerateImage", opts)

	job := jobs.NewGenerateImageJob()
	return job.Run(ctx, jctx, opts)
}

func runGeneratePodcast(ctx context.Context, jctx jobs.JobContext, args []string) error {
	fs := flag.NewFlagSet("job:GeneratePodcast", flag.ContinueOnError)
	sleep := fs.Int("sleep", 30, "Sleep time in seconds")
	queueFlag := fs.Bool("queue", false, "Process queue messages")
	verbose := fs.Bool("verbose", utils.Verbose, "Verbose logging")
	if err := fs.Parse(args); err != nil {
		return err
	}
	utils.Verbose = *verbose
	contentID, err := parseContentID(fs.Args())
	if err != nil {
		return err
	}
	opts := jobs.JobOptions{ContentID: contentID, Sleep: *sleep, Queue: *queueFlag}
	logJobStart("job:GeneratePodcast", opts)

	job := jobs.NewGeneratePodcastJob()
	return job.Run(ctx, jctx, opts)
}

func runFixSubtitles(ctx context.Context, jctx jobs.JobContext, args []string) error {
	fs := flag.NewFlagSet("job:FixSubtitles", flag.ContinueOnError)
	sleep := fs.Int("sleep", 30, "Sleep time in seconds")
	queueFlag := fs.Bool("queue", false, "Process queue messages")
	verbose := fs.Bool("verbose", utils.Verbose, "Verbose logging")
	if err := fs.Parse(args); err != nil {
		return err
	}
	utils.Verbose = *verbose
	contentID, err := parseContentID(fs.Args())
	if err != nil {
		return err
	}
	opts := jobs.JobOptions{ContentID: contentID, Sleep: *sleep, Queue: *queueFlag}
	logJobStart("job:FixSubtitles", opts)

	job := jobs.NewFixSubtitlesJob()
	return job.Run(ctx, jctx, opts)
}

func runCorrectSubtitles(ctx context.Context, jctx jobs.JobContext, args []string) error {
	fs := flag.NewFlagSet("job:CorrectSubtitles", flag.ContinueOnError)
	sleep := fs.Int("sleep", 30, "Sleep time in seconds")
	queueFlag := fs.Bool("queue", false, "Process queue messages")
	verbose := fs.Bool("verbose", utils.Verbose, "Verbose logging")
	if err := fs.Parse(args); err != nil {
		return err
	}
	utils.Verbose = *verbose
	contentID, err := parseContentID(fs.Args())
	if err != nil {
		return err
	}
	opts := jobs.JobOptions{ContentID: contentID, Sleep: *sleep, Queue: *queueFlag}
	logJobStart("job:CorrectSubtitles", opts)

	job := jobs.NewCorrectSubtitlesJob()
	return job.Run(ctx, jctx, opts)
}

func runSetupPodcast(ctx context.Context, jctx jobs.JobContext, args []string) error {
	fs := flag.NewFlagSet("job:SetupPodcast", flag.ContinueOnError)
	verbose := fs.Bool("verbose", utils.Verbose, "Verbose logging")
	if err := fs.Parse(args); err != nil {
		return err
	}
	utils.Verbose = *verbose
	contentID, err := parseContentID(fs.Args())
	if err != nil {
		return err
	}
	opts := jobs.JobOptions{ContentID: contentID}
	logJobStart("job:SetupPodcast", opts)

	job := jobs.NewSetupPodcastJob()
	return job.Run(ctx, jctx, opts)
}

func runUploadTikTok(ctx context.Context, jctx jobs.JobContext, args []string) error {
	fs := flag.NewFlagSet("job:UploadPodcastToTikTok", flag.ContinueOnError)
	sleep := fs.Int("sleep", 30, "Sleep time in seconds")
	queueFlag := fs.Bool("queue", false, "Process queue messages")
	info := fs.Bool("info", false, "Just show info, do not upload")
	verbose := fs.Bool("verbose", utils.Verbose, "Verbose logging")
	if err := fs.Parse(args); err != nil {
		return err
	}
	utils.Verbose = *verbose
	contentID, err := parseContentID(fs.Args())
	if err != nil {
		return err
	}
	opts := jobs.JobOptions{ContentID: contentID, Sleep: *sleep, Queue: *queueFlag, Info: *info}
	logJobStart("job:UploadPodcastToTikTok", opts)

	job := jobs.NewUploadTikTokJob()
	return job.Run(ctx, jctx, opts)
}

func runUploadYouTube(ctx context.Context, jctx jobs.JobContext, args []string) error {
	fs := flag.NewFlagSet("job:UploadPodcastToYoutube", flag.ContinueOnError)
	sleep := fs.Int("sleep", 30, "Sleep time in seconds")
	queueFlag := fs.Bool("queue", false, "Process queue messages")
	info := fs.Bool("info", false, "Just show info, do not upload")
	easyUpload := fs.Bool("easy-upload", false, "Upload with default settings")
	verbose := fs.Bool("verbose", utils.Verbose, "Verbose logging")
	if err := fs.Parse(args); err != nil {
		return err
	}
	utils.Verbose = *verbose
	contentID, err := parseContentID(fs.Args())
	if err != nil {
		return err
	}
	opts := jobs.JobOptions{ContentID: contentID, Sleep: *sleep, Queue: *queueFlag, Info: *info, EasyUpload: *easyUpload}
	logJobStart("job:UploadPodcastToYoutube", opts)

	job := jobs.NewUploadYouTubeJob()
	return job.Run(ctx, jctx, opts)
}

func logJobStart(name string, opts jobs.JobOptions) {
	utils.Logf("start %s content_id=%d queue=%t sleep=%d regenerate=%t info=%t easy_upload=%t", name, opts.ContentID, opts.Queue, opts.Sleep, opts.Regenerate, opts.Info, opts.EasyUpload)
}

func printUsage() {
	fmt.Println("Usage: manager <command> [args]")
	fmt.Println("Global flags:")
	fmt.Println("  --verbose   Enable diagnostic logging (can appear before or after the command).")
	fmt.Println("Commands:")
	fmt.Println("  Ai:GenerateFunFacts [content_id] [--sleep=N] [--queue] [--verbose]")
	fmt.Println("  Ai:SplitText [--verbose]")
	fmt.Println("  Backfill:ResponseDataToSentences [content_id] [--log] [--verbose]")
	fmt.Println("  Check:ImageIsGenerated [--verbose]")
	fmt.Println("  Check:Mp3IsGenerated [--verbose]")
	fmt.Println("  Check:PodcastIsGenerated [--verbose]")
	fmt.Println("  Check:SrtIsGenerated [--verbose]")
	fmt.Println("  Check:WavIsGenerated [--verbose]")
	fmt.Println("  Content:FindDuplicateTitles [--verbose]")
	fmt.Println("  Content:IdentifySubject --content-id=N [--verbose]")
	fmt.Println("  content:query [start] [end] [--verbose]")
	fmt.Println("  Gemini:GenerateFunFact [content_id] [--verbose]")
	fmt.Println("  job:GenerateWav [content_id] [--sleep=N] [--queue] [--verbose]")
	fmt.Println("  job:GenerateSrt [content_id] [--sleep=N] [--queue] [--verbose]")
	fmt.Println("  job:GenerateMp3 [content_id] [--sleep=N] [--queue] [--verbose]")
	fmt.Println("  job:PromptForImage [content_id] [--sleep=N] [--queue] [--regenerate] [--verbose]")
	fmt.Println("  job:GenerateImage [content_id] [--sleep=N] [--queue] [--verbose]")
	fmt.Println("  job:GeneratePodcast [content_id] [--sleep=N] [--queue] [--verbose]")
	fmt.Println("  job:FixSubtitles [content_id] [--sleep=N] [--queue] [--verbose]")
	fmt.Println("  job:CorrectSubtitles [content_id] [--sleep=N] [--queue] [--verbose]")
	fmt.Println("  job:SetupPodcast [content_id] [--verbose]")
	fmt.Println("  job:UploadPodcastToTikTok [content_id] [--sleep=N] [--queue] [--info] [--verbose]")
	fmt.Println("  job:UploadPodcastToYoutube [content_id] [--sleep=N] [--queue] [--info] [--easy-upload] [--verbose]")
	fmt.Println("  Rss:FetchHtml [--verbose]")
	fmt.Println("  Rss:Subscribe <url> [--verbose]")
	fmt.Println("  Subject:ProcessCollections [--verbose]")
	fmt.Println("  Youtube:UpdateMeta [--verbose]")
	fmt.Println("  app:fabric-extract-wisdom")
	fmt.Println("  chat:HiennaGPT <query> [--verbose]")
	fmt.Println("  sentences:check [id] [--verbose]")
	fmt.Println("  tiktok:publish [--access-token=...] [--file=...] [--verbose]")
	fmt.Println("  tts:SplitJobs <content_id> [sentence_id] [--verbose]")
	fmt.Println("  Slack:Serve [--listen=:8085] [--public-url=https://example.com] [--verbose]")
}
