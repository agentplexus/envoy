package mail

import (
	"context"
	"fmt"

	gomail "github.com/wneessen/go-mail"
)

// SMTPConfig configures the SMTP mailer.
type SMTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	// From is the envelope/header sender address.
	From string
}

// SMTPMailer delivers email over SMTP.
type SMTPMailer struct {
	cfg    SMTPConfig
	client *gomail.Client
}

// NewSMTPMailer creates an SMTP mailer. TLS is mandatory when a port is
// given (STARTTLS on 587 / implicit on 465 are handled by go-mail's port
// policy); auth is auto-discovered, or disabled when no username is set
// (common for relay-on-localhost setups).
func NewSMTPMailer(cfg SMTPConfig) (*SMTPMailer, error) {
	if cfg.Host == "" {
		return nil, fmt.Errorf("smtp: host is required")
	}
	if cfg.From == "" {
		return nil, fmt.Errorf("smtp: from is required")
	}
	port := cfg.Port
	if port == 0 {
		port = 587
	}

	opts := []gomail.Option{
		gomail.WithPort(port),
		gomail.WithTLSPortPolicy(gomail.TLSMandatory),
	}
	if cfg.Username != "" {
		opts = append(opts,
			gomail.WithSMTPAuth(gomail.SMTPAuthAutoDiscover),
			gomail.WithUsername(cfg.Username),
			gomail.WithPassword(cfg.Password),
		)
	} else {
		opts = append(opts, gomail.WithSMTPAuth(gomail.SMTPAuthNoAuth))
	}

	client, err := gomail.NewClient(cfg.Host, opts...)
	if err != nil {
		return nil, fmt.Errorf("smtp: new client: %w", err)
	}
	return &SMTPMailer{cfg: cfg, client: client}, nil
}

// Send delivers the message over SMTP with a plain-text body and an HTML
// alternative when provided.
func (m *SMTPMailer) Send(ctx context.Context, msg Message) error {
	gm := gomail.NewMsg()
	if err := gm.From(m.cfg.From); err != nil {
		return fmt.Errorf("smtp: from: %w", err)
	}
	if err := gm.To(msg.To); err != nil {
		return fmt.Errorf("smtp: to: %w", err)
	}
	gm.Subject(msg.Subject)
	gm.SetBodyString(gomail.TypeTextPlain, msg.TextBody)
	if msg.HTMLBody != "" {
		gm.AddAlternativeString(gomail.TypeTextHTML, msg.HTMLBody)
	}

	if err := m.client.DialAndSendWithContext(ctx, gm); err != nil {
		return fmt.Errorf("smtp: send: %w", err)
	}
	return nil
}
