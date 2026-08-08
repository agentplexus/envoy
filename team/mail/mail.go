// Package mail delivers team emails (magic-link logins). It exposes a small
// Mailer interface with two implementations: an SMTP mailer for real
// delivery and a log mailer for local development, which prints the message
// (including the magic link) to the logger so the flow can be exercised
// without any SMTP configuration.
package mail

import (
	"context"
	"log/slog"
)

// Message is an outbound email.
type Message struct {
	To       string
	Subject  string
	TextBody string
	HTMLBody string
}

// Mailer sends email messages.
type Mailer interface {
	Send(ctx context.Context, msg Message) error
}

// LogMailer logs messages instead of sending them. It is the default when no
// SMTP host is configured, so local/dev deployments can complete the
// magic-link flow by reading the link from the logs.
type LogMailer struct {
	Logger *slog.Logger
}

// NewLogMailer creates a LogMailer.
func NewLogMailer(logger *slog.Logger) *LogMailer {
	if logger == nil {
		logger = slog.Default()
	}
	return &LogMailer{Logger: logger}
}

// Send logs the message. The full text body (which contains the magic link)
// is logged at Info so an operator testing locally can copy the link.
func (m *LogMailer) Send(_ context.Context, msg Message) error {
	m.Logger.Info("email (log mailer — not actually sent)",
		"to", msg.To,
		"subject", msg.Subject,
		"body", msg.TextBody,
	)
	return nil
}
