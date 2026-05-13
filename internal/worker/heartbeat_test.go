package worker

import (
	"context"
	"testing"
	"time"
)

func TestHeartbeatRunUpdatesWorkerHeartbeat(t *testing.T) {
	jobs := &fakeHeartbeatJobRepo{}
	heartbeat := NewHeartbeat(jobs, "worker-1", 5*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		heartbeat.Run(ctx)
		close(done)
	}()

	time.Sleep(20 * time.Millisecond)
	cancel()
	<-done

	if jobs.workerID != "worker-1" {
		t.Fatalf("expected heartbeat to update worker lock, got %q", jobs.workerID)
	}
	if jobs.calls == 0 {
		t.Fatalf("expected heartbeat updates to run")
	}
}

type fakeHeartbeatJobRepo struct {
	workerID string
	calls    int
}

func (f *fakeHeartbeatJobRepo) UpdateHeartbeat(ctx context.Context, workerID string) error {
	f.workerID = workerID
	f.calls++
	return nil
}
