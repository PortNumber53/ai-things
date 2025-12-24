package jobs

import (
	"context"
	"encoding/json"
	"errors"

	"ai-things/manager-go/internal/db"
	"ai-things/manager-go/internal/utils"
)

type SetupPodcastJob struct {
	BaseJob
}

func NewSetupPodcastJob() SetupPodcastJob {
	return SetupPodcastJob{
		BaseJob: BaseJob{},
	}
}

func (j SetupPodcastJob) Run(ctx context.Context, jctx JobContext, opts JobOptions) error {
	utils.Info("SetupPodcast process", "content_id", opts.ContentID)
	contentID := opts.ContentID
	var content db.Content
	var err error

	if contentID == 0 {
		where := "WHERE status = 'build.podcast' AND type = 'gemini.payload'"
		content, err = jctx.Store.FindFirstContent(ctx, where)
		if err != nil {
			return err
		}
		if content.ID == 0 {
			return errors.New("content not found")
		}
	} else {
		content, err = jctx.Store.GetContentByID(ctx, contentID)
		if err != nil {
			return err
		}
	}

	payload, _ := json.Marshal(QueuePayload{ContentID: content.ID, Hostname: jctx.Config.Hostname})
	if jctx.Queue != nil {
		return jctx.Queue.Publish("podcast.built", payload)
	}
	return nil
}
