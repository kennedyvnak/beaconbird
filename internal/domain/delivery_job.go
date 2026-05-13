package domain

import "time"

type DeliveryJobStatus string

const (
	DeliveryJobStatusPending    DeliveryJobStatus = "pending"
	DeliveryJobStatusProcessing DeliveryJobStatus = "processing"
	DeliveryJobStatusSent       DeliveryJobStatus = "sent"
	DeliveryJobStatusFailed     DeliveryJobStatus = "failed"
	DeliveryJobStatusRetrying   DeliveryJobStatus = "retrying"
	DeliveryJobStatusDeadLetter DeliveryJobStatus = "dead_letter"
	DeliveryJobStatusCancelled  DeliveryJobStatus = "cancelled"
)

type DeliveryJob struct {
	ID             string
	NotificationID string
	ProviderID     *string
	Channel        Channel
	Status         DeliveryJobStatus
	Payload        map[string]any
	SendAt         time.Time
	LockedAt       *time.Time
	LockedBy       *string
	HeartbeatAt    *time.Time
	RetryCount     int32
	MaxRetries     int32
	NextRetryAt    *time.Time
	LastError      *string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
