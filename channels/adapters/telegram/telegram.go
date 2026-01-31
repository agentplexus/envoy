// Package telegram provides a Telegram channel adapter for envoy.
package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"gopkg.in/telebot.v3"

	"github.com/agentplexus/envoy/channels"
)

// Adapter implements the Channel interface for Telegram.
type Adapter struct {
	bot            *telebot.Bot
	token          string
	logger         *slog.Logger
	messageHandler channels.MessageHandler
	eventHandler   channels.EventHandler
}

// Config configures the Telegram adapter.
type Config struct {
	Token  string
	Logger *slog.Logger
}

// New creates a new Telegram adapter.
func New(config Config) (*Adapter, error) {
	if config.Token == "" {
		return nil, fmt.Errorf("telegram token required")
	}
	if config.Logger == nil {
		config.Logger = slog.Default()
	}

	return &Adapter{
		token:  config.Token,
		logger: config.Logger,
	}, nil
}

// Name returns the channel name.
func (a *Adapter) Name() string {
	return "telegram"
}

// Connect establishes connection to Telegram.
func (a *Adapter) Connect(ctx context.Context) error {
	pref := telebot.Settings{
		Token:  a.token,
		Poller: &telebot.LongPoller{Timeout: 10 * time.Second},
	}

	bot, err := telebot.NewBot(pref)
	if err != nil {
		return fmt.Errorf("create telegram bot: %w", err)
	}

	a.bot = bot

	// Set up message handler
	a.bot.Handle(telebot.OnText, func(c telebot.Context) error {
		if a.messageHandler == nil {
			return nil
		}

		msg := a.convertIncoming(c.Message())
		return a.messageHandler(ctx, msg)
	})

	// Start bot in background
	go func() {
		a.logger.Info("starting telegram bot")
		a.bot.Start()
	}()

	return nil
}

// Disconnect closes the Telegram connection.
func (a *Adapter) Disconnect(ctx context.Context) error {
	if a.bot != nil {
		a.bot.Stop()
		a.logger.Info("telegram bot stopped")
	}
	return nil
}

// Send sends a message to a Telegram chat.
func (a *Adapter) Send(ctx context.Context, chatID string, msg channels.OutgoingMessage) error {
	if a.bot == nil {
		return fmt.Errorf("telegram bot not connected")
	}

	// Parse chat ID
	chatIDInt, err := strconv.ParseInt(chatID, 10, 64)
	if err != nil {
		return fmt.Errorf("parse chat ID: %w", err)
	}
	chat, err := a.bot.ChatByID(chatIDInt)
	if err != nil {
		return fmt.Errorf("get chat: %w", err)
	}

	// Send text message
	opts := &telebot.SendOptions{}
	switch msg.Format {
	case channels.MessageFormatMarkdown:
		opts.ParseMode = telebot.ModeMarkdown
	case channels.MessageFormatHTML:
		opts.ParseMode = telebot.ModeHTML
	}

	// TODO: Handle reply_to when msg.ReplyTo != ""

	_, err = a.bot.Send(chat, msg.Content, opts)
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

// convertIncoming converts a Telegram message to an IncomingMessage.
func (a *Adapter) convertIncoming(msg *telebot.Message) channels.IncomingMessage {
	var chatType channels.ChannelType
	switch msg.Chat.Type {
	case telebot.ChatGroup, telebot.ChatSuperGroup:
		chatType = channels.ChannelTypeGroup
	case telebot.ChatChannel:
		chatType = channels.ChannelTypeChannel
	default:
		chatType = channels.ChannelTypeDM
	}

	senderName := msg.Sender.FirstName
	if msg.Sender.LastName != "" {
		senderName += " " + msg.Sender.LastName
	}
	if senderName == "" {
		senderName = msg.Sender.Username
	}

	return channels.IncomingMessage{
		ID:          fmt.Sprintf("%d", msg.ID),
		ChannelName: "telegram",
		ChatID:      fmt.Sprintf("%d", msg.Chat.ID),
		ChatType:    chatType,
		SenderID:    fmt.Sprintf("%d", msg.Sender.ID),
		SenderName:  senderName,
		Content:     msg.Text,
		Timestamp:   msg.Time(),
		Metadata: map[string]interface{}{
			"chat_title": msg.Chat.Title,
			"username":   msg.Sender.Username,
		},
	}
}

// Ensure Adapter implements Channel interface.
var _ channels.Channel = (*Adapter)(nil)
