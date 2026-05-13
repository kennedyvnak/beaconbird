package repository

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kennedyvnak/beaconbird/internal/repository/sqlc"
)

func TestToDeliveryJobMapsIsTest(t *testing.T) {
	now := time.Date(2026, 5, 13, 20, 0, 0, 0, time.UTC)

	job := toDeliveryJob(sqlc.DeliveryJob{
		ID:             "job-1",
		NotificationID: "notif-1",
		Channel:        "push",
		Status:         "pending",
		Payload:        []byte(`{"title":"hello"}`),
		SendAt:         pgtype.Timestamptz{Time: now, Valid: true},
		MaxRetries:     3,
		IsTest:         true,
		CreatedAt:      pgtype.Timestamptz{Time: now, Valid: true},
		UpdatedAt:      pgtype.Timestamptz{Time: now, Valid: true},
	})

	if !job.IsTest {
		t.Fatalf("expected IsTest to be mapped")
	}
}
