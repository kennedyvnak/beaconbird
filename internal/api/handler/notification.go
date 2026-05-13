package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/kennedyvnak/beaconbird/internal/api/dto"
	"github.com/kennedyvnak/beaconbird/internal/api/middleware"
	"github.com/kennedyvnak/beaconbird/internal/domain"
	"github.com/kennedyvnak/beaconbird/internal/service"
)

type notificationWriter interface {
	Ingest(ctx context.Context, params service.IngestParams) (*domain.Notification, bool, error)
	Cancel(ctx context.Context, id string) error
	Reschedule(ctx context.Context, id string, sendAt time.Time) error
	EditContent(ctx context.Context, id string, params service.EditContentParams) error
}

type NotificationListParams struct {
	Status  string
	Tag     string
	Channel string
	From    time.Time
	To      time.Time
	Offset  int32
	Limit   int32
}

type notificationReader interface {
	GetByID(ctx context.Context, id string) (*domain.Notification, error)
	List(ctx context.Context, params NotificationListParams) ([]*domain.Notification, error)
}

type notificationJobReader interface {
	ListByNotificationID(ctx context.Context, notificationID string) ([]*domain.DeliveryJob, error)
}

type NotificationHandler struct {
	service       notificationWriter
	notifications notificationReader
	jobs          notificationJobReader
}

func NewNotificationHandler(service notificationWriter, notifications notificationReader, jobs notificationJobReader) *NotificationHandler {
	return &NotificationHandler{
		service:       service,
		notifications: notifications,
		jobs:          jobs,
	}
}

func (h *NotificationHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateNotificationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	params, err := toIngestParams(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if key := middleware.APIKeyFromContext(r.Context()); key != nil {
		params.IsTest = key.Mode == domain.APIKeyModeTest
	}

	notification, wasExisting, err := h.service.Ingest(r.Context(), params)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	status := http.StatusCreated
	if wasExisting {
		status = http.StatusOK
	}
	resp, err := h.notificationResponse(r.Context(), notification, true)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, status, resp)
}

func (h *NotificationHandler) BulkCreate(w http.ResponseWriter, r *http.Request) {
	var req dto.BulkCreateNotificationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	results := make([]dto.BulkItemResult, 0, len(req.Notifications))
	for _, item := range req.Notifications {
		params, err := toIngestParams(item)
		if err != nil {
			msg := err.Error()
			results = append(results, dto.BulkItemResult{
				IdempotencyKey: item.IdempotencyKey,
				Status:         "error",
				Error:          &msg,
			})
			continue
		}
		if key := middleware.APIKeyFromContext(r.Context()); key != nil {
			params.IsTest = key.Mode == domain.APIKeyModeTest
		}

		notification, wasExisting, err := h.service.Ingest(r.Context(), params)
		if err != nil {
			msg := err.Error()
			results = append(results, dto.BulkItemResult{
				IdempotencyKey: item.IdempotencyKey,
				Status:         "error",
				Error:          &msg,
			})
			continue
		}

		status := "created"
		if wasExisting {
			status = "existing"
		}
		resp, err := h.notificationResponse(r.Context(), notification, true)
		if err != nil {
			msg := err.Error()
			results = append(results, dto.BulkItemResult{
				IdempotencyKey: item.IdempotencyKey,
				Status:         "error",
				Error:          &msg,
			})
			continue
		}
		results = append(results, dto.BulkItemResult{
			IdempotencyKey: item.IdempotencyKey,
			Status:         status,
			Notification:   &resp,
		})
	}

	writeJSON(w, http.StatusMultiStatus, dto.BulkCreateResult{Results: results})
}

func (h *NotificationHandler) Get(w http.ResponseWriter, r *http.Request) {
	notification, err := h.notifications.GetByID(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	resp, err := h.notificationResponse(r.Context(), notification, true)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *NotificationHandler) Patch(w http.ResponseWriter, r *http.Request) {
	var req dto.PatchNotificationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	id := chi.URLParam(r, "id")
	switch req.Action {
	case "cancel":
		if err := h.service.Cancel(r.Context(), id); err != nil {
			writeServiceError(w, err)
			return
		}
	case "reschedule":
		if req.SendAt == nil {
			writeError(w, http.StatusBadRequest, errors.New("send_at is required"))
			return
		}
		sendAt, err := time.Parse(time.RFC3339, *req.SendAt)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if err := h.service.Reschedule(r.Context(), id, sendAt.UTC()); err != nil {
			writeServiceError(w, err)
			return
		}
	default:
		writeError(w, http.StatusBadRequest, errors.New("invalid action"))
		return
	}

	notification, err := h.notifications.GetByID(r.Context(), id)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	resp, err := h.notificationResponse(r.Context(), notification, true)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *NotificationHandler) Edit(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req dto.EditNotificationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if len(req.Content) == 0 {
		writeError(w, http.StatusBadRequest, errors.New("content is required"))
		return
	}

	if err := h.service.EditContent(r.Context(), id, service.EditContentParams{
		Content:  req.Content,
		Metadata: req.Metadata,
	}); err != nil {
		writeServiceError(w, err)
		return
	}

	notification, err := h.notifications.GetByID(r.Context(), id)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	resp, err := h.notificationResponse(r.Context(), notification, true)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *NotificationHandler) List(w http.ResponseWriter, r *http.Request) {
	params := NotificationListParams{
		Status:  r.URL.Query().Get("status"),
		Tag:     r.URL.Query().Get("tag"),
		Channel: r.URL.Query().Get("channel"),
		From:    queryTime(r.URL.Query().Get("from"), time.Unix(0, 0).UTC()),
		To:      queryTime(r.URL.Query().Get("to"), time.Date(9999, 1, 1, 0, 0, 0, 0, time.UTC)),
		Offset:  queryInt32(r.URL.Query().Get("offset"), 0),
		Limit:   queryLimit(r.URL.Query().Get("limit")),
	}

	notifications, err := h.notifications.List(r.Context(), params)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	resp := make([]dto.NotificationResponse, 0, len(notifications))
	for _, notification := range notifications {
		row, err := h.notificationResponse(r.Context(), notification, true)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		resp = append(resp, row)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *NotificationHandler) notificationResponse(ctx context.Context, notification *domain.Notification, includeJobs bool) (dto.NotificationResponse, error) {
	resp := dto.NotificationResponse{
		ID:             notification.ID,
		Status:         string(notification.Status),
		IdempotencyKey: notification.IdempotencyKey,
		Tag:            notification.Tag,
		Content:        notification.Content,
		Metadata:       notification.Metadata,
		SendAt:         formatTime(notification.SendAt),
		CreatedAt:      formatTime(notification.CreatedAt),
		UpdatedAt:      formatTime(notification.UpdatedAt),
		CancelledAt:    formatTimePtr(notification.CancelledAt),
	}

	if includeJobs {
		jobs, err := h.jobs.ListByNotificationID(ctx, notification.ID)
		if err != nil {
			return dto.NotificationResponse{}, err
		}
		resp.DeliveryJobs = toDeliveryJobResponses(jobs, nil)
	}
	return resp, nil
}

func toIngestParams(req dto.CreateNotificationRequest) (service.IngestParams, error) {
	if req.IdempotencyKey == "" {
		return service.IngestParams{}, errors.New("idempotency_key is required")
	}
	if req.SendAt == "" {
		return service.IngestParams{}, errors.New("send_at is required")
	}
	if len(req.Deliveries) == 0 {
		return service.IngestParams{}, errors.New("deliveries is required")
	}

	deliveries := make([]service.DeliveryTarget, 0, len(req.Deliveries))
	for _, delivery := range req.Deliveries {
		switch delivery.Channel {
		case string(domain.ChannelEmail), string(domain.ChannelPush):
		default:
			return service.IngestParams{}, errors.New("invalid delivery channel")
		}
		deliveries = append(deliveries, service.DeliveryTarget{
			Channel: domain.Channel(delivery.Channel),
			Payload: delivery.Payload,
		})
	}

	return service.IngestParams{
		Deliveries:     deliveries,
		SendAt:         req.SendAt,
		IdempotencyKey: req.IdempotencyKey,
		Tag:            req.Tag,
		Metadata:       req.Metadata,
	}, nil
}
