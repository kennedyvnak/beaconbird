package domain

import (
	"testing"
	"time"
)

func TestNotificationCarriesCoreLifecycleFields(t *testing.T) {
	now := time.Now().UTC()
	id := "bb-notification-1"

	notification := Notification{
		ID:             id,
		IdempotencyKey: "welcome-email-123",
		Status:         NotificationStatusPending,
		Tag:            "welcome",
		Content: map[string]any{
			"subject": "Hello",
		},
		Metadata: map[string]any{
			"tenant": "acme",
		},
		SendAt:    now,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if notification.ID != id {
		t.Fatalf("expected ID %s, got %s", id, notification.ID)
	}

	if notification.Status != NotificationStatusPending {
		t.Fatalf("expected status %q, got %q", NotificationStatusPending, notification.Status)
	}

	if notification.Content["subject"] != "Hello" {
		t.Fatalf("expected content subject to be preserved")
	}
}

func TestDeliveryJobModelsRetryAndLockingState(t *testing.T) {
	now := time.Now().UTC()
	nextRetry := now.Add(30 * time.Second)
	lockedBy := "0a8f8479-d0c4-44a2-882f-8d8dd6f246f1"
	jobID := "bb-job-1"

	job := DeliveryJob{
		ID:             jobID,
		NotificationID: "bb-notification-1",
		ProviderID:     stringPtr("bb-provider-1"),
		Channel:        ChannelEmail,
		Status:         DeliveryJobStatusRetrying,
		Payload: map[string]any{
			"to": "user@example.com",
		},
		SendAt:      now,
		LockedAt:    &now,
		LockedBy:    &lockedBy,
		HeartbeatAt: &now,
		RetryCount:  1,
		MaxRetries:  3,
		NextRetryAt: &nextRetry,
		LastError:   stringPtr("temporary failure"),
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if job.Status != DeliveryJobStatusRetrying {
		t.Fatalf("expected retrying status, got %q", job.Status)
	}

	if job.LockedBy == nil || *job.LockedBy != lockedBy {
		t.Fatalf("expected lock owner %s, got %v", lockedBy, job.LockedBy)
	}

	if job.Payload["to"] != "user@example.com" {
		t.Fatalf("expected payload recipient to be preserved")
	}
}

func TestProviderAndAPIKeyEnumsExposeSupportedValues(t *testing.T) {
	if ChannelEmail != "email" {
		t.Fatalf("expected ChannelEmail to be %q, got %q", "email", ChannelEmail)
	}

	if ChannelPush != "push" {
		t.Fatalf("expected ChannelPush to be %q, got %q", "push", ChannelPush)
	}

	if APIKeyModeLive != "live" {
		t.Fatalf("expected APIKeyModeLive to be %q, got %q", "live", APIKeyModeLive)
	}

	if APIKeyModeTest != "test" {
		t.Fatalf("expected APIKeyModeTest to be %q, got %q", "test", APIKeyModeTest)
	}
}

func stringPtr(value string) *string {
	return &value
}
