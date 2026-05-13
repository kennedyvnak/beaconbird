package provider

import (
	"context"
	"errors"
	"testing"

	"firebase.google.com/go/v4/messaging"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
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

func TestSESProviderSendMapsPayload(t *testing.T) {
	client := &fakeSESClient{
		output: &sesv2.SendEmailOutput{MessageId: stringPtr("ses-msg-1")},
	}
	provider, err := newSESProviderWithClient(client, "noreply@example.com")
	if err != nil {
		t.Fatalf("newSESProviderWithClient returned error: %v", err)
	}

	result, err := provider.Send(context.Background(), domain.DeliveryJob{
		ID:      "job-1",
		Channel: domain.ChannelEmail,
		Payload: map[string]any{
			"to_email":  "user@example.com",
			"subject":   "Welcome",
			"body_text": "Hello text",
			"body_html": "<p>Hello html</p>",
		},
	})
	if err != nil {
		t.Fatalf("Send returned error: %v", err)
	}
	if result == nil || result.ProviderMessageID != "ses-msg-1" {
		t.Fatalf("expected provider message id to be returned, got %#v", result)
	}
	if client.input == nil {
		t.Fatalf("expected SES client to receive input")
	}
	if got := client.input.FromEmailAddress; got == nil || *got != "noreply@example.com" {
		t.Fatalf("expected from email to be mapped, got %#v", got)
	}
	if got := client.input.Destination.ToAddresses; len(got) != 1 || got[0] != "user@example.com" {
		t.Fatalf("expected destination to be mapped, got %#v", got)
	}
	if got := client.input.Content.Simple.Subject.Data; got == nil || *got != "Welcome" {
		t.Fatalf("expected subject to be mapped, got %#v", got)
	}
	if got := client.input.Content.Simple.Body.Text.Data; got == nil || *got != "Hello text" {
		t.Fatalf("expected text body to be mapped, got %#v", got)
	}
	if got := client.input.Content.Simple.Body.Html.Data; got == nil || *got != "<p>Hello html</p>" {
		t.Fatalf("expected html body to be mapped, got %#v", got)
	}
}

func TestSESProviderSendRequiresBody(t *testing.T) {
	client := &fakeSESClient{}
	provider, err := newSESProviderWithClient(client, "noreply@example.com")
	if err != nil {
		t.Fatalf("newSESProviderWithClient returned error: %v", err)
	}

	_, err = provider.Send(context.Background(), domain.DeliveryJob{
		ID:      "job-1",
		Channel: domain.ChannelEmail,
		Payload: map[string]any{
			"to_email": "user@example.com",
			"subject":  "Welcome",
		},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if got := err.Error(); got != "ses.Send: missing body_text or body_html" {
		t.Fatalf("expected wrapped error, got %q", got)
	}
}

func TestMockProviderSendReturnsSyntheticMessageID(t *testing.T) {
	provider := NewMockProvider(domain.ChannelEmail)

	result, err := provider.Send(context.Background(), domain.DeliveryJob{
		ID:      "job-1",
		Channel: domain.ChannelEmail,
	})
	if err != nil {
		t.Fatalf("Send returned error: %v", err)
	}
	if result == nil || result.ProviderMessageID == "" {
		t.Fatalf("expected synthetic provider message id, got %#v", result)
	}
	if provider.Name() != "mock-email" {
		t.Fatalf("expected email-specific mock provider name, got %q", provider.Name())
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

type fakeSESClient struct {
	input  *sesv2.SendEmailInput
	output *sesv2.SendEmailOutput
	err    error
}

func (f *fakeSESClient) SendEmail(ctx context.Context, input *sesv2.SendEmailInput, optFns ...func(*sesv2.Options)) (*sesv2.SendEmailOutput, error) {
	f.input = input
	return f.output, f.err
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
