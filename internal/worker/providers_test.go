package worker

import (
	"context"
	"errors"
	"testing"

	"github.com/kennedyvnak/beaconbird/config"
	"github.com/kennedyvnak/beaconbird/internal/domain"
	"github.com/kennedyvnak/beaconbird/internal/provider"
)

func TestBuildProviderRegistryRegistersMockFallbacks(t *testing.T) {
	registry, err := BuildProviderRegistry(context.Background(), &config.Config{
		EnableMockProvider: true,
	}, ProviderFactories{
		NewFCM: func(ctx context.Context, credentialsJSON []byte) (provider.DeliveryProvider, error) {
			t.Fatalf("did not expect FCM builder to be called")
			return nil, nil
		},
		NewSES: func(ctx context.Context, region, accessKeyID, secretAccessKey, fromEmail string) (provider.DeliveryProvider, error) {
			t.Fatalf("did not expect SES builder to be called")
			return nil, nil
		},
	})
	if err != nil {
		t.Fatalf("BuildProviderRegistry returned error: %v", err)
	}

	pushProvider, err := registry.For(domain.ChannelPush)
	if err != nil {
		t.Fatalf("expected push mock provider, got error: %v", err)
	}
	if pushProvider.Name() != "mock-push" {
		t.Fatalf("expected push mock provider, got %q", pushProvider.Name())
	}

	emailProvider, err := registry.For(domain.ChannelEmail)
	if err != nil {
		t.Fatalf("expected email mock provider, got error: %v", err)
	}
	if emailProvider.Name() != "mock-email" {
		t.Fatalf("expected email mock provider, got %q", emailProvider.Name())
	}
}

func TestBuildProviderRegistryPrefersRealProvidersOverMockFallbacks(t *testing.T) {
	registry, err := BuildProviderRegistry(context.Background(), &config.Config{
		EnableFCMProvider:  true,
		EnableMockProvider: true,
	}, ProviderFactories{
		NewFCM: func(ctx context.Context, credentialsJSON []byte) (provider.DeliveryProvider, error) {
			return stubDeliveryProvider{channel: domain.ChannelPush, name: "fcm"}, nil
		},
		NewSES: func(ctx context.Context, region, accessKeyID, secretAccessKey, fromEmail string) (provider.DeliveryProvider, error) {
			t.Fatalf("did not expect SES builder to be called")
			return nil, nil
		},
		ReadFile: func(path string) ([]byte, error) {
			return []byte("credentials"), nil
		},
	})
	if err != nil {
		t.Fatalf("BuildProviderRegistry returned error: %v", err)
	}

	pushProvider, err := registry.For(domain.ChannelPush)
	if err != nil {
		t.Fatalf("expected push provider, got error: %v", err)
	}
	if pushProvider.Name() != "fcm" {
		t.Fatalf("expected FCM provider to win for push, got %q", pushProvider.Name())
	}

	emailProvider, err := registry.For(domain.ChannelEmail)
	if err != nil {
		t.Fatalf("expected email mock provider, got error: %v", err)
	}
	if emailProvider.Name() != "mock-email" {
		t.Fatalf("expected email mock fallback, got %q", emailProvider.Name())
	}
}

func TestBuildProviderRegistryReturnsBuilderErrors(t *testing.T) {
	_, err := BuildProviderRegistry(context.Background(), &config.Config{
		EnableSESProvider: true,
	}, ProviderFactories{
		NewFCM: func(ctx context.Context, credentialsJSON []byte) (provider.DeliveryProvider, error) {
			t.Fatalf("did not expect FCM builder to be called")
			return nil, nil
		},
		NewSES: func(ctx context.Context, region, accessKeyID, secretAccessKey, fromEmail string) (provider.DeliveryProvider, error) {
			return nil, errors.New("boom")
		},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if got := err.Error(); got != "worker.BuildProviderRegistry build ses provider: boom" {
		t.Fatalf("expected SES builder error to be wrapped, got %q", got)
	}
}

type stubDeliveryProvider struct {
	channel domain.Channel
	name    string
}

func (s stubDeliveryProvider) Channel() domain.Channel { return s.channel }
func (s stubDeliveryProvider) Name() string            { return s.name }
func (s stubDeliveryProvider) Send(ctx context.Context, job domain.DeliveryJob) (*provider.ProviderResult, error) {
	return &provider.ProviderResult{ProviderMessageID: job.ID}, nil
}
