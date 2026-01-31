// Package channels provides channel abstraction for messaging platforms.
package channels

import (
	"context"
)

// Channel represents a messaging channel (Telegram, Discord, etc.).
type Channel interface {
	// Name returns the channel name (e.g., "telegram", "discord").
	Name() string

	// Connect establishes connection to the channel.
	Connect(ctx context.Context) error

	// Disconnect closes the channel connection.
	Disconnect(ctx context.Context) error

	// Send sends a message to a specific chat/channel.
	Send(ctx context.Context, chatID string, msg OutgoingMessage) error

	// OnMessage registers a handler for incoming messages.
	OnMessage(handler MessageHandler)

	// OnEvent registers a handler for channel events.
	OnEvent(handler EventHandler)
}

// StreamingChannel extends Channel with typing indicators.
type StreamingChannel interface {
	Channel

	// SendTyping sends a typing indicator.
	SendTyping(ctx context.Context, chatID string) error

	// SendStream sends a message as a stream of chunks.
	SendStream(ctx context.Context, chatID string, chunks <-chan string) error
}

// MessageHandler handles incoming messages.
type MessageHandler func(ctx context.Context, msg IncomingMessage) error

// EventHandler handles channel events.
type EventHandler func(ctx context.Context, event Event) error

// ChannelType represents the type of channel.
type ChannelType string

const (
	ChannelTypeDM      ChannelType = "dm"
	ChannelTypeGroup   ChannelType = "group"
	ChannelTypeChannel ChannelType = "channel"
	ChannelTypeThread  ChannelType = "thread"
)

// ChannelConfig is the base configuration for channels.
type ChannelConfig struct {
	Enabled bool
	Token   string
}
