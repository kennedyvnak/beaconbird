package provider

import (
	"context"
	"errors"
	"testing"

	"firebase.google.com/go/v4/messaging"
	"github.com/kennedyvnak/beaconbird/internal/domain"
)

func TestRegistryReturnsRegisteredProvider(t *testing.T) {
	registry := NewRegistry()
	expected := stubProvider{channel: domain.ChannelPush, name: "fcm"}

	registry.Register(expected)

	got, err := registry.For(domain.ChannelPush)
	if err != nil {
		t.Fatalf("For returned error: %v", err)
	}
	if got.Name() != expected.name {
		t.Fatalf("expected provider %q, got %q", expected.name, got.Name())
	}
}

func TestRegistryReturnsErrorForUnknownChannel(t *testing.T) {
	registry := NewRegistry()

	_, err := registry.For(domain.ChannelEmail)
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestFCMProviderSendMapsPayload(t *testing.T) {
	client := &fakeFCMClient{response: "provider-msg-1"}
	provider := newFCMProviderWithClient(client)

	result, err := provider.Send(context.Background(), domain.DeliveryJob{
		ID:      "job-1",
		Channel: domain.ChannelPush,
		Payload: map[string]any{
			"device_token": "device-1",
			"title":        "Hello",
			"body":         "World",
			"data": map[string]any{
				"campaign": "launch",
			},
		},
	})
	if err != nil {
		t.Fatalf("Send returned error: %v", err)
	}
	if result.ProviderMessageID != "provider-msg-1" {
		t.Fatalf("expected provider message id to be returned")
	}
	if client.message == nil {
		t.Fatalf("expected provider to send a message")
	}
	if client.message.Token != "device-1" {
		t.Fatalf("expected token to be mapped, got %q", client.message.Token)
	}
	if client.message.Notification == nil || client.message.Notification.Title != "Hello" || client.message.Notification.Body != "World" {
		t.Fatalf("expected notification payload to be mapped, got %#v", client.message.Notification)
	}
	if client.message.Data["campaign"] != "launch" {
		t.Fatalf("expected data payload to be mapped, got %#v", client.message.Data)
	}
}

func TestFCMProviderSendWrapsClientError(t *testing.T) {
	client := &fakeFCMClient{err: errors.New("boom")}
	provider := newFCMProviderWithClient(client)

	_, err := provider.Send(context.Background(), domain.DeliveryJob{
		ID:      "job-1",
		Channel: domain.ChannelPush,
		Payload: map[string]any{
			"device_token": "device-1",
			"title":        "Hello",
			"body":         "World",
		},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if got := err.Error(); got != "fcm.Send: boom" {
		t.Fatalf("expected wrapped error, got %q", got)
	}
}

type fakeFCMClient struct {
	message  *messaging.Message
	response string
	err      error
}

func (f *fakeFCMClient) Send(ctx context.Context, message *messaging.Message) (string, error) {
	f.message = message
	return f.response, f.err
}

type stubProvider struct {
	channel domain.Channel
	name    string
}

func (s stubProvider) Channel() domain.Channel { return s.channel }
func (s stubProvider) Name() string            { return s.name }
func (s stubProvider) Send(ctx context.Context, job domain.DeliveryJob) (*ProviderResult, error) {
	return &ProviderResult{ProviderMessageID: job.ID}, nil
}
