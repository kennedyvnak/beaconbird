package provider

import (
	"fmt"

	"github.com/kennedyvnak/beaconbird/internal/domain"
)

type Registry struct {
	providers map[domain.Channel]DeliveryProvider
}

func NewRegistry() *Registry {
	return &Registry{providers: make(map[domain.Channel]DeliveryProvider)}
}

func (r *Registry) Register(p DeliveryProvider) {
	if r.providers == nil {
		r.providers = make(map[domain.Channel]DeliveryProvider)
	}
	r.providers[p.Channel()] = p
}

func (r *Registry) For(channel domain.Channel) (DeliveryProvider, error) {
	provider, ok := r.providers[channel]
	if !ok {
		return nil, fmt.Errorf("no provider registered for channel %q", channel)
	}
	return provider, nil
}
