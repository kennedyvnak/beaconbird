package provider

import "context"

// ProviderResult captures provider-specific identifiers returned after delivery.
type ProviderResult struct {
	ProviderMessageID string
}

// TODO: implement SES and FCM providers plus registry wiring.

type Sender interface {
	Send(ctx context.Context, payload []byte) (*ProviderResult, error)
}
