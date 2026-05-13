package worker

import (
	"context"
	"sync"
	"time"

	"github.com/kennedyvnak/beaconbird/internal/domain"
)

type pollerJobRepo interface {
	LockReady(ctx context.Context, workerID string, limit int32) ([]*domain.DeliveryJob, error)
}

type pollerExecutor interface {
	Execute(ctx context.Context, job domain.DeliveryJob) error
}

type Poller struct {
	jobs     pollerJobRepo
	executor pollerExecutor
	workerID string
	sem      chan struct{}
	interval time.Duration
	wg       sync.WaitGroup
}

func NewPoller(jobs pollerJobRepo, executor pollerExecutor, workerID string, concurrency int, interval time.Duration) *Poller {
	if concurrency < 1 {
		concurrency = 1
	}
	return &Poller{
		jobs:     jobs,
		executor: executor,
		workerID: workerID,
		sem:      make(chan struct{}, concurrency),
		interval: interval,
	}
}

func (p *Poller) Run(ctx context.Context) {
	p.poll(ctx)

	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			p.wg.Wait()
			return
		case <-ticker.C:
			p.poll(ctx)
		}
	}
}

func (p *Poller) poll(ctx context.Context) {
	available := cap(p.sem) - len(p.sem)
	if available < 1 {
		return
	}

	jobs, err := p.jobs.LockReady(ctx, p.workerID, int32(available))
	if err != nil {
		return
	}
	for _, job := range jobs {
		select {
		case <-ctx.Done():
			return
		case p.sem <- struct{}{}:
		}

		p.wg.Add(1)
		go func(job domain.DeliveryJob) {
			defer p.wg.Done()
			defer func() { <-p.sem }()
			_ = p.executor.Execute(ctx, job)
		}(*job)
	}
}
