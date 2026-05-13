package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/kennedyvnak/beaconbird/internal/domain"
	"github.com/kennedyvnak/beaconbird/internal/repository/sqlc"
)

type DeliveryJobRepo struct {
	db sqlc.DBTX
	q  *sqlc.Queries
}

func NewDeliveryJobRepo(db sqlc.DBTX, q *sqlc.Queries) *DeliveryJobRepo {
	return &DeliveryJobRepo{db: db, q: q}
}

func (r *DeliveryJobRepo) Create(ctx context.Context, j *domain.DeliveryJob) (*domain.DeliveryJob, error) {
	row, err := queriesFor(ctx, r.q).CreateDeliveryJob(ctx, sqlc.CreateDeliveryJobParams{
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
	row, err := queriesFor(ctx, r.q).GetDeliveryJob(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("deliveryJobRepo.GetByID: %w", err)
	}
	return toDeliveryJob(row), nil
}

type ListDeliveryJobsParams struct {
	Status         string
	Channel        string
	NotificationID string
	From           time.Time
	To             time.Time
	Offset         int32
	Limit          int32
}

func (r *DeliveryJobRepo) List(ctx context.Context, p ListDeliveryJobsParams) ([]*domain.DeliveryJob, error) {
	rows, err := queriesFor(ctx, r.q).ListDeliveryJobs(ctx, sqlc.ListDeliveryJobsParams{
		Status:         p.Status,
		Channel:        p.Channel,
		NotificationID: p.NotificationID,
		FromTime:       toPgTime(p.From),
		ToTime:         toPgTime(p.To),
		OffsetCount:    p.Offset,
		LimitCount:     p.Limit,
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
	rows, err := queriesFor(ctx, r.q).ListReadyDeliveryJobs(ctx, sqlc.ListReadyDeliveryJobsParams{
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
	row, err := queriesFor(ctx, r.q).MarkDeliveryJobProcessing(ctx, sqlc.MarkDeliveryJobProcessingParams{
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
	row, err := queriesFor(ctx, r.q).MarkDeliveryJobSent(ctx, sqlc.MarkDeliveryJobSentParams{
		ID:        id,
		UpdatedAt: toPgTime(time.Now().UTC()),
	})
	if err != nil {
		return nil, fmt.Errorf("deliveryJobRepo.MarkSent: %w", err)
	}
	return toDeliveryJob(row), nil
}

func (r *DeliveryJobRepo) MarkRetry(ctx context.Context, id string, nextRetryAt time.Time, lastError *string) (*domain.DeliveryJob, error) {
	row, err := queriesFor(ctx, r.q).MarkDeliveryJobForRetry(ctx, sqlc.MarkDeliveryJobForRetryParams{
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
	row, err := queriesFor(ctx, r.q).MarkDeliveryJobDeadLetter(ctx, sqlc.MarkDeliveryJobDeadLetterParams{
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
	rows, err := queriesFor(ctx, r.q).CancelDeliveryJobsForNotification(ctx, sqlc.CancelDeliveryJobsForNotificationParams{
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
	rows, err := queriesFor(ctx, r.q).ResetStaleDeliveryJobs(ctx, sqlc.ResetStaleDeliveryJobsParams{
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
	_, err := queriesFor(ctx, r.q).UpdateHeartbeatForWorker(ctx, sqlc.UpdateHeartbeatForWorkerParams{
		LockedBy:    toPgText(&workerID),
		HeartbeatAt: toPgTime(now),
		UpdatedAt:   toPgTime(now),
	})
	if err != nil {
		return fmt.Errorf("deliveryJobRepo.UpdateHeartbeat: %w", err)
	}
	return nil
}

func (r *DeliveryJobRepo) CreateBatch(ctx context.Context, jobs []*domain.DeliveryJob) error {
	if q := queriesFor(ctx, r.q); q != r.q {
		for _, job := range jobs {
			_, err := q.CreateDeliveryJob(ctx, sqlc.CreateDeliveryJobParams{
				ID:             job.ID,
				NotificationID: job.NotificationID,
				ProviderID:     toPgText(job.ProviderID),
				Channel:        string(job.Channel),
				Status:         string(job.Status),
				Payload:        marshalJSON(job.Payload),
				SendAt:         toPgTime(job.SendAt),
				MaxRetries:     job.MaxRetries,
			})
			if err != nil {
				return fmt.Errorf("deliveryJobRepo.CreateBatch create job: %w", err)
			}
		}
		return nil
	}

	beginner, ok := r.db.(interface {
		Begin(context.Context) (pgx.Tx, error)
	})
	if !ok {
		return fmt.Errorf("deliveryJobRepo.CreateBatch: db does not support transactions")
	}

	tx, err := beginner.Begin(ctx)
	if err != nil {
		return fmt.Errorf("deliveryJobRepo.CreateBatch begin: %w", err)
	}
	defer tx.Rollback(ctx)

	q := r.q.WithTx(tx)
	for _, job := range jobs {
		_, err := q.CreateDeliveryJob(ctx, sqlc.CreateDeliveryJobParams{
			ID:             job.ID,
			NotificationID: job.NotificationID,
			ProviderID:     toPgText(job.ProviderID),
			Channel:        string(job.Channel),
			Status:         string(job.Status),
			Payload:        marshalJSON(job.Payload),
			SendAt:         toPgTime(job.SendAt),
			MaxRetries:     job.MaxRetries,
		})
		if err != nil {
			return fmt.Errorf("deliveryJobRepo.CreateBatch create job: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("deliveryJobRepo.CreateBatch commit: %w", err)
	}
	return nil
}

func (r *DeliveryJobRepo) RescheduleForNotification(ctx context.Context, notificationID string, sendAt time.Time) error {
	_, err := queriesFor(ctx, r.q).RescheduleDeliveryJobsForNotification(ctx, sqlc.RescheduleDeliveryJobsForNotificationParams{
		SendAt:         toPgTime(sendAt),
		UpdatedAt:      toPgTime(time.Now().UTC()),
		NotificationID: notificationID,
	})
	if err != nil {
		return fmt.Errorf("deliveryJobRepo.RescheduleForNotification: %w", err)
	}
	return nil
}

func (r *DeliveryJobRepo) UpdatePayloadForNotification(ctx context.Context, notificationID string, payload map[string]any) error {
	rowsAffected, err := queriesFor(ctx, r.q).UpdateDeliveryJobPayloadForNotification(ctx, sqlc.UpdateDeliveryJobPayloadForNotificationParams{
		Payload:        marshalJSON(payload),
		UpdatedAt:      toPgTime(time.Now().UTC()),
		NotificationID: notificationID,
	})
	if err != nil {
		return fmt.Errorf("deliveryJobRepo.UpdatePayloadForNotification: %w", err)
	}
	if rowsAffected == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *DeliveryJobRepo) ListByNotificationID(ctx context.Context, notificationID string) ([]*domain.DeliveryJob, error) {
	rows, err := queriesFor(ctx, r.q).ListDeliveryJobsByNotificationID(ctx, notificationID)
	if err != nil {
		return nil, fmt.Errorf("deliveryJobRepo.ListByNotificationID: %w", err)
	}
	result := make([]*domain.DeliveryJob, len(rows))
	for i, row := range rows {
		result[i] = toDeliveryJob(row)
	}
	return result, nil
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
