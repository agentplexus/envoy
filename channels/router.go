package channels

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
)

// AgentProcessor processes messages through an AI agent.
type AgentProcessor interface {
	Process(ctx context.Context, sessionID, content string) (string, error)
}

// Router routes messages between channels and the agent.
type Router struct {
	channels map[string]Channel
	handlers []RouteHandler
	agent    AgentProcessor
	logger   *slog.Logger
	mu       sync.RWMutex
}

// RouteHandler processes routed messages.
type RouteHandler struct {
	Pattern RoutePattern
	Handler MessageHandler
}

// RoutePattern defines which messages to match.
type RoutePattern struct {
	// Channels limits to specific channels (empty = all).
	Channels []string

	// ChatTypes limits to specific chat types (empty = all).
	ChatTypes []ChannelType

	// Prefix matches messages starting with a prefix.
	Prefix string
}

// NewRouter creates a new message router.
func NewRouter(logger *slog.Logger) *Router {
	if logger == nil {
		logger = slog.Default()
	}
	return &Router{
		channels: make(map[string]Channel),
		handlers: []RouteHandler{},
		logger:   logger,
	}
}

// SetAgent sets the agent processor for the router.
func (r *Router) SetAgent(agent AgentProcessor) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.agent = agent
}

// ProcessWithAgent creates a message handler that processes through the agent and sends responses.
func (r *Router) ProcessWithAgent() MessageHandler {
	return func(ctx context.Context, msg IncomingMessage) error {
		r.mu.RLock()
		agent := r.agent
		r.mu.RUnlock()

		if agent == nil {
			r.logger.Warn("no agent configured, message not processed",
				"channel", msg.ChannelName,
				"chat", msg.ChatID)
			return nil
		}

		// Use chatID as session ID for conversation continuity
		sessionID := fmt.Sprintf("%s:%s", msg.ChannelName, msg.ChatID)

		r.logger.Info("processing message",
			"channel", msg.ChannelName,
			"chat", msg.ChatID,
			"from", msg.SenderName)

		response, err := agent.Process(ctx, sessionID, msg.Content)
		if err != nil {
			r.logger.Error("agent processing error",
				"channel", msg.ChannelName,
				"chat", msg.ChatID,
				"error", err)
			return err
		}

		// Send response back to the same channel/chat
		return r.Send(ctx, msg.ChannelName, msg.ChatID, OutgoingMessage{
			Content: response,
			ReplyTo: msg.ID,
		})
	}
}

// Register adds a channel to the router.
func (r *Router) Register(channel Channel) {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := channel.Name()
	r.channels[name] = channel

	// Set up message handler
	channel.OnMessage(func(ctx context.Context, msg IncomingMessage) error {
		return r.route(ctx, msg)
	})

	r.logger.Info("channel registered", "name", name)
}

// Unregister removes a channel from the router.
func (r *Router) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.channels, name)
	r.logger.Info("channel unregistered", "name", name)
}

// OnMessage adds a message handler with a pattern.
func (r *Router) OnMessage(pattern RoutePattern, handler MessageHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers = append(r.handlers, RouteHandler{
		Pattern: pattern,
		Handler: handler,
	})
}

// Send sends a message to a specific channel and chat.
func (r *Router) Send(ctx context.Context, channelName, chatID string, msg OutgoingMessage) error {
	r.mu.RLock()
	channel, ok := r.channels[channelName]
	r.mu.RUnlock()

	if !ok {
		return fmt.Errorf("channel not found: %s", channelName)
	}

	return channel.Send(ctx, chatID, msg)
}

// Broadcast sends a message to all registered channels.
func (r *Router) Broadcast(ctx context.Context, chatIDs map[string]string, msg OutgoingMessage) error {
	r.mu.RLock()
	channels := make(map[string]Channel, len(r.channels))
	for k, v := range r.channels {
		channels[k] = v
	}
	r.mu.RUnlock()

	var errs []error
	for name, chatID := range chatIDs {
		if channel, ok := channels[name]; ok {
			if err := channel.Send(ctx, chatID, msg); err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", name, err))
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("broadcast errors: %v", errs)
	}
	return nil
}

// ConnectAll connects all registered channels.
func (r *Router) ConnectAll(ctx context.Context) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for name, channel := range r.channels {
		if err := channel.Connect(ctx); err != nil {
			return fmt.Errorf("connect %s: %w", name, err)
		}
		r.logger.Info("channel connected", "name", name)
	}
	return nil
}

// DisconnectAll disconnects all registered channels.
func (r *Router) DisconnectAll(ctx context.Context) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var errs []error
	for name, channel := range r.channels {
		if err := channel.Disconnect(ctx); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", name, err))
		} else {
			r.logger.Info("channel disconnected", "name", name)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("disconnect errors: %v", errs)
	}
	return nil
}

// GetChannel returns a channel by name.
func (r *Router) GetChannel(name string) (Channel, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ch, ok := r.channels[name]
	return ch, ok
}

// ListChannels returns all registered channel names.
func (r *Router) ListChannels() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.channels))
	for name := range r.channels {
		names = append(names, name)
	}
	return names
}

// route dispatches a message to matching handlers.
func (r *Router) route(ctx context.Context, msg IncomingMessage) error {
	r.mu.RLock()
	handlers := make([]RouteHandler, len(r.handlers))
	copy(handlers, r.handlers)
	r.mu.RUnlock()

	for _, h := range handlers {
		if matchPattern(h.Pattern, msg) {
			if err := h.Handler(ctx, msg); err != nil {
				r.logger.Error("handler error",
					"channel", msg.ChannelName,
					"chat", msg.ChatID,
					"error", err)
				// Continue to other handlers
			}
		}
	}
	return nil
}

// matchPattern checks if a message matches a route pattern.
func matchPattern(pattern RoutePattern, msg IncomingMessage) bool {
	// Check channel filter
	if len(pattern.Channels) > 0 {
		found := false
		for _, ch := range pattern.Channels {
			if ch == msg.ChannelName {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check chat type filter
	if len(pattern.ChatTypes) > 0 {
		found := false
		for _, ct := range pattern.ChatTypes {
			if ct == msg.ChatType {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check prefix filter
	if pattern.Prefix != "" {
		if len(msg.Content) < len(pattern.Prefix) {
			return false
		}
		if msg.Content[:len(pattern.Prefix)] != pattern.Prefix {
			return false
		}
	}

	return true
}

// All returns a pattern that matches all messages.
func All() RoutePattern {
	return RoutePattern{}
}

// FromChannels returns a pattern that matches messages from specific channels.
func FromChannels(channels ...string) RoutePattern {
	return RoutePattern{Channels: channels}
}

// DMOnly returns a pattern that matches only DM messages.
func DMOnly() RoutePattern {
	return RoutePattern{ChatTypes: []ChannelType{ChannelTypeDM}}
}

// GroupOnly returns a pattern that matches only group messages.
func GroupOnly() RoutePattern {
	return RoutePattern{ChatTypes: []ChannelType{ChannelTypeGroup}}
}
