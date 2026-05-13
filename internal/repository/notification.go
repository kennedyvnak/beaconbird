package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/kennedyvnak/beaconbird/internal/domain"
	"github.com/kennedyvnak/beaconbird/internal/repository/sqlc"
)

type NotificationRepo struct {
	q *sqlc.Queries
}

func NewNotificationRepo(q *sqlc.Queries) *NotificationRepo {
	return &NotificationRepo{q: q}
}

func (r *NotificationRepo) Create(ctx context.Context, n *domain.Notification) (*domain.Notification, error) {
	row, err := r.q.CreateNotification(ctx, sqlc.CreateNotificationParams{
		ID:             n.ID,
		IdempotencyKey: n.IdempotencyKey,
		Status:         string(n.Status),
		Tag:            n.Tag,
		Content:        marshalJSON(n.Content),
		Metadata:       marshalJSON(n.Metadata),
		SendAt:         toPgTime(n.SendAt),
	})
	if err != nil {
		return nil, fmt.Errorf("notificationRepo.Create: %w", err)
	}
	return toNotification(row), nil
}

func (r *NotificationRepo) GetByID(ctx context.Context, id string) (*domain.Notification, error) {
	row, err := r.q.GetNotification(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("notificationRepo.GetByID: %w", err)
	}
	return toNotification(row), nil
}

func (r *NotificationRepo) GetByIdempotencyKey(ctx context.Context, key string) (*domain.Notification, error) {
	row, err := r.q.GetNotificationByIdempotencyKey(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("notificationRepo.GetByIdempotencyKey: %w", err)
	}
	return toNotification(row), nil
}

type ListNotificationsParams struct {
	Status string
	Tag    string
	From   time.Time
	To     time.Time
	Offset int32
	Limit  int32
}

func (r *NotificationRepo) List(ctx context.Context, p ListNotificationsParams) ([]*domain.Notification, error) {
	rows, err := r.q.ListNotifications(ctx, sqlc.ListNotificationsParams{
		Status:      p.Status,
		Tag:         p.Tag,
		FromTime:    toPgTime(p.From),
		ToTime:      toPgTime(p.To),
		OffsetCount: p.Offset,
		LimitCount:  p.Limit,
	})
	if err != nil {
		return nil, fmt.Errorf("notificationRepo.List: %w", err)
	}
	result := make([]*domain.Notification, len(rows))
	for i, row := range rows {
		result[i] = toNotification(row)
	}
	return result, nil
}

func (r *NotificationRepo) UpdateStatus(ctx context.Context, id string, status domain.NotificationStatus) (*domain.Notification, error) {
	row, err := r.q.UpdateNotificationStatus(ctx, sqlc.UpdateNotificationStatusParams{
		ID:        id,
		Status:    string(status),
		UpdatedAt: toPgTime(time.Now().UTC()),
	})
	if err != nil {
		return nil, fmt.Errorf("notificationRepo.UpdateStatus: %w", err)
	}
	return toNotification(row), nil
}

func (r *NotificationRepo) Cancel(ctx context.Context, id string) (*domain.Notification, error) {
	now := time.Now().UTC()
	row, err := r.q.CancelNotification(ctx, sqlc.CancelNotificationParams{
		ID:          id,
		CancelledAt: toPgTime(now),
		UpdatedAt:   toPgTime(now),
	})
	if err != nil {
		return nil, fmt.Errorf("notificationRepo.Cancel: %w", err)
	}
	return toNotification(row), nil
}

func (r *NotificationRepo) Reschedule(ctx context.Context, id string, sendAt time.Time) (*domain.Notification, error) {
	row, err := r.q.RescheduleNotification(ctx, sqlc.RescheduleNotificationParams{
		ID:        id,
		SendAt:    toPgTime(sendAt),
		UpdatedAt: toPgTime(time.Now().UTC()),
	})
	if err != nil {
		return nil, fmt.Errorf("notificationRepo.Reschedule: %w", err)
	}
	return toNotification(row), nil
}

func (r *NotificationRepo) UpdateContent(ctx context.Context, id string, content, metadata map[string]any) (*domain.Notification, error) {
	row, err := r.q.UpdateNotificationContent(ctx, sqlc.UpdateNotificationContentParams{
		ID:        id,
		Content:   marshalJSON(content),
		Metadata:  marshalJSON(metadata),
		UpdatedAt: toPgTime(time.Now().UTC()),
	})
	if err != nil {
		return nil, fmt.Errorf("notificationRepo.UpdateContent: %w", err)
	}
	return toNotification(row), nil
}

func toNotification(row sqlc.Notification) *domain.Notification {
	return &domain.Notification{
		ID:             row.ID,
		IdempotencyKey: row.IdempotencyKey,
		Status:         domain.NotificationStatus(row.Status),
		Tag:            row.Tag,
		Content:        unmarshalJSON(row.Content),
		Metadata:       unmarshalJSON(row.Metadata),
		SendAt:         toTime(row.SendAt),
		CreatedAt:      toTime(row.CreatedAt),
		UpdatedAt:      toTime(row.UpdatedAt),
		CancelledAt:    toTimePtr(row.CancelledAt),
	}
}
