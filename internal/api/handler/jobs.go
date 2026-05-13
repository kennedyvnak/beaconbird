package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/kennedyvnak/beaconbird/internal/domain"
)

type DeliveryJobListParams struct {
	Status         string
	Channel        string
	NotificationID string
	From           time.Time
	To             time.Time
	Offset         int32
	Limit          int32
}

type deliveryJobReader interface {
	GetByID(ctx context.Context, id string) (*domain.DeliveryJob, error)
	List(ctx context.Context, params DeliveryJobListParams) ([]*domain.DeliveryJob, error)
}

type deliveryAttemptReader interface {
	ListByJob(ctx context.Context, jobID string) ([]*domain.DeliveryAttempt, error)
}

type DeliveryJobHandler struct {
	jobs     deliveryJobReader
	attempts deliveryAttemptReader
}

func NewDeliveryJobHandler(jobs deliveryJobReader, attempts deliveryAttemptReader) *DeliveryJobHandler {
	return &DeliveryJobHandler{jobs: jobs, attempts: attempts}
}

func (h *DeliveryJobHandler) Get(w http.ResponseWriter, r *http.Request) {
	job, err := h.jobs.GetByID(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	attempts, err := h.attempts.ListByJob(r.Context(), job.ID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	rows := toDeliveryJobResponses([]*domain.DeliveryJob{job}, map[string][]*domain.DeliveryAttempt{job.ID: attempts})
	writeJSON(w, http.StatusOK, rows[0])
}

func (h *DeliveryJobHandler) List(w http.ResponseWriter, r *http.Request) {
	params := DeliveryJobListParams{
		Status:         r.URL.Query().Get("status"),
		Channel:        r.URL.Query().Get("channel"),
		NotificationID: r.URL.Query().Get("notification_id"),
		From:           queryTime(r.URL.Query().Get("from"), time.Unix(0, 0).UTC()),
		To:             queryTime(r.URL.Query().Get("to"), time.Date(9999, 1, 1, 0, 0, 0, 0, time.UTC)),
		Offset:         queryInt32(r.URL.Query().Get("offset"), 0),
		Limit:          queryLimit(r.URL.Query().Get("limit")),
	}

	jobs, err := h.jobs.List(r.Context(), params)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toDeliveryJobResponses(jobs, nil))
}
