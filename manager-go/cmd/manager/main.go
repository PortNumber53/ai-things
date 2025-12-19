package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"

	"ai-things/manager-go/internal/config"
	"ai-things/manager-go/internal/db"
	"ai-things/manager-go/internal/jobs"
	"ai-things/manager-go/internal/queue"
	"ai-things/manager-go/internal/utils"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	ctx := context.Background()
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}

	store, err := db.NewStore(ctx, cfg.DBConnString())
	if err != nil {
		fmt.Fprintf(os.Stderr, "db error: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	queueClient, err := queue.New(cfg.RabbitMQURL())
	if err != nil {
		fmt.Fprintf(os.Stderr, "queue error: %v\n", err)
		os.Exit(1)
	}
	defer queueClient.Close()

	jctx := jobs.JobContext{
		Config: cfg,
		Store:  store,
		Queue:  queueClient,
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	var runErr error
	switch cmd {
	case "job:GenerateWav":
		runErr = runGenerateWav(ctx, jctx, args)
	case "job:GenerateSrt":
		runErr = runGenerateSrt(ctx, jctx, args)
	case "job:GenerateMp3":
		runErr = runGenerateMp3(ctx, jctx, args)
	case "job:PromptForImage":
		runErr = runPromptForImage(ctx, jctx, args)
	case "job:GenerateImage":
		runErr = runGenerateImage(ctx, jctx, args)
	case "job:GeneratePodcast":
		runErr = runGeneratePodcast(ctx, jctx, args)
	case "job:FixSubtitles":
		runErr = runFixSubtitles(ctx, jctx, args)
	case "job:CorrectSubtitles":
		runErr = runCorrectSubtitles(ctx, jctx, args)
	case "job:SetupPodcast":
		runErr = runSetupPodcast(ctx, jctx, args)
	case "job:UploadPodcastToTikTok":
		runErr = runUploadTikTok(ctx, jctx, args)
	case "job:UploadPodcastToYoutube":
		runErr = runUploadYouTube(ctx, jctx, args)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}

	if runErr != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", runErr)
		os.Exit(1)
	}
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
	verbose := fs.Bool("verbose", false, "Verbose logging")
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
	verbose := fs.Bool("verbose", false, "Verbose logging")
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
	verbose := fs.Bool("verbose", false, "Verbose logging")
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
	verbose := fs.Bool("verbose", false, "Verbose logging")
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
	verbose := fs.Bool("verbose", false, "Verbose logging")
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
	verbose := fs.Bool("verbose", false, "Verbose logging")
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
	verbose := fs.Bool("verbose", false, "Verbose logging")
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
	verbose := fs.Bool("verbose", false, "Verbose logging")
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
	verbose := fs.Bool("verbose", false, "Verbose logging")
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
	verbose := fs.Bool("verbose", false, "Verbose logging")
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
	verbose := fs.Bool("verbose", false, "Verbose logging")
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
	fmt.Println("Commands:")
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
}
