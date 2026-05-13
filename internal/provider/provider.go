package provider

import (
	"context"

	"github.com/kennedyvnak/beaconbird/internal/domain"
)

// ProviderResult captures provider-specific identifiers returned after delivery.
type ProviderResult struct {
	ProviderMessageID string
}

type DeliveryProvider interface {
	Channel() domain.Channel
	Name() string
	Send(ctx context.Context, job domain.DeliveryJob) (*ProviderResult, error)
}
