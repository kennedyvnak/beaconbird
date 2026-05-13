package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/kennedyvnak/beaconbird/internal/api/dto"
	"github.com/kennedyvnak/beaconbird/internal/domain"
	"github.com/kennedyvnak/beaconbird/internal/service"
)

const (
	defaultListLimit int32 = 100
	maxListLimit     int32 = 500
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, dto.ErrorResponse{Error: err.Error()})
}

func writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrNotFound), errors.Is(err, pgx.ErrNoRows):
		writeError(w, http.StatusNotFound, err)
	case errors.Is(err, service.ErrAlreadyCancelled), errors.Is(err, service.ErrAlreadySent):
		writeError(w, http.StatusConflict, err)
	case errors.Is(err, service.ErrInvalidSendAt):
		writeError(w, http.StatusBadRequest, err)
	case errors.Is(err, service.ErrSendAtInPast):
		writeError(w, http.StatusUnprocessableEntity, err)
	default:
		writeError(w, http.StatusInternalServerError, err)
	}
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func formatTimePtr(t *time.Time) *string {
	if t == nil || t.IsZero() {
		return nil
	}
	s := t.UTC().Format(time.RFC3339)
	return &s
}

func queryTime(raw string, fallback time.Time) time.Time {
	if raw == "" {
		return fallback
	}
	v, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return fallback
	}
	return v.UTC()
}

func queryInt32(raw string, fallback int32) int32 {
	if raw == "" {
		return fallback
	}
	n, err := strconv.ParseInt(raw, 10, 32)
	if err != nil || n < 0 {
		return fallback
	}
	return int32(n)
}

func queryLimit(raw string) int32 {
	limit := queryInt32(raw, defaultListLimit)
	if limit <= 0 {
		return defaultListLimit
	}
	if limit > maxListLimit {
		return maxListLimit
	}
	return limit
}

func toDeliveryJobResponses(jobs []*domain.DeliveryJob, attemptsByJob map[string][]*domain.DeliveryAttempt) []dto.DeliveryJobResponse {
	resp := make([]dto.DeliveryJobResponse, 0, len(jobs))
	for _, job := range jobs {
		row := dto.DeliveryJobResponse{
			ID:             job.ID,
			NotificationID: job.NotificationID,
			Channel:        string(job.Channel),
			Status:         string(job.Status),
			Payload:        job.Payload,
			RetryCount:     job.RetryCount,
			MaxRetries:     job.MaxRetries,
			SendAt:         formatTime(job.SendAt),
			NextRetryAt:    formatTimePtr(job.NextRetryAt),
			LastError:      job.LastError,
			CreatedAt:      formatTime(job.CreatedAt),
			UpdatedAt:      formatTime(job.UpdatedAt),
		}

		for _, attempt := range attemptsByJob[job.ID] {
			row.Attempts = append(row.Attempts, dto.DeliveryAttemptResponse{
				ID:            attempt.ID,
				DeliveryJobID: attempt.DeliveryJobID,
				AttemptNumber: attempt.AttemptNumber,
				ProviderName:  attempt.ProviderName,
				Status:        string(attempt.Status),
				ErrorMessage:  attempt.ErrorMessage,
				DurationMS:    attempt.DurationMS,
				AttemptedAt:   formatTime(attempt.AttemptedAt),
				Request:       attempt.RequestPayload,
				Response:      attempt.ResponsePayload,
			})
		}

		resp = append(resp, row)
	}
	return resp
}
