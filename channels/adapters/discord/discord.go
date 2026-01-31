// Package discord provides a Discord channel adapter for envoy.
package discord

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bwmarrin/discordgo"

	"github.com/agentplexus/envoy/channels"
)

// Adapter implements the Channel interface for Discord.
type Adapter struct {
	session        *discordgo.Session
	token          string
	guildID        string
	logger         *slog.Logger
	messageHandler channels.MessageHandler
	eventHandler   channels.EventHandler
}

// Config configures the Discord adapter.
type Config struct {
	Token   string
	GuildID string
	Logger  *slog.Logger
}

// New creates a new Discord adapter.
func New(config Config) (*Adapter, error) {
	if config.Token == "" {
		return nil, fmt.Errorf("discord token required")
	}
	if config.Logger == nil {
		config.Logger = slog.Default()
	}

	return &Adapter{
		token:   config.Token,
		guildID: config.GuildID,
		logger:  config.Logger,
	}, nil
}

// Name returns the channel name.
func (a *Adapter) Name() string {
	return "discord"
}

// Connect establishes connection to Discord.
func (a *Adapter) Connect(ctx context.Context) error {
	session, err := discordgo.New("Bot " + a.token)
	if err != nil {
		return fmt.Errorf("create discord session: %w", err)
	}

	a.session = session

	// Set up message handler
	a.session.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		// Ignore messages from the bot itself
		if m.Author.ID == s.State.User.ID {
			return
		}

		if a.messageHandler != nil {
			msg := a.convertIncoming(m)
			if err := a.messageHandler(ctx, msg); err != nil {
				a.logger.Error("message handler error", "error", err)
			}
		}
	})

	// Set intents
	a.session.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentsDirectMessages | discordgo.IntentsMessageContent

	// Open connection
	if err := a.session.Open(); err != nil {
		return fmt.Errorf("open discord session: %w", err)
	}

	a.logger.Info("discord bot connected", "user", a.session.State.User.Username)
	return nil
}

// Disconnect closes the Discord connection.
func (a *Adapter) Disconnect(ctx context.Context) error {
	if a.session != nil {
		if err := a.session.Close(); err != nil {
			return fmt.Errorf("close discord session: %w", err)
		}
		a.logger.Info("discord bot disconnected")
	}
	return nil
}

// Send sends a message to a Discord channel.
func (a *Adapter) Send(ctx context.Context, channelID string, msg channels.OutgoingMessage) error {
	if a.session == nil {
		return fmt.Errorf("discord session not connected")
	}

	// Build message send options
	data := &discordgo.MessageSend{
		Content: msg.Content,
	}

	if msg.ReplyTo != "" {
		data.Reference = &discordgo.MessageReference{
			MessageID: msg.ReplyTo,
		}
	}

	_, err := a.session.ChannelMessageSendComplex(channelID, data)
	if err != nil {
		return fmt.Errorf("send message: %w", err)
	}

	return nil
}

// OnMessage registers a message handler.
func (a *Adapter) OnMessage(handler channels.MessageHandler) {
	a.messageHandler = handler
}

// OnEvent registers an event handler.
func (a *Adapter) OnEvent(handler channels.EventHandler) {
	a.eventHandler = handler
}

// convertIncoming converts a Discord message to an IncomingMessage.
func (a *Adapter) convertIncoming(m *discordgo.MessageCreate) channels.IncomingMessage {
	chatType := channels.ChannelTypeGroup
	// Check if it's a DM
	if m.GuildID == "" {
		chatType = channels.ChannelTypeDM
	}

	// Check for thread
	if m.Thread != nil {
		chatType = channels.ChannelTypeThread
	}

	return channels.IncomingMessage{
		ID:          m.ID,
		ChannelName: "discord",
		ChatID:      m.ChannelID,
		ChatType:    chatType,
		SenderID:    m.Author.ID,
		SenderName:  m.Author.Username,
		Content:     m.Content,
		ReplyTo:     getReplyTo(m),
		Timestamp:   m.Timestamp,
		Metadata: map[string]interface{}{
			"guild_id":      m.GuildID,
			"discriminator": m.Author.Discriminator,
		},
	}
}

// getReplyTo extracts the reply-to message ID if present.
func getReplyTo(m *discordgo.MessageCreate) string {
	if m.MessageReference != nil {
		return m.MessageReference.MessageID
	}
	return ""
}

// Ensure Adapter implements Channel interface.
var _ channels.Channel = (*Adapter)(nil)
