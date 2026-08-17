package gateway

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"time"

	"github.com/plexusone/omniagent/sessions"
)

// DefaultMessageHandler provides a basic message handler implementation.
type DefaultMessageHandler struct {
	gateway *Gateway
}

// NewDefaultMessageHandler creates a new default message handler.
func NewDefaultMessageHandler(gw *Gateway) *DefaultMessageHandler {
	return &DefaultMessageHandler{gateway: gw}
}

// Handle processes incoming messages.
func (h *DefaultMessageHandler) Handle(ctx context.Context, client *Client, msg *Message) (*Message, error) {
	switch msg.Type {
	case MessageTypePing:
		return h.handlePing(ctx, client, msg)
	case MessageTypeChat:
		return h.handleChat(ctx, client, msg)
	case MessageTypeAuth:
		return h.handleAuth(ctx, client, msg)
	case MessageTypeSubscribe:
		return h.handleSubscribe(ctx, client, msg)
	case MessageTypeSessionTools:
		return h.handleSessionTools(ctx, client, msg)
	case MessageTypeSessionModel:
		return h.handleSessionModel(ctx, client, msg)
	default:
		return NewErrorMessage(msg.ID, "unknown message type"), nil
	}
}

// gateClient applies the shared rate-limit and authentication gates for
// state-changing messages. It returns a non-nil error message when the
// request must be rejected.
func (h *DefaultMessageHandler) gateClient(client *Client, msg *Message) *Message {
	if h.gateway.rateLimiter != nil && !h.gateway.rateLimiter.Allow(client.ID) {
		h.gateway.logger.Warn("rate limit exceeded", "client_id", client.ID)
		return &Message{
			ID:   msg.ID,
			Type: MessageTypeError,
			Data: map[string]interface{}{
				"error":   "rate_limit_exceeded",
				"message": "Too many messages, please slow down",
			},
			Timestamp: time.Now(),
		}
	}

	if h.gateway.config.RequireAuth {
		authVal, _ := client.GetMetadata("authenticated")
		authenticated, _ := authVal.(bool)
		if !authenticated {
			return &Message{
				ID:   msg.ID,
				Type: MessageTypeError,
				Data: map[string]interface{}{
					"error":   "authentication_required",
					"message": "Please authenticate first",
				},
				Timestamp: time.Now(),
			}
		}
	}

	return nil
}

// handleSessionModel sets (or clears) the model override for the client's
// session. Requires an agent implementing SessionModelConfigurator. Data:
// {"model": "<name>", "sticky": bool} — empty model clears the override;
// sticky also updates the agent default (in-process, until restart).
func (h *DefaultMessageHandler) handleSessionModel(ctx context.Context, client *Client, msg *Message) (*Message, error) {
	if reject := h.gateClient(client, msg); reject != nil {
		return reject, nil
	}

	configurator, ok := h.gateway.agent.(SessionModelConfigurator)
	if !ok {
		return NewErrorMessage(msg.ID, "session model selection not supported"), nil
	}

	model, _ := msg.Data["model"].(string)
	sticky, _ := msg.Data["sticky"].(bool)

	if err := configurator.SetSessionModel(ctx, client.ID, model, sticky); err != nil {
		return NewErrorMessage(msg.ID, "set session model: "+err.Error()), nil
	}

	h.gateway.logger.Info("session model updated",
		"client_id", client.ID, "model", model, "sticky", sticky, "cleared", model == "")

	return &Message{
		ID:   msg.ID,
		Type: MessageTypeResponse,
		Data: map[string]interface{}{
			"model":      model,
			"sticky":     sticky,
			"session_id": client.ID,
		},
		Timestamp: time.Now(),
	}, nil
}

// handlePing handles ping messages.
func (h *DefaultMessageHandler) handlePing(_ context.Context, _ *Client, msg *Message) (*Message, error) {
	return &Message{
		ID:        msg.ID,
		Type:      MessageTypePong,
		Timestamp: time.Now(),
	}, nil
}

// handleChat handles chat messages.
func (h *DefaultMessageHandler) handleChat(ctx context.Context, client *Client, msg *Message) (*Message, error) {
	// Check rate limiting
	if h.gateway.rateLimiter != nil && !h.gateway.rateLimiter.Allow(client.ID) {
		h.gateway.logger.Warn("rate limit exceeded", "client_id", client.ID)
		return &Message{
			ID:   msg.ID,
			Type: MessageTypeError,
			Data: map[string]interface{}{
				"error":   "rate_limit_exceeded",
				"message": "Too many messages, please slow down",
			},
			Timestamp: time.Now(),
		}, nil
	}

	// Check if authentication is required
	if h.gateway.config.RequireAuth {
		authVal, _ := client.GetMetadata("authenticated")
		authenticated, _ := authVal.(bool)
		if !authenticated {
			return &Message{
				ID:   msg.ID,
				Type: MessageTypeError,
				Data: map[string]interface{}{
					"error":   "authentication_required",
					"message": "Please authenticate before sending messages",
				},
				Timestamp: time.Now(),
			}, nil
		}
	}

	// If no agent configured, echo the message
	if h.gateway.agent == nil {
		return &Message{
			ID:        msg.ID,
			Type:      MessageTypeResponse,
			Content:   "Message received: " + msg.Content,
			Timestamp: time.Now(),
		}, nil
	}

	// Process through agent.
	// Use client ID as session ID for conversation continuity: when the
	// agent has a durable session store configured, route through
	// ProcessWithSession so history actually persists (RMI-OMNIAGENT-007);
	// otherwise fall back to the stateless Process, unchanged from before.
	var response string
	var err error
	if sa, ok := h.gateway.agent.(SessionAwareProcessor); ok && sa.SessionStore() != nil {
		response, err = sa.ProcessWithSession(ctx, client.ID, msg.Content)
	} else {
		response, err = h.gateway.agent.Process(ctx, client.ID, msg.Content)
	}
	if err != nil {
		return NewErrorMessage(msg.ID, err.Error()), nil
	}

	return &Message{
		ID:        msg.ID,
		Type:      MessageTypeResponse,
		Content:   response,
		Channel:   msg.Channel,
		Timestamp: time.Now(),
	}, nil
}

// handleAuth handles authentication messages.
func (h *DefaultMessageHandler) handleAuth(ctx context.Context, client *Client, msg *Message) (*Message, error) {
	// If no API keys configured and auth not required, accept all
	if len(h.gateway.config.APIKeys) == 0 && !h.gateway.config.RequireAuth {
		client.SetMetadata("authenticated", true)
		return &Message{
			ID:   msg.ID,
			Type: MessageTypeResponse,
			Data: map[string]interface{}{
				"authenticated": true,
				"client_id":     client.ID,
			},
			Timestamp: time.Now(),
		}, nil
	}

	// Extract API key from message data
	apiKey, _ := msg.Data["api_key"].(string)
	if apiKey == "" {
		// Also check token field for compatibility
		apiKey, _ = msg.Data["token"].(string)
	}

	if apiKey == "" {
		return &Message{
			ID:   msg.ID,
			Type: MessageTypeError,
			Data: map[string]interface{}{
				"error":   "authentication_required",
				"message": "API key required",
			},
			Timestamp: time.Now(),
		}, nil
	}

	// Validate API key using constant-time comparison
	authenticated := false
	for _, validKey := range h.gateway.config.APIKeys {
		if subtle.ConstantTimeCompare([]byte(apiKey), []byte(validKey)) == 1 {
			authenticated = true
			break
		}
	}

	if !authenticated {
		h.gateway.logger.Warn("authentication failed", "client_id", client.ID, "remote_ip", client.RemoteIP())
		// Apply the escalating failure delay after the credential comparison
		// so correct credentials are never delayed. No source is locked out
		// (loopback included) — repeated failures only pay a bounded,
		// escalating delay.
		if err := h.gateway.authLimiter.recordFailureAndDelay(ctx, client.RemoteIP()); err != nil {
			return nil, err
		}
		return &Message{
			ID:   msg.ID,
			Type: MessageTypeError,
			Data: map[string]interface{}{
				"error":   "invalid_credentials",
				"message": "Invalid API key",
			},
			Timestamp: time.Now(),
		}, nil
	}

	// Successful authentication resets the penalty state for this source.
	h.gateway.authLimiter.reset(client.RemoteIP())
	client.SetMetadata("authenticated", true)
	h.gateway.logger.Info("client authenticated", "client_id", client.ID)

	return &Message{
		ID:   msg.ID,
		Type: MessageTypeResponse,
		Data: map[string]interface{}{
			"authenticated": true,
			"client_id":     client.ID,
		},
		Timestamp: time.Now(),
	}, nil
}

// handleSessionTools sets (or clears) per-session tool overrides for the
// client's session. Requires an agent implementing SessionToolConfigurator.
func (h *DefaultMessageHandler) handleSessionTools(ctx context.Context, client *Client, msg *Message) (*Message, error) {
	// Check rate limiting
	if h.gateway.rateLimiter != nil && !h.gateway.rateLimiter.Allow(client.ID) {
		h.gateway.logger.Warn("rate limit exceeded", "client_id", client.ID)
		return &Message{
			ID:   msg.ID,
			Type: MessageTypeError,
			Data: map[string]interface{}{
				"error":   "rate_limit_exceeded",
				"message": "Too many messages, please slow down",
			},
			Timestamp: time.Now(),
		}, nil
	}

	// Check if authentication is required
	if h.gateway.config.RequireAuth {
		authVal, _ := client.GetMetadata("authenticated")
		authenticated, _ := authVal.(bool)
		if !authenticated {
			return &Message{
				ID:   msg.ID,
				Type: MessageTypeError,
				Data: map[string]interface{}{
					"error":   "authentication_required",
					"message": "Please authenticate before configuring tools",
				},
				Timestamp: time.Now(),
			}, nil
		}
	}

	configurator, ok := h.gateway.agent.(SessionToolConfigurator)
	if !ok {
		return NewErrorMessage(msg.ID, "session tool overrides not supported"), nil
	}

	// Parse overrides from message data. Absent or null data clears them.
	var overrides *sessions.ToolOverrides
	if raw, exists := msg.Data["tool_overrides"]; exists && raw != nil {
		encoded, err := json.Marshal(raw)
		if err != nil {
			return NewErrorMessage(msg.ID, "invalid tool_overrides: "+err.Error()), nil
		}
		overrides = &sessions.ToolOverrides{}
		if err := json.Unmarshal(encoded, overrides); err != nil {
			return NewErrorMessage(msg.ID, "invalid tool_overrides: "+err.Error()), nil
		}
	}

	// The chat path uses the client ID as the session ID; overrides target
	// the same session.
	if err := configurator.SetSessionToolOverrides(ctx, client.ID, overrides); err != nil {
		return NewErrorMessage(msg.ID, "set tool overrides: "+err.Error()), nil
	}

	h.gateway.logger.Info("session tool overrides updated",
		"client_id", client.ID, "cleared", overrides == nil)

	return &Message{
		ID:   msg.ID,
		Type: MessageTypeResponse,
		Data: map[string]interface{}{
			"tool_overrides_set": overrides != nil,
			"session_id":         client.ID,
		},
		Timestamp: time.Now(),
	}, nil
}

// handleSubscribe handles channel subscription messages.
func (h *DefaultMessageHandler) handleSubscribe(_ context.Context, client *Client, msg *Message) (*Message, error) {
	channel := msg.Channel
	if channel == "" {
		return NewErrorMessage(msg.ID, "channel required"), nil
	}

	// Store subscription in client metadata
	subs, _ := client.GetMetadata("subscriptions")
	subscriptions, ok := subs.([]string)
	if !ok {
		subscriptions = []string{}
	}
	subscriptions = append(subscriptions, channel)
	client.SetMetadata("subscriptions", subscriptions)

	return &Message{
		ID:      msg.ID,
		Type:    MessageTypeResponse,
		Channel: channel,
		Data: map[string]interface{}{
			"subscribed": true,
		},
		Timestamp: time.Now(),
	}, nil
}
