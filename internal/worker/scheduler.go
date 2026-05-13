package worker

import (
	"context"
	"math"
	"math/rand"
	"time"

	"github.com/kennedyvnak/beaconbird/internal/domain"
)

type schedulerJobRepo interface {
	RecoverStale(ctx context.Context, staleBefore time.Time) ([]*domain.DeliveryJob, error)
}

type Scheduler struct {
	jobs             schedulerJobRepo
	baseDelay        time.Duration
	maxDelay         time.Duration
	staleThreshold   time.Duration
	recoveryInterval time.Duration
	now              func() time.Time
	jitter           func(max time.Duration) time.Duration
	computeNextRetry func(retryCount int32) time.Time
}

func NewScheduler(jobs schedulerJobRepo, baseDelay, maxDelay, staleThreshold, recoveryInterval time.Duration) *Scheduler {
	s := &Scheduler{
		jobs:             jobs,
		baseDelay:        baseDelay,
		maxDelay:         maxDelay,
		staleThreshold:   staleThreshold,
		recoveryInterval: recoveryInterval,
		now: func() time.Time {
			return time.Now().UTC()
		},
		jitter: func(max time.Duration) time.Duration {
			if max <= 0 {
				return 0
			}
			return time.Duration(rand.Int63n(int64(max) + 1))
		},
	}
	s.computeNextRetry = s.defaultComputeNextRetry
	return s
}

func (s *Scheduler) ComputeNextRetry(retryCount int32) time.Time {
	return s.computeNextRetry(retryCount)
}

func (s *Scheduler) defaultComputeNextRetry(retryCount int32) time.Time {
	backoff := float64(s.baseDelay) * math.Pow(2, float64(retryCount))
	delay := time.Duration(backoff) + s.jitter(10*time.Second)
	if delay > s.maxDelay {
		delay = s.maxDelay
	}
	return s.now().Add(delay)
}

func (s *Scheduler) RunRecovery(ctx context.Context) {
	ticker := time.NewTicker(s.recoveryInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if s.jobs == nil {
				continue
			}
			_, _ = s.jobs.RecoverStale(ctx, s.now().Add(-s.staleThreshold))
		}
	}
}
