package provider

import (
	"context"
	"fmt"

	"github.com/kennedyvnak/beaconbird/internal/domain"
)

type MockProvider struct {
	channel domain.Channel
	name    string
}

func NewMockProvider(channel domain.Channel) *MockProvider {
	return &MockProvider{
		channel: channel,
		name:    fmt.Sprintf("mock-%s", channel),
	}
}

func (p *MockProvider) Channel() domain.Channel {
	return p.channel
}

func (p *MockProvider) Name() string {
	return p.name
}

func (p *MockProvider) Send(ctx context.Context, job domain.DeliveryJob) (*ProviderResult, error) {
	return &ProviderResult{
		ProviderMessageID: fmt.Sprintf("mock:%s:%s", p.channel, job.ID),
	}, nil
}
