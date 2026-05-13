package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/kennedyvnak/beaconbird/internal/domain"
)

var errBoomService = errors.New("boom")

func TestNotificationServiceIngestReturnsExistingNotification(t *testing.T) {
	existing := &domain.Notification{
		ID:             "notif-existing",
		IdempotencyKey: "idem-1",
		Status:         domain.NotificationStatusPending,
	}

	notifications := &fakeNotificationRepo{
		byIdempotencyKey: map[string]*domain.Notification{"idem-1": existing},
	}
	jobs := &fakeDeliveryJobRepo{}
	svc := NewNotificationService(notifications, jobs)

	got, wasExisting, err := svc.Ingest(context.Background(), IngestParams{
		IdempotencyKey: "idem-1",
		SendAt:         "now",
		Deliveries: []DeliveryTarget{
			{Channel: domain.ChannelEmail, Payload: map[string]any{"subject": "hello"}},
		},
	})
	if err != nil {
		t.Fatalf("Ingest returned error: %v", err)
	}
	if !wasExisting {
		t.Fatalf("expected existing notification")
	}
	if got.ID != existing.ID {
		t.Fatalf("expected %q, got %q", existing.ID, got.ID)
	}
	if notifications.createCalls != 0 {
		t.Fatalf("expected no create call, got %d", notifications.createCalls)
	}
	if jobs.createBatchCalls != 0 {
		t.Fatalf("expected no job create call, got %d", jobs.createBatchCalls)
	}
}

func TestNotificationServiceIngestCreatesNotificationAndJobs(t *testing.T) {
	notifications := &fakeNotificationRepo{}
	jobs := &fakeDeliveryJobRepo{}
	now := time.Date(2026, 5, 13, 19, 0, 0, 0, time.UTC)
	svc := NewNotificationService(notifications, jobs)
	svc.now = func() time.Time { return now }
	svc.idGenerator = func() string {
		ids := []string{"notif-1", "job-1", "job-2"}
		next := ids[0]
		ids = ids[1:]
		svc.idGenerator = func() string {
			v := ids[0]
			ids = ids[1:]
			return v
		}
		return next
	}

	got, wasExisting, err := svc.Ingest(context.Background(), IngestParams{
		IdempotencyKey: "idem-2",
		SendAt:         "now",
		Tag:            "marketing",
		Metadata:       map[string]any{"campaign": "launch"},
		Deliveries: []DeliveryTarget{
			{Channel: domain.ChannelEmail, Payload: map[string]any{"subject": "hello"}},
			{Channel: domain.ChannelPush, Payload: map[string]any{"title": "hi"}},
		},
	})
	if err != nil {
		t.Fatalf("Ingest returned error: %v", err)
	}
	if wasExisting {
		t.Fatalf("expected newly created notification")
	}
	if got.ID != "notif-1" {
		t.Fatalf("expected notification ID notif-1, got %q", got.ID)
	}
	if notifications.created == nil {
		t.Fatalf("expected notification to be created")
	}
	if notifications.created.Tag != "marketing" {
		t.Fatalf("expected tag to be preserved")
	}
	if len(jobs.created) != 2 {
		t.Fatalf("expected 2 delivery jobs, got %d", len(jobs.created))
	}
	if jobs.created[0].NotificationID != "notif-1" || jobs.created[1].NotificationID != "notif-1" {
		t.Fatalf("expected jobs to point at created notification")
	}
	if jobs.created[0].Status != domain.DeliveryJobStatusPending {
		t.Fatalf("expected pending delivery job status, got %q", jobs.created[0].Status)
	}
}

func TestNotificationServiceIngestSetsIsTestOnCreatedJobs(t *testing.T) {
	notifications := &fakeNotificationRepo{}
	jobs := &fakeDeliveryJobRepo{}
	svc := NewNotificationService(notifications, jobs)
	svc.idGenerator = func() string {
		ids := []string{"notif-1", "job-1"}
		next := ids[0]
		ids = ids[1:]
		svc.idGenerator = func() string {
			v := ids[0]
			ids = ids[1:]
			return v
		}
		return next
	}

	_, _, err := svc.Ingest(context.Background(), IngestParams{
		IdempotencyKey: "idem-test",
		SendAt:         "now",
		IsTest:         true,
		Deliveries: []DeliveryTarget{
			{Channel: domain.ChannelPush, Payload: map[string]any{"title": "hello"}},
		},
	})
	if err != nil {
		t.Fatalf("Ingest returned error: %v", err)
	}
	if len(jobs.created) != 1 {
		t.Fatalf("expected 1 created job, got %d", len(jobs.created))
	}
	if !jobs.created[0].IsTest {
		t.Fatalf("expected created job to be marked as test")
	}
}

func TestNotificationServiceIngestRejectsPastSendAt(t *testing.T) {
	notifications := &fakeNotificationRepo{}
	jobs := &fakeDeliveryJobRepo{}
	now := time.Date(2026, 5, 13, 19, 0, 0, 0, time.UTC)
	svc := NewNotificationService(notifications, jobs)
	svc.now = func() time.Time { return now }

	_, _, err := svc.Ingest(context.Background(), IngestParams{
		IdempotencyKey: "idem-3",
		SendAt:         now.Add(-31 * time.Second).Format(time.RFC3339),
		Deliveries: []DeliveryTarget{
			{Channel: domain.ChannelEmail, Payload: map[string]any{"subject": "hello"}},
		},
	})
	if !errors.Is(err, ErrSendAtInPast) {
		t.Fatalf("expected ErrSendAtInPast, got %v", err)
	}
}

func TestNotificationServiceIngestRejectsMalformedSendAt(t *testing.T) {
	svc := NewNotificationService(&fakeNotificationRepo{}, &fakeDeliveryJobRepo{})

	_, _, err := svc.Ingest(context.Background(), IngestParams{
		IdempotencyKey: "idem-3",
		SendAt:         "not-a-timestamp",
		Deliveries: []DeliveryTarget{
			{Channel: domain.ChannelEmail, Payload: map[string]any{"subject": "hello"}},
		},
	})
	if !errors.Is(err, ErrInvalidSendAt) {
		t.Fatalf("expected ErrInvalidSendAt, got %v", err)
	}
}

func TestNotificationServiceCancelRejectsNonPendingNotifications(t *testing.T) {
	notifications := &fakeNotificationRepo{
		byID: map[string]*domain.Notification{
			"notif-1": {ID: "notif-1", Status: domain.NotificationStatusSent},
		},
	}
	svc := NewNotificationService(notifications, &fakeDeliveryJobRepo{})

	err := svc.Cancel(context.Background(), "notif-1")
	if !errors.Is(err, ErrAlreadySent) {
		t.Fatalf("expected ErrAlreadySent, got %v", err)
	}
}

func TestNotificationServiceCancelRejectsAlreadyCancelledNotifications(t *testing.T) {
	notifications := &fakeNotificationRepo{
		byID: map[string]*domain.Notification{
			"notif-1": {ID: "notif-1", Status: domain.NotificationStatusCancelled},
		},
	}
	svc := NewNotificationService(notifications, &fakeDeliveryJobRepo{})

	err := svc.Cancel(context.Background(), "notif-1")
	if !errors.Is(err, ErrAlreadyCancelled) {
		t.Fatalf("expected ErrAlreadyCancelled, got %v", err)
	}
}

func TestNotificationServiceCancelRollsBackWhenJobCancellationFails(t *testing.T) {
	notifications := &fakeNotificationRepo{
		byID: map[string]*domain.Notification{
			"notif-1": {ID: "notif-1", Status: domain.NotificationStatusPending},
		},
	}
	jobs := &fakeDeliveryJobRepo{cancelErr: errBoomService}
	svc := NewNotificationService(notifications, jobs, &fakeTxRunner{
		notifications: notifications,
		jobs:          jobs,
	})

	err := svc.Cancel(context.Background(), "notif-1")
	if err == nil {
		t.Fatalf("expected error")
	}
	if got := notifications.byID["notif-1"].Status; got != domain.NotificationStatusPending {
		t.Fatalf("expected rollback to pending status, got %q", got)
	}
}

func TestNotificationServiceRescheduleRejectsNonPendingNotifications(t *testing.T) {
	notifications := &fakeNotificationRepo{
		byID: map[string]*domain.Notification{
			"notif-1": {ID: "notif-1", Status: domain.NotificationStatusSent},
		},
	}
	svc := NewNotificationService(notifications, &fakeDeliveryJobRepo{})

	err := svc.Reschedule(context.Background(), "notif-1", time.Now().UTC().Add(time.Hour))
	if !errors.Is(err, ErrAlreadySent) {
		t.Fatalf("expected ErrAlreadySent, got %v", err)
	}
}

func TestNotificationServiceRescheduleRollsBackWhenJobRescheduleFails(t *testing.T) {
	originalSendAt := time.Date(2026, 5, 13, 19, 0, 0, 0, time.UTC)
	notifications := &fakeNotificationRepo{
		byID: map[string]*domain.Notification{
			"notif-1": {ID: "notif-1", Status: domain.NotificationStatusPending, SendAt: originalSendAt},
		},
	}
	jobs := &fakeDeliveryJobRepo{rescheduleErr: errBoomService}
	svc := NewNotificationService(notifications, jobs, &fakeTxRunner{
		notifications: notifications,
		jobs:          jobs,
	})

	err := svc.Reschedule(context.Background(), "notif-1", originalSendAt.Add(time.Hour))
	if err == nil {
		t.Fatalf("expected error")
	}
	if got := notifications.byID["notif-1"].SendAt; !got.Equal(originalSendAt) {
		t.Fatalf("expected rollback to original send_at, got %v", got)
	}
}

func TestNotificationServiceEditContentRejectsProcessedJobs(t *testing.T) {
	notifications := &fakeNotificationRepo{
		byID: map[string]*domain.Notification{
			"notif-1": {ID: "notif-1", Status: domain.NotificationStatusPending},
		},
	}
	jobs := &fakeDeliveryJobRepo{
		byNotificationID: map[string][]*domain.DeliveryJob{
			"notif-1": {
				{ID: "job-1", Status: domain.DeliveryJobStatusSent},
			},
		},
	}
	svc := NewNotificationService(notifications, jobs)

	err := svc.EditContent(context.Background(), "notif-1", EditContentParams{
		Content: map[string]any{"subject": "updated"},
	})
	if !errors.Is(err, ErrAlreadySent) {
		t.Fatalf("expected ErrAlreadySent, got %v", err)
	}
}

func TestNotificationServiceEditContentRejectsCancelledJobs(t *testing.T) {
	notifications := &fakeNotificationRepo{
		byID: map[string]*domain.Notification{
			"notif-1": {ID: "notif-1", Status: domain.NotificationStatusPending},
		},
	}
	jobs := &fakeDeliveryJobRepo{
		byNotificationID: map[string][]*domain.DeliveryJob{
			"notif-1": {
				{ID: "job-1", Status: domain.DeliveryJobStatusCancelled},
			},
		},
	}
	svc := NewNotificationService(notifications, jobs)

	err := svc.EditContent(context.Background(), "notif-1", EditContentParams{
		Content: map[string]any{"subject": "updated"},
	})
	if !errors.Is(err, ErrAlreadyCancelled) {
		t.Fatalf("expected ErrAlreadyCancelled, got %v", err)
	}
}

func TestNotificationServiceEditContentRollsBackWhenJobUpdateFails(t *testing.T) {
	originalContent := map[string]any{"subject": "hello"}
	originalMetadata := map[string]any{"campaign": "launch"}
	notifications := &fakeNotificationRepo{
		byID: map[string]*domain.Notification{
			"notif-1": {
				ID:       "notif-1",
				Status:   domain.NotificationStatusPending,
				Content:  cloneMap(originalContent),
				Metadata: cloneMap(originalMetadata),
			},
		},
	}
	jobs := &fakeDeliveryJobRepo{
		updatePayloadErr: errBoomService,
		byNotificationID: map[string][]*domain.DeliveryJob{
			"notif-1": {
				{ID: "job-1", Status: domain.DeliveryJobStatusPending},
			},
		},
	}
	svc := NewNotificationService(notifications, jobs, &fakeTxRunner{
		notifications: notifications,
		jobs:          jobs,
	})

	err := svc.EditContent(context.Background(), "notif-1", EditContentParams{
		Content: map[string]any{"subject": "updated"},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if got := notifications.byID["notif-1"].Content["subject"]; got != "hello" {
		t.Fatalf("expected rollback to original content, got %#v", notifications.byID["notif-1"].Content)
	}
	if got := notifications.byID["notif-1"].Metadata["campaign"]; got != "launch" {
		t.Fatalf("expected rollback to original metadata, got %#v", notifications.byID["notif-1"].Metadata)
	}
}

func TestNotificationServiceIngestReturnsExistingNotificationOnUniqueConflict(t *testing.T) {
	existing := &domain.Notification{
		ID:             "notif-existing",
		IdempotencyKey: "idem-1",
		Status:         domain.NotificationStatusPending,
	}
	notifications := &fakeNotificationRepo{
		createErr: &pgconn.PgError{Code: "23505"},
		getByIdempotencyResponses: []getByIdempotencyResult{
			{notification: nil, err: pgx.ErrNoRows},
			{notification: existing, err: nil},
		},
	}
	svc := NewNotificationService(notifications, &fakeDeliveryJobRepo{})

	got, wasExisting, err := svc.Ingest(context.Background(), IngestParams{
		IdempotencyKey: "idem-1",
		SendAt:         "now",
		Deliveries: []DeliveryTarget{
			{Channel: domain.ChannelEmail, Payload: map[string]any{"subject": "hello"}},
		},
	})
	if err != nil {
		t.Fatalf("Ingest returned error: %v", err)
	}
	if !wasExisting {
		t.Fatalf("expected existing notification")
	}
	if got.ID != existing.ID {
		t.Fatalf("expected %q, got %q", existing.ID, got.ID)
	}
}

func TestNotificationServiceIngestDeletesNotificationWhenJobBatchFails(t *testing.T) {
	notifications := &fakeNotificationRepo{}
	jobs := &fakeDeliveryJobRepo{createBatchErr: errors.New("boom")}
	svc := NewNotificationService(notifications, jobs)

	_, _, err := svc.Ingest(context.Background(), IngestParams{
		IdempotencyKey: "idem-1",
		SendAt:         "now",
		Deliveries: []DeliveryTarget{
			{Channel: domain.ChannelEmail, Payload: map[string]any{"subject": "hello"}},
		},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if notifications.deletedID == "" {
		t.Fatalf("expected notification rollback delete")
	}
}

type fakeNotificationRepo struct {
	byID                      map[string]*domain.Notification
	byIdempotencyKey          map[string]*domain.Notification
	getByIdempotencyResponses []getByIdempotencyResult
	created                   *domain.Notification
	createErr                 error
	createCalls               int
	cancelledID               string
	rescheduledID             string
	rescheduledSend           time.Time
	updatedContentID          string
	updatedContent            map[string]any
	updatedMetadata           map[string]any
	deletedID                 string
}

func (f *fakeNotificationRepo) Create(ctx context.Context, n *domain.Notification) (*domain.Notification, error) {
	f.createCalls++
	if f.createErr != nil {
		return nil, f.createErr
	}
	cpy := *n
	f.created = &cpy
	return &cpy, nil
}

func (f *fakeNotificationRepo) GetByID(ctx context.Context, id string) (*domain.Notification, error) {
	if n, ok := f.byID[id]; ok {
		cpy := *n
		return &cpy, nil
	}
	return nil, pgx.ErrNoRows
}

func (f *fakeNotificationRepo) GetByIdempotencyKey(ctx context.Context, key string) (*domain.Notification, error) {
	if len(f.getByIdempotencyResponses) > 0 {
		result := f.getByIdempotencyResponses[0]
		f.getByIdempotencyResponses = f.getByIdempotencyResponses[1:]
		return result.notification, result.err
	}
	if n, ok := f.byIdempotencyKey[key]; ok {
		cpy := *n
		return &cpy, nil
	}
	return nil, pgx.ErrNoRows
}

func (f *fakeNotificationRepo) Cancel(ctx context.Context, id string) (*domain.Notification, error) {
	f.cancelledID = id
	n := *f.byID[id]
	n.Status = domain.NotificationStatusCancelled
	f.byID[id] = &n
	return &n, nil
}

func (f *fakeNotificationRepo) Reschedule(ctx context.Context, id string, sendAt time.Time) (*domain.Notification, error) {
	f.rescheduledID = id
	f.rescheduledSend = sendAt
	n := *f.byID[id]
	n.SendAt = sendAt
	f.byID[id] = &n
	return &n, nil
}

func (f *fakeNotificationRepo) UpdateContent(ctx context.Context, id string, content, metadata map[string]any) (*domain.Notification, error) {
	f.updatedContentID = id
	f.updatedContent = content
	f.updatedMetadata = metadata
	n := *f.byID[id]
	n.Content = cloneMap(content)
	n.Metadata = cloneMap(metadata)
	f.byID[id] = &n
	return &n, nil
}

func (f *fakeNotificationRepo) Delete(ctx context.Context, id string) error {
	f.deletedID = id
	return nil
}

type fakeDeliveryJobRepo struct {
	created            []*domain.DeliveryJob
	createBatchCalls   int
	createBatchErr     error
	cancelledNotifID   string
	cancelErr          error
	rescheduledNotifID string
	rescheduledSend    time.Time
	rescheduleErr      error
	updatedNotifID     string
	updatedPayload     map[string]any
	updatePayloadErr   error
	byNotificationID   map[string][]*domain.DeliveryJob
}

func (f *fakeDeliveryJobRepo) CreateBatch(ctx context.Context, jobs []*domain.DeliveryJob) error {
	f.createBatchCalls++
	if f.createBatchErr != nil {
		return f.createBatchErr
	}
	f.created = append(f.created, jobs...)
	return nil
}

func (f *fakeDeliveryJobRepo) CancelForNotification(ctx context.Context, notificationID string) ([]*domain.DeliveryJob, error) {
	f.cancelledNotifID = notificationID
	if f.cancelErr != nil {
		return nil, f.cancelErr
	}
	return nil, nil
}

func (f *fakeDeliveryJobRepo) RescheduleForNotification(ctx context.Context, notificationID string, sendAt time.Time) error {
	f.rescheduledNotifID = notificationID
	f.rescheduledSend = sendAt
	if f.rescheduleErr != nil {
		return f.rescheduleErr
	}
	return nil
}

func (f *fakeDeliveryJobRepo) UpdatePayloadForNotification(ctx context.Context, notificationID string, payload map[string]any) error {
	f.updatedNotifID = notificationID
	f.updatedPayload = payload
	if f.updatePayloadErr != nil {
		return f.updatePayloadErr
	}
	return nil
}

func (f *fakeDeliveryJobRepo) ListByNotificationID(ctx context.Context, notificationID string) ([]*domain.DeliveryJob, error) {
	rows := f.byNotificationID[notificationID]
	out := make([]*domain.DeliveryJob, len(rows))
	for i, row := range rows {
		cpy := *row
		out[i] = &cpy
	}
	return out, nil
}

type getByIdempotencyResult struct {
	notification *domain.Notification
	err          error
}

type fakeTxRunner struct {
	notifications *fakeNotificationRepo
	jobs          *fakeDeliveryJobRepo
}

func (f *fakeTxRunner) RunInTx(ctx context.Context, fn func(context.Context) error) error {
	notificationSnapshot := snapshotNotifications(f.notifications.byID)
	jobSnapshot := snapshotDeliveryJobs(f.jobs.byNotificationID)

	err := fn(ctx)
	if err != nil {
		f.notifications.byID = notificationSnapshot
		f.jobs.byNotificationID = jobSnapshot
	}
	return err
}

func snapshotNotifications(src map[string]*domain.Notification) map[string]*domain.Notification {
	if src == nil {
		return nil
	}
	dst := make(map[string]*domain.Notification, len(src))
	for id, notification := range src {
		cpy := *notification
		cpy.Content = cloneMap(notification.Content)
		cpy.Metadata = cloneMap(notification.Metadata)
		dst[id] = &cpy
	}
	return dst
}

func snapshotDeliveryJobs(src map[string][]*domain.DeliveryJob) map[string][]*domain.DeliveryJob {
	if src == nil {
		return nil
	}
	dst := make(map[string][]*domain.DeliveryJob, len(src))
	for id, jobs := range src {
		copied := make([]*domain.DeliveryJob, len(jobs))
		for i, job := range jobs {
			cpy := *job
			cpy.Payload = cloneMap(job.Payload)
			copied[i] = &cpy
		}
		dst[id] = copied
	}
	return dst
}
