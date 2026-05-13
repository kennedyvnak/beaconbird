package worker

import (
	"context"
	"fmt"
	"os"

	"github.com/kennedyvnak/beaconbird/config"
	"github.com/kennedyvnak/beaconbird/internal/domain"
	"github.com/kennedyvnak/beaconbird/internal/provider"
)

type ProviderFactories struct {
	NewFCM   func(ctx context.Context, credentialsJSON []byte) (provider.DeliveryProvider, error)
	NewSES   func(ctx context.Context, region, accessKeyID, secretAccessKey, fromEmail string) (provider.DeliveryProvider, error)
	ReadFile func(path string) ([]byte, error)
}

func BuildProviderRegistry(ctx context.Context, cfg *config.Config, factories ProviderFactories) (*provider.Registry, error) {
	if cfg == nil {
		return nil, fmt.Errorf("worker.BuildProviderRegistry: config is required")
	}
	if factories.NewFCM == nil {
		factories.NewFCM = func(ctx context.Context, credentialsJSON []byte) (provider.DeliveryProvider, error) {
			return provider.NewFCMProvider(ctx, credentialsJSON)
		}
	}
	if factories.NewSES == nil {
		factories.NewSES = func(ctx context.Context, region, accessKeyID, secretAccessKey, fromEmail string) (provider.DeliveryProvider, error) {
			return provider.NewSESProvider(ctx, region, accessKeyID, secretAccessKey, fromEmail)
		}
	}
	if factories.ReadFile == nil {
		factories.ReadFile = os.ReadFile
	}

	registry := provider.NewRegistry()

	if cfg.EnableFCMProvider {
		credentialsJSON, err := factories.ReadFile(cfg.FCMCredentialsJSON)
		if err != nil {
			return nil, fmt.Errorf("worker.BuildProviderRegistry read fcm credentials: %w", err)
		}
		fcmProvider, err := factories.NewFCM(ctx, credentialsJSON)
		if err != nil {
			return nil, fmt.Errorf("worker.BuildProviderRegistry build fcm provider: %w", err)
		}
		registry.Register(fcmProvider)
	}

	if cfg.EnableSESProvider {
		sesProvider, err := factories.NewSES(ctx, cfg.AWSRegion, cfg.AWSAccessKeyID, cfg.AWSSecretAccessKey, cfg.SESFromEmail)
		if err != nil {
			return nil, fmt.Errorf("worker.BuildProviderRegistry build ses provider: %w", err)
		}
		registry.Register(sesProvider)
	}

	if cfg.EnableMockProvider {
		registerMockIfMissing(registry, domain.ChannelPush)
		registerMockIfMissing(registry, domain.ChannelEmail)
	}

	return registry, nil
}

func registerMockIfMissing(registry *provider.Registry, channel domain.Channel) {
	if _, err := registry.For(channel); err == nil {
		return
	}
	registry.Register(provider.NewMockProvider(channel))
}
