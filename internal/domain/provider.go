package domain

import "time"

type Channel string

const (
	ChannelEmail Channel = "email"
	ChannelPush  Channel = "push"
)

type Provider struct {
	ID        string
	Name      string
	Channel   Channel
	Config    map[string]any
	IsActive  bool
	CreatedAt time.Time
	UpdatedAt time.Time
}
