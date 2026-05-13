package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/kennedyvnak/beaconbird/internal/domain"
	"github.com/kennedyvnak/beaconbird/internal/repository/sqlc"
)

type DeliveryJobRepo struct {
	q *sqlc.Queries
}

func NewDeliveryJobRepo(q *sqlc.Queries) *DeliveryJobRepo {
	return &DeliveryJobRepo{q: q}
}

func (r *DeliveryJobRepo) Create(ctx context.Context, j *domain.DeliveryJob) (*domain.DeliveryJob, error) {
	row, err := r.q.CreateDeliveryJob(ctx, sqlc.CreateDeliveryJobParams{
		ID:             j.ID,
		NotificationID: j.NotificationID,
		ProviderID:     toPgText(j.ProviderID),
		Channel:        string(j.Channel),
		Status:         string(j.Status),
		Payload:        marshalJSON(j.Payload),
		SendAt:         toPgTime(j.SendAt),
		MaxRetries:     j.MaxRetries,
	})
	if err != nil {
		return nil, fmt.Errorf("deliveryJobRepo.Create: %w", err)
	}
	return toDeliveryJob(row), nil
}

func (r *DeliveryJobRepo) GetByID(ctx context.Context, id string) (*domain.DeliveryJob, error) {
	row, err := r.q.GetDeliveryJob(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("deliveryJobRepo.GetByID: %w", err)
	}
	return toDeliveryJob(row), nil
}

type ListDeliveryJobsParams struct {
	Status  string
	Channel string
	From    time.Time
	To      time.Time
	Offset  int32
	Limit   int32
}

func (r *DeliveryJobRepo) List(ctx context.Context, p ListDeliveryJobsParams) ([]*domain.DeliveryJob, error) {
	rows, err := r.q.ListDeliveryJobs(ctx, sqlc.ListDeliveryJobsParams{
		Status:      p.Status,
		Channel:     p.Channel,
		FromTime:    toPgTime(p.From),
		ToTime:      toPgTime(p.To),
		OffsetCount: p.Offset,
		LimitCount:  p.Limit,
	})
	if err != nil {
		return nil, fmt.Errorf("deliveryJobRepo.List: %w", err)
	}
	result := make([]*domain.DeliveryJob, len(rows))
	for i, row := range rows {
		result[i] = toDeliveryJob(row)
	}
	return result, nil
}

func (r *DeliveryJobRepo) ListReady(ctx context.Context, readyAt time.Time, limit int32) ([]*domain.DeliveryJob, error) {
	rows, err := r.q.ListReadyDeliveryJobs(ctx, sqlc.ListReadyDeliveryJobsParams{
		ReadyAt:    toPgTime(readyAt),
		LimitCount: limit,
	})
	if err != nil {
		return nil, fmt.Errorf("deliveryJobRepo.ListReady: %w", err)
	}
	result := make([]*domain.DeliveryJob, len(rows))
	for i, row := range rows {
		result[i] = toDeliveryJob(row)
	}
	return result, nil
}

func (r *DeliveryJobRepo) MarkProcessing(ctx context.Context, id, workerID string) (*domain.DeliveryJob, error) {
	now := time.Now().UTC()
	row, err := r.q.MarkDeliveryJobProcessing(ctx, sqlc.MarkDeliveryJobProcessingParams{
		ID:          id,
		LockedAt:    toPgTime(now),
		LockedBy:    toPgText(&workerID),
		HeartbeatAt: toPgTime(now),
		UpdatedAt:   toPgTime(now),
	})
	if err != nil {
		return nil, fmt.Errorf("deliveryJobRepo.MarkProcessing: %w", err)
	}
	return toDeliveryJob(row), nil
}

func (r *DeliveryJobRepo) MarkSent(ctx context.Context, id string) (*domain.DeliveryJob, error) {
	row, err := r.q.MarkDeliveryJobSent(ctx, sqlc.MarkDeliveryJobSentParams{
		ID:        id,
		UpdatedAt: toPgTime(time.Now().UTC()),
	})
	if err != nil {
		return nil, fmt.Errorf("deliveryJobRepo.MarkSent: %w", err)
	}
	return toDeliveryJob(row), nil
}

func (r *DeliveryJobRepo) MarkRetry(ctx context.Context, id string, nextRetryAt time.Time, lastError *string) (*domain.DeliveryJob, error) {
	row, err := r.q.MarkDeliveryJobForRetry(ctx, sqlc.MarkDeliveryJobForRetryParams{
		ID:          id,
		NextRetryAt: toPgTime(nextRetryAt),
		LastError:   toPgText(lastError),
		UpdatedAt:   toPgTime(time.Now().UTC()),
	})
	if err != nil {
		return nil, fmt.Errorf("deliveryJobRepo.MarkRetry: %w", err)
	}
	return toDeliveryJob(row), nil
}

func (r *DeliveryJobRepo) MarkDeadLetter(ctx context.Context, id string, lastError *string) (*domain.DeliveryJob, error) {
	row, err := r.q.MarkDeliveryJobDeadLetter(ctx, sqlc.MarkDeliveryJobDeadLetterParams{
		ID:        id,
		LastError: toPgText(lastError),
		UpdatedAt: toPgTime(time.Now().UTC()),
	})
	if err != nil {
		return nil, fmt.Errorf("deliveryJobRepo.MarkDeadLetter: %w", err)
	}
	return toDeliveryJob(row), nil
}

func (r *DeliveryJobRepo) CancelForNotification(ctx context.Context, notificationID string) ([]*domain.DeliveryJob, error) {
	rows, err := r.q.CancelDeliveryJobsForNotification(ctx, sqlc.CancelDeliveryJobsForNotificationParams{
		NotificationID: notificationID,
		UpdatedAt:      toPgTime(time.Now().UTC()),
	})
	if err != nil {
		return nil, fmt.Errorf("deliveryJobRepo.CancelForNotification: %w", err)
	}
	result := make([]*domain.DeliveryJob, len(rows))
	for i, row := range rows {
		result[i] = toDeliveryJob(row)
	}
	return result, nil
}

func (r *DeliveryJobRepo) ResetStale(ctx context.Context, staleBefore time.Time) ([]*domain.DeliveryJob, error) {
	rows, err := r.q.ResetStaleDeliveryJobs(ctx, sqlc.ResetStaleDeliveryJobsParams{
		StaleBefore: toPgTime(staleBefore),
		UpdatedAt:   toPgTime(time.Now().UTC()),
	})
	if err != nil {
		return nil, fmt.Errorf("deliveryJobRepo.ResetStale: %w", err)
	}
	result := make([]*domain.DeliveryJob, len(rows))
	for i, row := range rows {
		result[i] = toDeliveryJob(row)
	}
	return result, nil
}

func (r *DeliveryJobRepo) UpdateHeartbeat(ctx context.Context, workerID string) error {
	now := time.Now().UTC()
	_, err := r.q.UpdateHeartbeatForWorker(ctx, sqlc.UpdateHeartbeatForWorkerParams{
		LockedBy:    toPgText(&workerID),
		HeartbeatAt: toPgTime(now),
		UpdatedAt:   toPgTime(now),
	})
	if err != nil {
		return fmt.Errorf("deliveryJobRepo.UpdateHeartbeat: %w", err)
	}
	return nil
}

func toDeliveryJob(row sqlc.DeliveryJob) *domain.DeliveryJob {
	return &domain.DeliveryJob{
		ID:             row.ID,
		NotificationID: row.NotificationID,
		ProviderID:     toStrPtr(row.ProviderID),
		Channel:        domain.Channel(row.Channel),
		Status:         domain.DeliveryJobStatus(row.Status),
		Payload:        unmarshalJSON(row.Payload),
		SendAt:         toTime(row.SendAt),
		LockedAt:       toTimePtr(row.LockedAt),
		LockedBy:       toStrPtr(row.LockedBy),
		HeartbeatAt:    toTimePtr(row.HeartbeatAt),
		RetryCount:     row.RetryCount,
		MaxRetries:     row.MaxRetries,
		NextRetryAt:    toTimePtr(row.NextRetryAt),
		LastError:      toStrPtr(row.LastError),
		CreatedAt:      toTime(row.CreatedAt),
		UpdatedAt:      toTime(row.UpdatedAt),
	}
}
