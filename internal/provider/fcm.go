package provider

import (
	"context"
	"fmt"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"github.com/kennedyvnak/beaconbird/internal/domain"
	"google.golang.org/api/option"
)

type fcmClient interface {
	Send(ctx context.Context, message *messaging.Message) (string, error)
}

type FCMProvider struct {
	client fcmClient
}

func NewFCMProvider(ctx context.Context, credentialsJSON []byte) (*FCMProvider, error) {
	app, err := firebase.NewApp(ctx, nil, option.WithCredentialsJSON(credentialsJSON))
	if err != nil {
		return nil, fmt.Errorf("provider.NewFCMProvider app: %w", err)
	}
	client, err := app.Messaging(ctx)
	if err != nil {
		return nil, fmt.Errorf("provider.NewFCMProvider messaging: %w", err)
	}
	return &FCMProvider{client: client}, nil
}

func newFCMProviderWithClient(client fcmClient) *FCMProvider {
	return &FCMProvider{client: client}
}

func (p *FCMProvider) Channel() domain.Channel {
	return domain.ChannelPush
}

func (p *FCMProvider) Name() string {
	return "fcm"
}

func (p *FCMProvider) Send(ctx context.Context, job domain.DeliveryJob) (*ProviderResult, error) {
	token, err := payloadString(job.Payload, "device_token")
	if err != nil {
		return nil, fmt.Errorf("fcm.Send: %w", err)
	}
	title, err := payloadString(job.Payload, "title")
	if err != nil {
		return nil, fmt.Errorf("fcm.Send: %w", err)
	}
	body, err := payloadString(job.Payload, "body")
	if err != nil {
		return nil, fmt.Errorf("fcm.Send: %w", err)
	}

	message := &messaging.Message{
		Token: token,
		Notification: &messaging.Notification{
			Title: title,
			Body:  body,
		},
	}
	if data, err := payloadStringMap(job.Payload, "data"); err != nil {
		return nil, fmt.Errorf("fcm.Send: %w", err)
	} else if len(data) > 0 {
		message.Data = data
	}

	resp, err := p.client.Send(ctx, message)
	if err != nil {
		return nil, fmt.Errorf("fcm.Send: %w", err)
	}
	return &ProviderResult{ProviderMessageID: resp}, nil
}

func payloadString(payload map[string]any, key string) (string, error) {
	value, ok := payload[key]
	if !ok {
		return "", fmt.Errorf("missing %s", key)
	}
	text, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("invalid %s", key)
	}
	return text, nil
}

func payloadStringMap(payload map[string]any, key string) (map[string]string, error) {
	value, ok := payload[key]
	if !ok || value == nil {
		return nil, nil
	}

	switch typed := value.(type) {
	case map[string]string:
		result := make(map[string]string, len(typed))
		for k, v := range typed {
			result[k] = v
		}
		return result, nil
	case map[string]any:
		result := make(map[string]string, len(typed))
		for k, v := range typed {
			text, ok := v.(string)
			if !ok {
				return nil, fmt.Errorf("invalid %s.%s", key, k)
			}
			result[k] = text
		}
		return result, nil
	default:
		return nil, fmt.Errorf("invalid %s", key)
	}
}
