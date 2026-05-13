package worker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kennedyvnak/beaconbird/internal/domain"
	"github.com/kennedyvnak/beaconbird/internal/provider"
)

func TestExecutorMarksTestJobsSentWithoutProvider(t *testing.T) {
	jobs := &fakeExecutorJobRepo{}
	attempts := &fakeExecutorAttemptRepo{}
	executor := NewExecutor(provider.NewRegistry(), jobs, attempts, &Scheduler{})
	executor.now = func() time.Time { return time.Date(2026, 5, 13, 20, 0, 0, 0, time.UTC) }
	executor.idGenerator = func() string { return "attempt-1" }

	err := executor.Execute(context.Background(), domain.DeliveryJob{
		ID:         "job-1",
		Channel:    domain.ChannelPush,
		Status:     domain.DeliveryJobStatusProcessing,
		RetryCount: 0,
		IsTest:     true,
		Payload:    map[string]any{"title": "hello"},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if jobs.sentID != "job-1" {
		t.Fatalf("expected test job to be marked sent")
	}
	if len(attempts.created) != 1 {
		t.Fatalf("expected attempt to be recorded")
	}
	if attempts.created[0].ProviderName != "test" {
		t.Fatalf("expected test provider name, got %q", attempts.created[0].ProviderName)
	}
}

func TestExecutorMarksUnknownProviderJobsDeadLetter(t *testing.T) {
	jobs := &fakeExecutorJobRepo{}
	attempts := &fakeExecutorAttemptRepo{}
	executor := NewExecutor(provider.NewRegistry(), jobs, attempts, &Scheduler{})
	executor.now = func() time.Time { return time.Date(2026, 5, 13, 20, 0, 0, 0, time.UTC) }
	executor.idGenerator = func() string { return "attempt-1" }

	err := executor.Execute(context.Background(), domain.DeliveryJob{
		ID:         "job-1",
		Channel:    domain.ChannelPush,
		Status:     domain.DeliveryJobStatusProcessing,
		RetryCount: 0,
		MaxRetries: 3,
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if jobs.deadLetterID != "job-1" {
		t.Fatalf("expected job to be dead-lettered")
	}
}

func TestExecutorSchedulesRetryOnProviderFailure(t *testing.T) {
	jobs := &fakeExecutorJobRepo{}
	attempts := &fakeExecutorAttemptRepo{}
	registry := provider.NewRegistry()
	registry.Register(fakeDeliveryProvider{
		channel: domain.ChannelPush,
		name:    "fcm",
		err:     errors.New("send failed"),
	})
	scheduler := &Scheduler{}
	scheduler.computeNextRetry = func(retryCount int32) time.Time {
		return time.Date(2026, 5, 13, 20, 1, 0, 0, time.UTC)
	}
	executor := NewExecutor(registry, jobs, attempts, scheduler)
	executor.now = func() time.Time { return time.Date(2026, 5, 13, 20, 0, 0, 0, time.UTC) }
	executor.idGenerator = func() string { return "attempt-1" }

	err := executor.Execute(context.Background(), domain.DeliveryJob{
		ID:         "job-1",
		Channel:    domain.ChannelPush,
		Status:     domain.DeliveryJobStatusProcessing,
		RetryCount: 0,
		MaxRetries: 3,
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if jobs.retryingID != "job-1" {
		t.Fatalf("expected job to be marked retrying")
	}
	if jobs.retryingAt == nil || !jobs.retryingAt.Equal(time.Date(2026, 5, 13, 20, 1, 0, 0, time.UTC)) {
		t.Fatalf("expected retry time to be set, got %v", jobs.retryingAt)
	}
}

func TestExecutorMarksSentOnProviderSuccess(t *testing.T) {
	jobs := &fakeExecutorJobRepo{}
	attempts := &fakeExecutorAttemptRepo{}
	registry := provider.NewRegistry()
	registry.Register(fakeDeliveryProvider{
		channel: domain.ChannelPush,
		name:    "fcm",
		result:  &provider.ProviderResult{ProviderMessageID: "provider-msg-1"},
	})
	executor := NewExecutor(registry, jobs, attempts, &Scheduler{})
	executor.now = func() time.Time { return time.Date(2026, 5, 13, 20, 0, 0, 0, time.UTC) }
	executor.idGenerator = func() string { return "attempt-1" }

	err := executor.Execute(context.Background(), domain.DeliveryJob{
		ID:         "job-1",
		Channel:    domain.ChannelPush,
		Status:     domain.DeliveryJobStatusProcessing,
		RetryCount: 1,
		MaxRetries: 3,
		Payload:    map[string]any{"title": "hello"},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if jobs.sentID != "job-1" {
		t.Fatalf("expected job to be marked sent")
	}
	if len(attempts.created) != 1 {
		t.Fatalf("expected attempt to be recorded")
	}
	if attempts.created[0].ResponsePayload["provider_message_id"] != "provider-msg-1" {
		t.Fatalf("expected provider response payload to be recorded, got %#v", attempts.created[0].ResponsePayload)
	}
}

func TestExecutorRollsBackSuccessAttemptWhenMarkSentFails(t *testing.T) {
	jobs := &fakeExecutorJobRepo{markSentErr: errors.New("mark sent failed")}
	attempts := &fakeExecutorAttemptRepo{}
	registry := provider.NewRegistry()
	registry.Register(fakeDeliveryProvider{
		channel: domain.ChannelPush,
		name:    "fcm",
		result:  &provider.ProviderResult{ProviderMessageID: "provider-msg-1"},
	})
	executor := NewExecutor(registry, jobs, attempts, &Scheduler{}, newFakeExecutorTxRunner(jobs, attempts))
	executor.now = func() time.Time { return time.Date(2026, 5, 13, 20, 0, 0, 0, time.UTC) }
	executor.idGenerator = func() string { return "attempt-1" }

	err := executor.Execute(context.Background(), domain.DeliveryJob{
		ID:         "job-1",
		Channel:    domain.ChannelPush,
		Status:     domain.DeliveryJobStatusProcessing,
		RetryCount: 1,
		MaxRetries: 3,
		Payload:    map[string]any{"title": "hello"},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if len(attempts.created) != 0 {
		t.Fatalf("expected success attempt to roll back on mark-sent error, got %d attempts", len(attempts.created))
	}
}

func TestExecutorRollsBackFailureAttemptWhenRetryTransitionFails(t *testing.T) {
	jobs := &fakeExecutorJobRepo{markRetryingErr: errors.New("mark retrying failed")}
	attempts := &fakeExecutorAttemptRepo{}
	registry := provider.NewRegistry()
	registry.Register(fakeDeliveryProvider{
		channel: domain.ChannelPush,
		name:    "fcm",
		err:     errors.New("send failed"),
	})
	scheduler := &Scheduler{}
	scheduler.computeNextRetry = func(retryCount int32) time.Time {
		return time.Date(2026, 5, 13, 20, 1, 0, 0, time.UTC)
	}
	executor := NewExecutor(registry, jobs, attempts, scheduler, newFakeExecutorTxRunner(jobs, attempts))
	executor.now = func() time.Time { return time.Date(2026, 5, 13, 20, 0, 0, 0, time.UTC) }
	executor.idGenerator = func() string { return "attempt-1" }

	err := executor.Execute(context.Background(), domain.DeliveryJob{
		ID:         "job-1",
		Channel:    domain.ChannelPush,
		Status:     domain.DeliveryJobStatusProcessing,
		RetryCount: 0,
		MaxRetries: 3,
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if len(attempts.created) != 0 {
		t.Fatalf("expected failure attempt to roll back on retry-transition error, got %d attempts", len(attempts.created))
	}
}

type fakeExecutorJobRepo struct {
	sentID            string
	retryingID        string
	retryingAt        *time.Time
	deadLetterID      string
	lastError         *string
	markSentErr       error
	markRetryingErr   error
	markDeadLetterErr error
}

func (f *fakeExecutorJobRepo) MarkSent(ctx context.Context, id string) (*domain.DeliveryJob, error) {
	state := txStateFromContext(ctx)
	if state != nil {
		state.sentID = id
	} else {
		f.sentID = id
	}
	if f.markSentErr != nil {
		return nil, f.markSentErr
	}
	return &domain.DeliveryJob{ID: id, Status: domain.DeliveryJobStatusSent}, nil
}

func (f *fakeExecutorJobRepo) MarkRetrying(ctx context.Context, id string, nextRetryAt time.Time, lastError *string) (*domain.DeliveryJob, error) {
	state := txStateFromContext(ctx)
	if state != nil {
		state.retryingID = id
		state.retryingAt = &nextRetryAt
		state.lastError = lastError
	} else {
		f.retryingID = id
		f.retryingAt = &nextRetryAt
		f.lastError = lastError
	}
	if f.markRetryingErr != nil {
		return nil, f.markRetryingErr
	}
	return &domain.DeliveryJob{ID: id, Status: domain.DeliveryJobStatusRetrying}, nil
}

func (f *fakeExecutorJobRepo) MarkDeadLetter(ctx context.Context, id string, lastError *string) (*domain.DeliveryJob, error) {
	state := txStateFromContext(ctx)
	if state != nil {
		state.deadLetterID = id
		state.lastError = lastError
	} else {
		f.deadLetterID = id
		f.lastError = lastError
	}
	if f.markDeadLetterErr != nil {
		return nil, f.markDeadLetterErr
	}
	return &domain.DeliveryJob{ID: id, Status: domain.DeliveryJobStatusDeadLetter}, nil
}

type fakeExecutorAttemptRepo struct {
	created []*domain.DeliveryAttempt
}

func (f *fakeExecutorAttemptRepo) Create(ctx context.Context, attempt *domain.DeliveryAttempt) (*domain.DeliveryAttempt, error) {
	cpy := *attempt
	if state := txStateFromContext(ctx); state != nil {
		state.created = append(state.created, &cpy)
	} else {
		f.created = append(f.created, &cpy)
	}
	return &cpy, nil
}

type fakeDeliveryProvider struct {
	channel domain.Channel
	name    string
	result  *provider.ProviderResult
	err     error
}

func (f fakeDeliveryProvider) Channel() domain.Channel { return f.channel }
func (f fakeDeliveryProvider) Name() string            { return f.name }
func (f fakeDeliveryProvider) Send(ctx context.Context, job domain.DeliveryJob) (*provider.ProviderResult, error) {
	return f.result, f.err
}

type fakeExecutorTxRunner struct {
	jobs     *fakeExecutorJobRepo
	attempts *fakeExecutorAttemptRepo
}

type fakeExecutorTxContextKey struct{}

type fakeExecutorTxState struct {
	created      []*domain.DeliveryAttempt
	sentID       string
	retryingID   string
	retryingAt   *time.Time
	deadLetterID string
	lastError    *string
}

func (r *fakeExecutorTxRunner) RunInTx(ctx context.Context, fn func(context.Context) error) error {
	state := &fakeExecutorTxState{}
	txCtx := context.WithValue(ctx, fakeExecutorTxContextKey{}, state)
	if err := fn(txCtx); err != nil {
		return err
	}
	r.attempts.created = append(r.attempts.created, state.created...)
	if state.sentID != "" {
		r.jobs.sentID = state.sentID
	}
	if state.retryingID != "" {
		r.jobs.retryingID = state.retryingID
		r.jobs.retryingAt = state.retryingAt
	}
	if state.deadLetterID != "" {
		r.jobs.deadLetterID = state.deadLetterID
	}
	if state.lastError != nil {
		r.jobs.lastError = state.lastError
	}
	return nil
}

func newFakeExecutorTxRunner(jobs *fakeExecutorJobRepo, attempts *fakeExecutorAttemptRepo) *fakeExecutorTxRunner {
	return &fakeExecutorTxRunner{jobs: jobs, attempts: attempts}
}

func txStateFromContext(ctx context.Context) *fakeExecutorTxState {
	state, _ := ctx.Value(fakeExecutorTxContextKey{}).(*fakeExecutorTxState)
	return state
}
