package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"ai-things/manager-go/internal/config"
	"ai-things/manager-go/internal/db"
	"ai-things/manager-go/internal/queue"
	"ai-things/manager-go/internal/utils"
)

type JobContext struct {
	Config config.Config
	Store  *db.Store
	Queue  *queue.Client
}

type JobOptions struct {
	ContentID  int64
	Sleep      int
	Queue      bool
	Regenerate bool
	Info       bool
	EasyUpload bool
	QueueOnce  bool
}

type BaseJob struct {
	QueueInput      string
	QueueOutput     string
	IgnoreHostCheck bool
}

type QueuePayload struct {
	ContentID int64  `json:"content_id"`
	Hostname  string `json:"hostname"`
}

type QueueHandler func(ctx context.Context, contentID int64, hostname string) error

func (b BaseJob) RunQueue(ctx context.Context, jctx JobContext, opts JobOptions, handler QueueHandler) error {
	if jctx.Queue == nil {
		return fmt.Errorf("queue client is not configured")
	}

	sleep := opts.Sleep
	if sleep <= 0 {
		sleep = 30
	}

	for {
		msg, err := jctx.Queue.Pop(b.QueueInput)
		if err != nil {
			return err
		}
		if msg == nil {
			utils.Debug("queue empty", "queue", b.QueueInput, "sleep_s", sleep)
			time.Sleep(time.Duration(sleep) * time.Second)
			if opts.QueueOnce {
				return nil
			}
			continue
		}

		var payload QueuePayload
		if err := json.Unmarshal(msg.Body, &payload); err != nil {
			utils.Warn("queue payload json decode failed", "queue", b.QueueInput, "err", err)
			_ = msg.Ack()
			continue
		}
		if payload.ContentID == 0 {
			utils.Warn("queue payload invalid (missing content_id)", "queue", b.QueueInput)
			_ = msg.Ack()
			continue
		}

		if !b.IgnoreHostCheck && payload.Hostname != "" && payload.Hostname != jctx.Config.Hostname {
			utils.Warn("queue host mismatch", "queue", b.QueueInput, "message_host", payload.Hostname, "local_host", jctx.Config.Hostname)
			_ = msg.Nack(true)
			time.Sleep(time.Duration(sleep) * time.Second)
			continue
		}

		if err := handler(ctx, payload.ContentID, payload.Hostname); err != nil {
			utils.Error("queue handler error", "queue", b.QueueInput, "content_id", payload.ContentID, "err", err)
			_ = msg.Nack(true)
			continue
		}
		_ = msg.Ack()
	}
}
