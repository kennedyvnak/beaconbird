package worker

import (
	"context"
	"time"
)

type heartbeatJobRepo interface {
	UpdateHeartbeat(ctx context.Context, workerID string) error
}

type Heartbeat struct {
	jobs     heartbeatJobRepo
	workerID string
	interval time.Duration
}

func NewHeartbeat(jobs heartbeatJobRepo, workerID string, interval time.Duration) *Heartbeat {
	return &Heartbeat{
		jobs:     jobs,
		workerID: workerID,
		interval: interval,
	}
}

func (h *Heartbeat) Run(ctx context.Context) {
	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = h.jobs.UpdateHeartbeat(ctx, h.workerID)
		}
	}
}
