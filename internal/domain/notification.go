package domain

import "time"

type NotificationStatus string

const (
	NotificationStatusPending    NotificationStatus = "pending"
	NotificationStatusProcessing NotificationStatus = "processing"
	NotificationStatusSent       NotificationStatus = "sent"
	NotificationStatusFailed     NotificationStatus = "failed"
	NotificationStatusCancelled  NotificationStatus = "cancelled"
)

type Notification struct {
	ID             string
	IdempotencyKey string
	Status         NotificationStatus
	Tag            string
	Content        map[string]any
	Metadata       map[string]any
	SendAt         time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
	CancelledAt    *time.Time
}
