package worker

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/kennedyvnak/beaconbird/internal/domain"
)

func TestPollerRunLocksAndExecutesJobsUpToConcurrency(t *testing.T) {
	repo := &fakePollerJobRepo{
		jobs: []*domain.DeliveryJob{
			{ID: "job-1"},
			{ID: "job-2"},
		},
	}
	executor := &fakePollerExecutor{started: make(chan string, 2), release: make(chan struct{})}
	poller := NewPoller(repo, executor, "worker-1", 2, 5*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		poller.Run(ctx)
		close(done)
	}()

	got1 := <-executor.started
	got2 := <-executor.started
	if got1 == got2 {
		t.Fatalf("expected two distinct jobs to start")
	}
	if repo.workerID != "worker-1" {
		t.Fatalf("expected poller to lock jobs with worker id, got %q", repo.workerID)
	}
	if repo.limit != 2 {
		t.Fatalf("expected poller to request up to concurrency limit, got %d", repo.limit)
	}

	close(executor.release)
	cancel()
	<-done
}

func TestPollerDoesNotLockJobsWhenAllSlotsBusy(t *testing.T) {
	repo := &fakePollerJobRepo{
		jobsByCall: [][]*domain.DeliveryJob{
			{{ID: "job-1"}},
			{{ID: "job-2"}},
		},
	}
	executor := &fakePollerExecutor{started: make(chan string, 1), release: make(chan struct{})}
	poller := NewPoller(repo, executor, "worker-1", 1, time.Minute)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	poller.poll(ctx)
	if got := <-executor.started; got != "job-1" {
		t.Fatalf("expected first job to start, got %q", got)
	}

	poller.poll(ctx)

	if repo.calls != 1 {
		t.Fatalf("expected poller not to lock more jobs while full, got %d calls", repo.calls)
	}

	close(executor.release)
	poller.wg.Wait()
}

type fakePollerJobRepo struct {
	jobs       []*domain.DeliveryJob
	jobsByCall [][]*domain.DeliveryJob
	workerID   string
	limit      int32
	once       sync.Once
	calls      int
}

func (f *fakePollerJobRepo) LockReady(ctx context.Context, workerID string, limit int32) ([]*domain.DeliveryJob, error) {
	f.workerID = workerID
	f.limit = limit
	f.calls++

	if len(f.jobsByCall) > 0 {
		idx := f.calls - 1
		if idx < len(f.jobsByCall) {
			return f.jobsByCall[idx], nil
		}
		return nil, nil
	}

	var out []*domain.DeliveryJob
	f.once.Do(func() {
		out = f.jobs
	})
	return out, nil
}

type fakePollerExecutor struct {
	started chan string
	release chan struct{}
}

func (f *fakePollerExecutor) Execute(ctx context.Context, job domain.DeliveryJob) error {
	f.started <- job.ID
	<-f.release
	return nil
}
