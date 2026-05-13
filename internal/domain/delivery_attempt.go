package domain

import "time"

type DeliveryAttempt struct {
	ID              string
	DeliveryJobID   string
	AttemptNumber   int32
	ProviderName    string
	Status          DeliveryJobStatus
	RequestPayload  map[string]any
	ResponsePayload map[string]any
	ErrorMessage    *string
	DurationMS      *int32
	AttemptedAt     time.Time
}
