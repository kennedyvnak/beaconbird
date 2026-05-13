package worker

import (
	"context"
	"testing"
	"time"

	"github.com/kennedyvnak/beaconbird/internal/domain"
)

func TestSchedulerComputeNextRetryHonorsMaxDelay(t *testing.T) {
	scheduler := NewScheduler(nil, 30*time.Second, time.Hour, 2*time.Minute, time.Minute)
	now := time.Date(2026, 5, 13, 20, 0, 0, 0, time.UTC)
	scheduler.now = func() time.Time { return now }
	scheduler.jitter = func(time.Duration) time.Duration { return 10 * time.Second }

	got := scheduler.ComputeNextRetry(10)

	if !got.Equal(now.Add(time.Hour)) {
		t.Fatalf("expected retry to be capped at max delay, got %v", got)
	}
}

func TestSchedulerRunRecoveryRecoversStaleJobs(t *testing.T) {
	jobs := &fakeSchedulerJobRepo{}
	scheduler := NewScheduler(jobs, 30*time.Second, time.Hour, 2*time.Minute, 5*time.Millisecond)
	now := time.Date(2026, 5, 13, 20, 0, 0, 0, time.UTC)
	scheduler.now = func() time.Time { return now }

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		scheduler.RunRecovery(ctx)
		close(done)
	}()

	time.Sleep(20 * time.Millisecond)
	cancel()
	<-done

	if jobs.staleBefore == nil {
		t.Fatalf("expected stale recovery to run")
	}
	expected := now.Add(-2 * time.Minute)
	if !jobs.staleBefore.Equal(expected) {
		t.Fatalf("expected stale threshold %v, got %v", expected, jobs.staleBefore)
	}
}

type fakeSchedulerJobRepo struct {
	staleBefore *time.Time
}

func (f *fakeSchedulerJobRepo) RecoverStale(ctx context.Context, staleBefore time.Time) ([]*domain.DeliveryJob, error) {
	f.staleBefore = &staleBefore
	return nil, nil
}
