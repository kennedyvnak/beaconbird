package service

import (
	"context"
	"errors"
)

var ErrNotImplemented = errors.New("service not implemented")

// NotificationService will coordinate idempotent ingestion and status transitions.
type NotificationService struct{}

func (s *NotificationService) Ingest(ctx context.Context) error {
	_ = ctx
	return ErrNotImplemented
}

func (s *NotificationService) Cancel(ctx context.Context) error {
	_ = ctx
	return ErrNotImplemented
}

func (s *NotificationService) Reschedule(ctx context.Context) error {
	_ = ctx
	return ErrNotImplemented
}

func (s *NotificationService) EditContent(ctx context.Context) error {
	_ = ctx
	return ErrNotImplemented
}
