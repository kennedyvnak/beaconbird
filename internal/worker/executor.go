package worker

import (
	"context"
	"crypto/rand"
	"fmt"
	"time"

	"github.com/kennedyvnak/beaconbird/internal/domain"
	"github.com/kennedyvnak/beaconbird/internal/provider"
)

type executorJobRepo interface {
	MarkSent(ctx context.Context, id string) (*domain.DeliveryJob, error)
	MarkRetrying(ctx context.Context, id string, nextRetryAt time.Time, lastError *string) (*domain.DeliveryJob, error)
	MarkDeadLetter(ctx context.Context, id string, lastError *string) (*domain.DeliveryJob, error)
}

type executorAttemptRepo interface {
	Create(ctx context.Context, attempt *domain.DeliveryAttempt) (*domain.DeliveryAttempt, error)
}

type executorTxRunner interface {
	RunInTx(ctx context.Context, fn func(context.Context) error) error
}

type Executor struct {
	registry    *provider.Registry
	jobs        executorJobRepo
	attempts    executorAttemptRepo
	scheduler   *Scheduler
	txRunner    executorTxRunner
	now         func() time.Time
	idGenerator func() string
}

func NewExecutor(registry *provider.Registry, jobs executorJobRepo, attempts executorAttemptRepo, scheduler *Scheduler, txRunners ...executorTxRunner) *Executor {
	executor := &Executor{
		registry:  registry,
		jobs:      jobs,
		attempts:  attempts,
		scheduler: scheduler,
		now: func() time.Time {
			return time.Now().UTC()
		},
		idGenerator: newID,
	}
	if len(txRunners) > 0 {
		executor.txRunner = txRunners[0]
	}
	return executor
}

func (e *Executor) Execute(ctx context.Context, job domain.DeliveryJob) error {
	if job.IsTest {
		return e.runInTx(ctx, func(txCtx context.Context) error {
			if _, err := e.attempts.Create(txCtx, e.newAttempt(job, "test", domain.DeliveryJobStatusSent, nil, map[string]any{"mode": "test"})); err != nil {
				return fmt.Errorf("executor.Execute create test attempt: %w", err)
			}
			if _, err := e.jobs.MarkSent(txCtx, job.ID); err != nil {
				return fmt.Errorf("executor.Execute mark test job sent: %w", err)
			}
			return nil
		})
	}

	deliveryProvider, err := e.registry.For(job.Channel)
	if err != nil {
		msg := err.Error()
		if txErr := e.runInTx(ctx, func(txCtx context.Context) error {
			if _, attemptErr := e.attempts.Create(txCtx, e.newAttempt(job, "unknown", domain.DeliveryJobStatusFailed, &msg, nil)); attemptErr != nil {
				return fmt.Errorf("executor.Execute create missing-provider attempt: %w", attemptErr)
			}
			if _, markErr := e.jobs.MarkDeadLetter(txCtx, job.ID, &msg); markErr != nil {
				return fmt.Errorf("executor.Execute dead-letter missing-provider job: %w", markErr)
			}
			return nil
		}); txErr != nil {
			return txErr
		}
		return err
	}

	result, sendErr := deliveryProvider.Send(ctx, job)
	if sendErr == nil {
		response := map[string]any{}
		if result != nil && result.ProviderMessageID != "" {
			response["provider_message_id"] = result.ProviderMessageID
		}
		return e.runInTx(ctx, func(txCtx context.Context) error {
			if _, err := e.attempts.Create(txCtx, e.newAttempt(job, deliveryProvider.Name(), domain.DeliveryJobStatusSent, nil, response)); err != nil {
				return fmt.Errorf("executor.Execute create success attempt: %w", err)
			}
			if _, err := e.jobs.MarkSent(txCtx, job.ID); err != nil {
				return fmt.Errorf("executor.Execute mark job sent: %w", err)
			}
			return nil
		})
	}

	msg := sendErr.Error()
	if job.RetryCount+1 >= job.MaxRetries {
		if err := e.runInTx(ctx, func(txCtx context.Context) error {
			if _, err := e.attempts.Create(txCtx, e.newAttempt(job, deliveryProvider.Name(), domain.DeliveryJobStatusFailed, &msg, nil)); err != nil {
				return fmt.Errorf("executor.Execute create failure attempt: %w", err)
			}
			if _, err := e.jobs.MarkDeadLetter(txCtx, job.ID, &msg); err != nil {
				return fmt.Errorf("executor.Execute mark dead letter: %w", err)
			}
			return nil
		}); err != nil {
			return err
		}
		return sendErr
	}

	nextRetryAt := e.scheduler.ComputeNextRetry(job.RetryCount + 1)
	if err := e.runInTx(ctx, func(txCtx context.Context) error {
		if _, err := e.attempts.Create(txCtx, e.newAttempt(job, deliveryProvider.Name(), domain.DeliveryJobStatusFailed, &msg, nil)); err != nil {
			return fmt.Errorf("executor.Execute create failure attempt: %w", err)
		}
		if _, err := e.jobs.MarkRetrying(txCtx, job.ID, nextRetryAt, &msg); err != nil {
			return fmt.Errorf("executor.Execute mark retrying: %w", err)
		}
		return nil
	}); err != nil {
		return err
	}
	return sendErr
}

func (e *Executor) runInTx(ctx context.Context, fn func(context.Context) error) error {
	if e.txRunner == nil {
		return fn(ctx)
	}
	return e.txRunner.RunInTx(ctx, fn)
}

func (e *Executor) newAttempt(job domain.DeliveryJob, providerName string, status domain.DeliveryJobStatus, errorMessage *string, responsePayload map[string]any) *domain.DeliveryAttempt {
	return &domain.DeliveryAttempt{
		ID:              e.idGenerator(),
		DeliveryJobID:   job.ID,
		AttemptNumber:   job.RetryCount + 1,
		ProviderName:    providerName,
		Status:          status,
		RequestPayload:  clonePayload(job.Payload),
		ResponsePayload: clonePayload(responsePayload),
		ErrorMessage:    errorMessage,
		AttemptedAt:     e.now(),
	}
}

func newID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

func clonePayload(src map[string]any) map[string]any {
	if src == nil {
		return nil
	}
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
