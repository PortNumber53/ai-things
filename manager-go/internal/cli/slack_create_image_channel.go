package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"strings"
	"time"

	"ai-things/manager-go/internal/jobs"
	"ai-things/manager-go/internal/slack"
	"ai-things/manager-go/internal/utils"
)

func runSlackCreateImageChannel(ctx context.Context, jctx jobs.JobContext, args []string) error {
	fs := flag.NewFlagSet("Slack:CreateImageChannel", flag.ContinueOnError)
	name := fs.String("name", "ai-images", "Channel name to create (without #)")
	private := fs.Bool("private", false, "Create a private channel")
	verbose := fs.Bool("verbose", utils.Verbose, "Verbose logging")
	if err := fs.Parse(args); err != nil {
		return err
	}
	utils.ConfigureLogging(*verbose)

	cfg := jctx.Config
	teamID := strings.TrimSpace(cfg.SlackTeamID)
	if teamID == "" {
		var err error
		teamID, err = jctx.Store.GetDefaultSlackTeamID(ctx)
		if err != nil {
			return err
		}
		if teamID == "" {
			return errors.New("no Slack installation found in DB (run Slack:Serve install flow first)")
		}
	}

	token, err := jctx.Store.GetSlackBotToken(ctx, teamID)
	if err != nil || token == "" {
		return fmt.Errorf("missing slack bot token for team_id=%s (install the Slack app first)", teamID)
	}

	chName := strings.TrimSpace(*name)
	chName = strings.TrimPrefix(chName, "#")
	if chName == "" {
		return errors.New("missing --name")
	}

	client := &http.Client{Timeout: 20 * time.Second}
	channelID, err := slack.CreateChannel(ctx, client, token, chName, *private)
	if err != nil {
		return err
	}

	// Join ensures posting works even if workspace defaults change.
	_ = slack.JoinChannel(ctx, client, token, channelID)

	if err := jctx.Store.UpsertSlackImageChannel(ctx, teamID, channelID); err != nil {
		return err
	}

	fmt.Printf("team_id=%s image_channel_id=%s\n", teamID, channelID)
	return nil
}
