package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/kennedyvnak/beaconbird/internal/api/handler"
	"github.com/kennedyvnak/beaconbird/internal/domain"
	"github.com/kennedyvnak/beaconbird/internal/repository"
	"github.com/kennedyvnak/beaconbird/internal/service"
)

func TestNotificationReaderAdapterMapsNotFound(t *testing.T) {
	adapter := notificationReaderAdapter{
		repo: fakeNotificationReadRepo{getErr: pgx.ErrNoRows},
	}

	_, err := adapter.GetByID(context.Background(), "missing")
	if !errors.Is(err, service.ErrNotFound) {
		t.Fatalf("expected not found sentinel, got %v", err)
	}
}

func TestDeliveryJobReaderAdapterMapsNotFound(t *testing.T) {
	adapter := deliveryJobReaderAdapter{
		repo: &fakeDeliveryJobReadRepo{getErr: pgx.ErrNoRows},
	}

	_, err := adapter.GetByID(context.Background(), "missing")
	if !errors.Is(err, service.ErrNotFound) {
		t.Fatalf("expected not found sentinel, got %v", err)
	}
}

func TestDeliveryJobReaderAdapterForwardsNotificationIDAndPaging(t *testing.T) {
	repo := &fakeDeliveryJobReadRepo{}
	adapter := deliveryJobReaderAdapter{repo: repo}

	_, _ = adapter.List(context.Background(), handler.DeliveryJobListParams{
		Status:         "pending",
		Channel:        "email",
		NotificationID: "notif-1",
		From:           time.Unix(1, 0).UTC(),
		To:             time.Unix(2, 0).UTC(),
		Offset:         5,
		Limit:          25,
	})

	if repo.listParams.NotificationID != "notif-1" {
		t.Fatalf("expected notification filter to be forwarded")
	}
	if repo.listParams.Offset != 5 || repo.listParams.Limit != 25 {
		t.Fatalf("expected paging to be forwarded, got offset=%d limit=%d", repo.listParams.Offset, repo.listParams.Limit)
	}
}

func TestNotificationReaderAdapterForwardsChannelAndPaging(t *testing.T) {
	repo := &fakeNotificationListRepo{}
	adapter := notificationReaderAdapter{repo: repo}

	_, _ = adapter.List(context.Background(), handler.NotificationListParams{
		Status:  "pending",
		Tag:     "marketing",
		Channel: "push",
		From:    time.Unix(1, 0).UTC(),
		To:      time.Unix(2, 0).UTC(),
		Offset:  5,
		Limit:   25,
	})

	if repo.listParams.Channel != "push" {
		t.Fatalf("expected channel filter to be forwarded")
	}
	if repo.listParams.Offset != 5 || repo.listParams.Limit != 25 {
		t.Fatalf("expected paging to be forwarded, got offset=%d limit=%d", repo.listParams.Offset, repo.listParams.Limit)
	}
}

type fakeNotificationReadRepo struct {
	getErr error
}

func (f fakeNotificationReadRepo) GetByID(ctx context.Context, id string) (*domain.Notification, error) {
	return nil, f.getErr
}

func (f fakeNotificationReadRepo) List(ctx context.Context, params repository.ListNotificationsParams) ([]*domain.Notification, error) {
	return nil, nil
}

type fakeNotificationListRepo struct {
	listParams repository.ListNotificationsParams
}

func (f *fakeNotificationListRepo) GetByID(ctx context.Context, id string) (*domain.Notification, error) {
	return nil, nil
}

func (f *fakeNotificationListRepo) List(ctx context.Context, params repository.ListNotificationsParams) ([]*domain.Notification, error) {
	f.listParams = params
	return nil, nil
}

type fakeDeliveryJobReadRepo struct {
	getErr     error
	listParams repository.ListDeliveryJobsParams
}

func (f *fakeDeliveryJobReadRepo) GetByID(ctx context.Context, id string) (*domain.DeliveryJob, error) {
	return nil, f.getErr
}

func (f *fakeDeliveryJobReadRepo) List(ctx context.Context, params repository.ListDeliveryJobsParams) ([]*domain.DeliveryJob, error) {
	f.listParams = params
	return nil, nil
}
