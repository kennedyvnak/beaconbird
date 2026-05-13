package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/kennedyvnak/beaconbird/internal/domain"
	"github.com/kennedyvnak/beaconbird/internal/service"
)

func TestNotificationHandlerCreateReturnsCreatedNotification(t *testing.T) {
	svc := &fakeNotificationService{
		ingestNotification: &domain.Notification{
			ID:             "notif-1",
			IdempotencyKey: "idem-1",
			Status:         domain.NotificationStatusPending,
			Tag:            "marketing",
			Content:        map[string]any{"subject": "hello"},
			Metadata:       map[string]any{"campaign": "launch"},
			SendAt:         time.Date(2026, 5, 13, 19, 0, 0, 0, time.UTC),
			CreatedAt:      time.Date(2026, 5, 13, 19, 0, 0, 0, time.UTC),
			UpdatedAt:      time.Date(2026, 5, 13, 19, 0, 0, 0, time.UTC),
		},
	}
	jobRepo := &fakeDeliveryJobReader{
		byNotificationID: map[string][]*domain.DeliveryJob{
			"notif-1": {
				{
					ID:             "job-1",
					NotificationID: "notif-1",
					Channel:        domain.ChannelEmail,
					Status:         domain.DeliveryJobStatusPending,
					Payload:        map[string]any{"subject": "hello"},
					SendAt:         time.Date(2026, 5, 13, 19, 0, 0, 0, time.UTC),
					CreatedAt:      time.Date(2026, 5, 13, 19, 0, 0, 0, time.UTC),
				},
			},
		},
	}
	h := NewNotificationHandler(svc, &fakeNotificationReader{}, jobRepo)

	body := []byte(`{"deliveries":[{"channel":"email","payload":{"subject":"hello"}}],"send_at":"now","idempotency_key":"idem-1","tag":"marketing","metadata":{"campaign":"launch"}}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/notifications", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["id"] != "notif-1" {
		t.Fatalf("expected notification ID in response, got %v", payload["id"])
	}
	if svc.ingestParams.IdempotencyKey != "idem-1" {
		t.Fatalf("expected service to receive idempotency key")
	}
}

func TestNotificationHandlerCreateReturnsBadRequestForInvalidSendAt(t *testing.T) {
	h := NewNotificationHandler(
		&fakeNotificationService{ingestErr: service.ErrInvalidSendAt},
		&fakeNotificationReader{},
		&fakeDeliveryJobReader{},
	)

	body := []byte(`{"deliveries":[{"channel":"email","payload":{"subject":"hello"}}],"send_at":"not-a-timestamp","idempotency_key":"idem-1"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/notifications", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestNotificationHandlerPatchRejectsInvalidAction(t *testing.T) {
	h := NewNotificationHandler(&fakeNotificationService{}, &fakeNotificationReader{}, &fakeDeliveryJobReader{})

	req := httptest.NewRequest(http.MethodPatch, "/v1/notifications/notif-1", bytes.NewReader([]byte(`{"action":"pause"}`)))
	rec := httptest.NewRecorder()
	router := chi.NewRouter()
	router.Route("/v1/notifications", func(r chi.Router) {
		r.Patch("/{id}", h.Patch)
	})

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestNotificationHandlerGetReturnsServerErrorWhenJobLookupFails(t *testing.T) {
	h := NewNotificationHandler(
		&fakeNotificationService{},
		&fakeNotificationReader{
			byID: map[string]*domain.Notification{
				"notif-1": {ID: "notif-1", Status: domain.NotificationStatusPending},
			},
		},
		&fakeDeliveryJobReader{err: errBoom},
	)

	req := httptest.NewRequest(http.MethodGet, "/v1/notifications/notif-1", nil)
	rec := httptest.NewRecorder()
	router := chi.NewRouter()
	router.Route("/v1/notifications", func(r chi.Router) {
		r.Get("/{id}", h.Get)
	})

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func TestNotificationHandlerListDoesNotPostFilterChannelMatches(t *testing.T) {
	notifications := &fakeNotificationReader{
		list: []*domain.Notification{
			{ID: "notif-1", Status: domain.NotificationStatusPending},
		},
	}
	jobs := &fakeDeliveryJobReader{}
	h := NewNotificationHandler(&fakeNotificationService{}, notifications, jobs)

	req := httptest.NewRequest(http.MethodGet, "/v1/notifications?channel=push", nil)
	rec := httptest.NewRecorder()

	h.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var payload []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(payload))
	}
	if notifications.listParams.Channel != "push" {
		t.Fatalf("expected channel filter to be forwarded, got %q", notifications.listParams.Channel)
	}
}

func TestDeliveryJobHandlerGetIncludesAttempts(t *testing.T) {
	jobs := &fakeDeliveryJobReader{
		byID: map[string]*domain.DeliveryJob{
			"job-1": {
				ID:             "job-1",
				NotificationID: "notif-1",
				Channel:        domain.ChannelEmail,
				Status:         domain.DeliveryJobStatusPending,
				Payload:        map[string]any{"subject": "hello"},
				SendAt:         time.Date(2026, 5, 13, 19, 0, 0, 0, time.UTC),
				CreatedAt:      time.Date(2026, 5, 13, 19, 0, 0, 0, time.UTC),
			},
		},
	}
	attempts := &fakeDeliveryAttemptReader{
		byJobID: map[string][]*domain.DeliveryAttempt{
			"job-1": {
				{
					ID:            "attempt-1",
					DeliveryJobID: "job-1",
					AttemptNumber: 1,
					Status:        domain.DeliveryJobStatusPending,
					AttemptedAt:   time.Date(2026, 5, 13, 19, 0, 0, 0, time.UTC),
				},
			},
		},
	}
	h := NewDeliveryJobHandler(jobs, attempts)

	req := httptest.NewRequest(http.MethodGet, "/v1/delivery-jobs/job-1", nil)
	rec := httptest.NewRecorder()
	router := chi.NewRouter()
	router.Route("/v1/delivery-jobs", func(r chi.Router) {
		r.Get("/{id}", h.Get)
	})

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	attemptList, ok := payload["attempts"].([]any)
	if !ok || len(attemptList) != 1 {
		t.Fatalf("expected 1 attempt, got %#v", payload["attempts"])
	}
}

func TestAPIKeyHandlerCreateReturnsRawKey(t *testing.T) {
	repo := &fakeAPIKeyRepo{
		created: &domain.APIKey{
			ID:        "key-1",
			Name:      "test key",
			Mode:      domain.APIKeyModeTest,
			CreatedAt: time.Date(2026, 5, 13, 19, 0, 0, 0, time.UTC),
		},
	}
	h := NewAPIKeyHandler(repo)
	h.keyGenerator = func(mode string) (string, string, error) {
		return "bb_test_deadbeefdeadbeefdeadbeefdeadbeef", "hash-value", nil
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/api-keys", bytes.NewReader([]byte(`{"name":"test key","mode":"test"}`)))
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["raw_key"] != "bb_test_deadbeefdeadbeefdeadbeefdeadbeef" {
		t.Fatalf("expected raw_key in response, got %#v", payload["raw_key"])
	}
	if repo.createInput == nil || repo.createInput.KeyHash != "hash-value" {
		t.Fatalf("expected hashed key to be stored")
	}
}

type fakeNotificationService struct {
	ingestNotification *domain.Notification
	ingestExisting     bool
	ingestErr          error
	ingestParams       service.IngestParams
	cancelErr          error
	rescheduleErr      error
	editErr            error
}

func (f *fakeNotificationService) Ingest(ctx context.Context, params service.IngestParams) (*domain.Notification, bool, error) {
	f.ingestParams = params
	return f.ingestNotification, f.ingestExisting, f.ingestErr
}

func (f *fakeNotificationService) Cancel(ctx context.Context, id string) error {
	return f.cancelErr
}

func (f *fakeNotificationService) Reschedule(ctx context.Context, id string, sendAt time.Time) error {
	return f.rescheduleErr
}

func (f *fakeNotificationService) EditContent(ctx context.Context, id string, params service.EditContentParams) error {
	return f.editErr
}

type fakeNotificationReader struct {
	byID       map[string]*domain.Notification
	list       []*domain.Notification
	listParams NotificationListParams
	err        error
}

func (f *fakeNotificationReader) GetByID(ctx context.Context, id string) (*domain.Notification, error) {
	if f.err != nil {
		return nil, f.err
	}
	if n, ok := f.byID[id]; ok {
		return n, nil
	}
	return nil, service.ErrNotFound
}

func (f *fakeNotificationReader) List(ctx context.Context, params NotificationListParams) ([]*domain.Notification, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.listParams = params
	return f.list, nil
}

type fakeDeliveryJobReader struct {
	byID             map[string]*domain.DeliveryJob
	byNotificationID map[string][]*domain.DeliveryJob
	list             []*domain.DeliveryJob
	err              error
}

func (f *fakeDeliveryJobReader) GetByID(ctx context.Context, id string) (*domain.DeliveryJob, error) {
	if f.err != nil {
		return nil, f.err
	}
	if job, ok := f.byID[id]; ok {
		return job, nil
	}
	return nil, service.ErrNotFound
}

func (f *fakeDeliveryJobReader) List(ctx context.Context, params DeliveryJobListParams) ([]*domain.DeliveryJob, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.list, nil
}

func (f *fakeDeliveryJobReader) ListByNotificationID(ctx context.Context, notificationID string) ([]*domain.DeliveryJob, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.byNotificationID[notificationID], nil
}

type fakeDeliveryAttemptReader struct {
	byJobID map[string][]*domain.DeliveryAttempt
	err     error
}

func (f *fakeDeliveryAttemptReader) ListByJob(ctx context.Context, jobID string) ([]*domain.DeliveryAttempt, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.byJobID[jobID], nil
}

type fakeAPIKeyRepo struct {
	created     *domain.APIKey
	createInput *domain.APIKey
	list        []*domain.APIKey
	err         error
}

func (f *fakeAPIKeyRepo) Create(ctx context.Context, key *domain.APIKey) (*domain.APIKey, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.createInput = key
	return f.created, nil
}

func (f *fakeAPIKeyRepo) List(ctx context.Context) ([]*domain.APIKey, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.list, nil
}

func (f *fakeAPIKeyRepo) Delete(ctx context.Context, id string) error {
	return f.err
}

var errBoom = errors.New("boom")
