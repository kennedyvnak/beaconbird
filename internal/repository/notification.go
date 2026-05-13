package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/kennedyvnak/beaconbird/internal/domain"
	"github.com/kennedyvnak/beaconbird/internal/repository/sqlc"
)

type NotificationRepo struct {
	db sqlc.DBTX
	q  *sqlc.Queries
}

func NewNotificationRepo(db sqlc.DBTX, q *sqlc.Queries) *NotificationRepo {
	return &NotificationRepo{db: db, q: q}
}

func (r *NotificationRepo) Create(ctx context.Context, n *domain.Notification) (*domain.Notification, error) {
	row, err := queriesFor(ctx, r.q).CreateNotification(ctx, sqlc.CreateNotificationParams{
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
	row, err := queriesFor(ctx, r.q).GetNotification(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("notificationRepo.GetByID: %w", err)
	}
	return toNotification(row), nil
}

func (r *NotificationRepo) GetByIdempotencyKey(ctx context.Context, key string) (*domain.Notification, error) {
	row, err := queriesFor(ctx, r.q).GetNotificationByIdempotencyKey(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("notificationRepo.GetByIdempotencyKey: %w", err)
	}
	return toNotification(row), nil
}

type ListNotificationsParams struct {
	Status  string
	Tag     string
	Channel string
	From    time.Time
	To      time.Time
	Offset  int32
	Limit   int32
}

func (r *NotificationRepo) List(ctx context.Context, p ListNotificationsParams) ([]*domain.Notification, error) {
	q := queriesFor(ctx, r.q)
	var (
		rows []sqlc.Notification
		err  error
	)
	if p.Channel == "" {
		rows, err = q.ListNotifications(ctx, sqlc.ListNotificationsParams{
			Status:      p.Status,
			Tag:         p.Tag,
			FromTime:    toPgTime(p.From),
			ToTime:      toPgTime(p.To),
			OffsetCount: p.Offset,
			LimitCount:  p.Limit,
		})
	} else {
		rows, err = q.ListNotificationsByChannel(ctx, sqlc.ListNotificationsByChannelParams{
			Status:      p.Status,
			Tag:         p.Tag,
			Channel:     p.Channel,
			FromTime:    toPgTime(p.From),
			ToTime:      toPgTime(p.To),
			OffsetCount: p.Offset,
			LimitCount:  p.Limit,
		})
	}
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
	row, err := queriesFor(ctx, r.q).UpdateNotificationStatus(ctx, sqlc.UpdateNotificationStatusParams{
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
	row, err := queriesFor(ctx, r.q).CancelNotification(ctx, sqlc.CancelNotificationParams{
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
	row, err := queriesFor(ctx, r.q).RescheduleNotification(ctx, sqlc.RescheduleNotificationParams{
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
	row, err := queriesFor(ctx, r.q).UpdateNotificationContent(ctx, sqlc.UpdateNotificationContentParams{
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

func (r *NotificationRepo) Delete(ctx context.Context, id string) error {
	rows, err := queriesFor(ctx, r.q).DeleteNotification(ctx, id)
	if err != nil {
		return fmt.Errorf("notificationRepo.Delete: %w", err)
	}
	if rows == 0 {
		return pgx.ErrNoRows
	}
	return nil
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
