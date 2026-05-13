package service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/kennedyvnak/beaconbird/internal/domain"
)

var (
	ErrNotFound         = errors.New("not found")
	ErrAlreadyCancelled = errors.New("notification cannot be cancelled")
	ErrAlreadySent      = errors.New("notification can no longer be modified")
	ErrSendAtInPast     = errors.New("send_at is too far in the past")
	ErrInvalidSendAt    = errors.New("invalid send_at")
)

type notificationRepo interface {
	Create(ctx context.Context, n *domain.Notification) (*domain.Notification, error)
	GetByID(ctx context.Context, id string) (*domain.Notification, error)
	GetByIdempotencyKey(ctx context.Context, key string) (*domain.Notification, error)
	Cancel(ctx context.Context, id string) (*domain.Notification, error)
	Reschedule(ctx context.Context, id string, sendAt time.Time) (*domain.Notification, error)
	UpdateContent(ctx context.Context, id string, content, metadata map[string]any) (*domain.Notification, error)
	Delete(ctx context.Context, id string) error
}

type deliveryJobRepo interface {
	CreateBatch(ctx context.Context, jobs []*domain.DeliveryJob) error
	CancelForNotification(ctx context.Context, notificationID string) ([]*domain.DeliveryJob, error)
	RescheduleForNotification(ctx context.Context, notificationID string, sendAt time.Time) error
	UpdatePayloadForNotification(ctx context.Context, notificationID string, payload map[string]any) error
	ListByNotificationID(ctx context.Context, notificationID string) ([]*domain.DeliveryJob, error)
}

type DeliveryTarget struct {
	Channel domain.Channel
	Payload map[string]any
}

type IngestParams struct {
	Deliveries     []DeliveryTarget
	SendAt         string
	IdempotencyKey string
	Tag            string
	Metadata       map[string]any
}

type EditContentParams struct {
	Content  map[string]any
	Metadata map[string]any
}

type NotificationService struct {
	notifications notificationRepo
	jobs          deliveryJobRepo
	txRunner      notificationTxRunner
	now           func() time.Time
	idGenerator   func() string
	maxRetries    int32
}

type notificationTxRunner interface {
	RunInTx(ctx context.Context, fn func(context.Context) error) error
}

func NewNotificationService(notifications notificationRepo, jobs deliveryJobRepo, txRunners ...notificationTxRunner) *NotificationService {
	svc := &NotificationService{
		notifications: notifications,
		jobs:          jobs,
		now: func() time.Time {
			return time.Now().UTC()
		},
		idGenerator: newID,
		maxRetries:  3,
	}
	if len(txRunners) > 0 {
		svc.txRunner = txRunners[0]
	}
	return svc
}

func (s *NotificationService) Ingest(ctx context.Context, params IngestParams) (*domain.Notification, bool, error) {
	existing, err := s.notifications.GetByIdempotencyKey(ctx, params.IdempotencyKey)
	switch {
	case err == nil:
		return existing, true, nil
	case !errors.Is(err, pgx.ErrNoRows):
		return nil, false, fmt.Errorf("notificationService.Ingest lookup: %w", err)
	}

	sendAt, err := s.parseSendAt(params.SendAt)
	if err != nil {
		return nil, false, err
	}

	notification := &domain.Notification{
		ID:             s.idGenerator(),
		IdempotencyKey: params.IdempotencyKey,
		Status:         domain.NotificationStatusPending,
		Tag:            params.Tag,
		Content:        buildNotificationContent(params.Deliveries),
		Metadata:       cloneMap(params.Metadata),
		SendAt:         sendAt,
	}

	created, err := s.notifications.Create(ctx, notification)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			existing, getErr := s.notifications.GetByIdempotencyKey(ctx, params.IdempotencyKey)
			if getErr != nil {
				return nil, false, fmt.Errorf("notificationService.Ingest lookup after conflict: %w", getErr)
			}
			return existing, true, nil
		}
		return nil, false, fmt.Errorf("notificationService.Ingest create notification: %w", err)
	}

	jobs := make([]*domain.DeliveryJob, 0, len(params.Deliveries))
	for _, delivery := range params.Deliveries {
		jobs = append(jobs, &domain.DeliveryJob{
			ID:             s.idGenerator(),
			NotificationID: created.ID,
			Channel:        delivery.Channel,
			Status:         domain.DeliveryJobStatusPending,
			Payload:        cloneMap(delivery.Payload),
			SendAt:         sendAt,
			MaxRetries:     s.maxRetries,
		})
	}

	if err := s.jobs.CreateBatch(ctx, jobs); err != nil {
		if deleteErr := s.notifications.Delete(ctx, created.ID); deleteErr != nil {
			return nil, false, fmt.Errorf("notificationService.Ingest create jobs: %w; rollback notification: %v", err, deleteErr)
		}
		return nil, false, fmt.Errorf("notificationService.Ingest create jobs: %w", err)
	}

	return created, false, nil
}

func (s *NotificationService) Cancel(ctx context.Context, id string) error {
	notification, err := s.notifications.GetByID(ctx, id)
	if err != nil {
		return mapNotFound("notificationService.Cancel get notification", err)
	}
	if notification.Status != domain.NotificationStatusPending {
		if notification.Status == domain.NotificationStatusCancelled {
			return ErrAlreadyCancelled
		}
		return ErrAlreadySent
	}

	return s.runInTx(ctx, func(txCtx context.Context) error {
		if _, err := s.notifications.Cancel(txCtx, id); err != nil {
			return fmt.Errorf("notificationService.Cancel cancel notification: %w", err)
		}
		if _, err := s.jobs.CancelForNotification(txCtx, id); err != nil {
			return fmt.Errorf("notificationService.Cancel cancel jobs: %w", err)
		}
		return nil
	})
}

func (s *NotificationService) Reschedule(ctx context.Context, id string, sendAt time.Time) error {
	notification, err := s.notifications.GetByID(ctx, id)
	if err != nil {
		return mapNotFound("notificationService.Reschedule get notification", err)
	}
	if notification.Status != domain.NotificationStatusPending {
		return ErrAlreadySent
	}
	if sendAt.UTC().Before(s.now().Add(-30 * time.Second)) {
		return ErrSendAtInPast
	}

	sendAt = sendAt.UTC()
	return s.runInTx(ctx, func(txCtx context.Context) error {
		if _, err := s.notifications.Reschedule(txCtx, id, sendAt); err != nil {
			return fmt.Errorf("notificationService.Reschedule update notification: %w", err)
		}
		if err := s.jobs.RescheduleForNotification(txCtx, id, sendAt); err != nil {
			return fmt.Errorf("notificationService.Reschedule update jobs: %w", err)
		}
		return nil
	})
}

func (s *NotificationService) EditContent(ctx context.Context, id string, params EditContentParams) error {
	notification, err := s.notifications.GetByID(ctx, id)
	if err != nil {
		return mapNotFound("notificationService.EditContent get notification", err)
	}

	jobs, err := s.jobs.ListByNotificationID(ctx, id)
	if err != nil {
		return fmt.Errorf("notificationService.EditContent list jobs: %w", err)
	}
	for _, job := range jobs {
		if job.Status != domain.DeliveryJobStatusPending {
			if job.Status == domain.DeliveryJobStatusCancelled {
				return ErrAlreadyCancelled
			}
			return ErrAlreadySent
		}
	}

	content := cloneMap(params.Content)
	metadata := cloneMap(params.Metadata)
	if metadata == nil {
		metadata = cloneMap(notification.Metadata)
	}
	return s.runInTx(ctx, func(txCtx context.Context) error {
		if _, err := s.notifications.UpdateContent(txCtx, id, content, metadata); err != nil {
			return fmt.Errorf("notificationService.EditContent update notification: %w", err)
		}
		if err := s.jobs.UpdatePayloadForNotification(txCtx, id, content); err != nil {
			return fmt.Errorf("notificationService.EditContent update jobs: %w", err)
		}
		return nil
	})
}

func (s *NotificationService) parseSendAt(raw string) (time.Time, error) {
	if raw == "now" {
		return s.now(), nil
	}

	sendAt, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: %v", ErrInvalidSendAt, err)
	}
	sendAt = sendAt.UTC()
	if sendAt.Before(s.now().Add(-30 * time.Second)) {
		return time.Time{}, ErrSendAtInPast
	}
	return sendAt, nil
}

func (s *NotificationService) runInTx(ctx context.Context, fn func(context.Context) error) error {
	if s.txRunner == nil {
		return fn(ctx)
	}
	return s.txRunner.RunInTx(ctx, fn)
}

func mapNotFound(prefix string, err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return fmt.Errorf("%s: %w", prefix, err)
}

func buildNotificationContent(deliveries []DeliveryTarget) map[string]any {
	if len(deliveries) == 1 {
		return cloneMap(deliveries[0].Payload)
	}

	items := make([]map[string]any, 0, len(deliveries))
	for _, delivery := range deliveries {
		items = append(items, map[string]any{
			"channel": string(delivery.Channel),
			"payload": cloneMap(delivery.Payload),
		})
	}
	return map[string]any{"deliveries": items}
}

func cloneMap(src map[string]any) map[string]any {
	if src == nil {
		return nil
	}
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func newID() string {
	buf := make([]byte, 16)
	_, _ = rand.Read(buf)
	return fmt.Sprintf("%x", buf)
}
