package dto

type CreateNotificationRequest struct {
	Deliveries     []DeliveryTarget `json:"deliveries"`
	SendAt         string           `json:"send_at"`
	IdempotencyKey string           `json:"idempotency_key"`
	Tag            string           `json:"tag"`
	Metadata       map[string]any   `json:"metadata"`
}

type DeliveryTarget struct {
	Channel string         `json:"channel"`
	Payload map[string]any `json:"payload"`
}

type BulkCreateNotificationRequest struct {
	Notifications []CreateNotificationRequest `json:"notifications"`
}

type PatchNotificationRequest struct {
	Action string  `json:"action"`
	SendAt *string `json:"send_at,omitempty"`
}

type EditNotificationRequest struct {
	Content  map[string]any `json:"content"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type CreateAPIKeyRequest struct {
	Name string `json:"name"`
	Mode string `json:"mode"`
}

type NotificationResponse struct {
	ID             string                `json:"id"`
	Status         string                `json:"status"`
	IdempotencyKey string                `json:"idempotency_key"`
	Tag            string                `json:"tag"`
	Content        map[string]any        `json:"content,omitempty"`
	Metadata       map[string]any        `json:"metadata,omitempty"`
	SendAt         string                `json:"send_at"`
	CreatedAt      string                `json:"created_at"`
	UpdatedAt      string                `json:"updated_at"`
	CancelledAt    *string               `json:"cancelled_at,omitempty"`
	DeliveryJobs   []DeliveryJobResponse `json:"delivery_jobs,omitempty"`
}

type DeliveryJobResponse struct {
	ID             string                    `json:"id"`
	NotificationID string                    `json:"notification_id"`
	Channel        string                    `json:"channel"`
	Status         string                    `json:"status"`
	Payload        map[string]any            `json:"payload,omitempty"`
	RetryCount     int32                     `json:"retry_count"`
	MaxRetries     int32                     `json:"max_retries"`
	SendAt         string                    `json:"send_at"`
	NextRetryAt    *string                   `json:"next_retry_at,omitempty"`
	LastError      *string                   `json:"last_error,omitempty"`
	CreatedAt      string                    `json:"created_at"`
	UpdatedAt      string                    `json:"updated_at"`
	Attempts       []DeliveryAttemptResponse `json:"attempts,omitempty"`
}

type DeliveryAttemptResponse struct {
	ID            string         `json:"id"`
	DeliveryJobID string         `json:"delivery_job_id"`
	AttemptNumber int32          `json:"attempt_number"`
	ProviderName  string         `json:"provider_name"`
	Status        string         `json:"status"`
	ErrorMessage  *string        `json:"error_message,omitempty"`
	DurationMS    *int32         `json:"duration_ms,omitempty"`
	AttemptedAt   string         `json:"attempted_at"`
	Request       map[string]any `json:"request,omitempty"`
	Response      map[string]any `json:"response,omitempty"`
}

type BulkCreateResult struct {
	Results []BulkItemResult `json:"results"`
}

type BulkItemResult struct {
	IdempotencyKey string                `json:"idempotency_key"`
	Status         string                `json:"status"`
	Notification   *NotificationResponse `json:"notification,omitempty"`
	Error          *string               `json:"error,omitempty"`
}

type APIKeyCreateResponse struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Mode       string  `json:"mode"`
	CreatedAt  string  `json:"created_at"`
	LastUsedAt *string `json:"last_used_at,omitempty"`
	RawKey     string  `json:"raw_key"`
}

type APIKeyResponse struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Mode       string  `json:"mode"`
	CreatedAt  string  `json:"created_at"`
	LastUsedAt *string `json:"last_used_at,omitempty"`
	IsRevoked  bool    `json:"is_revoked"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}
