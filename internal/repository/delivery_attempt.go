package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kennedyvnak/beaconbird/internal/domain"
	"github.com/kennedyvnak/beaconbird/internal/repository/sqlc"
)

type DeliveryAttemptRepo struct {
	q *sqlc.Queries
}

func NewDeliveryAttemptRepo(q *sqlc.Queries) *DeliveryAttemptRepo {
	return &DeliveryAttemptRepo{q: q}
}

func (r *DeliveryAttemptRepo) Create(ctx context.Context, a *domain.DeliveryAttempt) (*domain.DeliveryAttempt, error) {
	row, err := queriesFor(ctx, r.q).CreateDeliveryAttempt(ctx, sqlc.CreateDeliveryAttemptParams{
		ID:              a.ID,
		DeliveryJobID:   a.DeliveryJobID,
		AttemptNumber:   a.AttemptNumber,
		ProviderName:    a.ProviderName,
		Status:          string(a.Status),
		RequestPayload:  marshalJSON(a.RequestPayload),
		ResponsePayload: marshalJSON(a.ResponsePayload),
		ErrorMessage:    toPgText(a.ErrorMessage),
		DurationMs:      toPgInt4(a.DurationMS),
		AttemptedAt:     toPgTime(a.AttemptedAt),
	})
	if err != nil {
		return nil, fmt.Errorf("deliveryAttemptRepo.Create: %w", err)
	}
	return toDeliveryAttempt(row), nil
}

func (r *DeliveryAttemptRepo) ListByJob(ctx context.Context, jobID string) ([]*domain.DeliveryAttempt, error) {
	rows, err := queriesFor(ctx, r.q).ListDeliveryAttemptsByJob(ctx, jobID)
	if err != nil {
		return nil, fmt.Errorf("deliveryAttemptRepo.ListByJob: %w", err)
	}
	result := make([]*domain.DeliveryAttempt, len(rows))
	for i, row := range rows {
		result[i] = toDeliveryAttempt(row)
	}
	return result, nil
}

func toDeliveryAttempt(row sqlc.DeliveryAttempt) *domain.DeliveryAttempt {
	var attemptedAt time.Time
	if row.AttemptedAt != (pgtype.Timestamptz{}) && row.AttemptedAt.Valid {
		attemptedAt = row.AttemptedAt.Time.UTC()
	}
	return &domain.DeliveryAttempt{
		ID:              row.ID,
		DeliveryJobID:   row.DeliveryJobID,
		AttemptNumber:   row.AttemptNumber,
		ProviderName:    row.ProviderName,
		Status:          domain.DeliveryJobStatus(row.Status),
		RequestPayload:  unmarshalJSON(row.RequestPayload),
		ResponsePayload: unmarshalJSON(row.ResponsePayload),
		ErrorMessage:    toStrPtr(row.ErrorMessage),
		DurationMS:      toInt32Ptr(row.DurationMs),
		AttemptedAt:     attemptedAt,
	}
}
