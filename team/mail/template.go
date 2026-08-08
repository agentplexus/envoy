package mail

import (
	"fmt"
	"html"
	"time"
)

// MagicLinkData is the content for a magic-link email.
type MagicLinkData struct {
	// AppName is shown in the email (e.g. the team/agent name).
	AppName string
	// Link is the full verify URL the recipient clicks.
	Link string
	// TTL is how long the link remains valid.
	TTL time.Duration
}

// MagicLinkMessage renders the login email for a recipient.
func MagicLinkMessage(to string, d MagicLinkData) Message {
	appName := d.AppName
	if appName == "" {
		appName = "OmniAgent"
	}
	minutes := int(d.TTL.Minutes())
	if minutes < 1 {
		minutes = 1
	}

	subject := fmt.Sprintf("Sign in to %s", appName)

	text := fmt.Sprintf(`Sign in to %s

Click the link below to sign in. It expires in %d minutes and can be used once.

%s

If you did not request this, you can ignore this email.
`, appName, minutes, d.Link)

	htmlBody := fmt.Sprintf(`<!doctype html>
<html>
  <body style="font-family: -apple-system, Segoe UI, Roboto, sans-serif; line-height: 1.5; color: #1a1a1a;">
    <h2 style="margin: 0 0 12px;">Sign in to %s</h2>
    <p>Click the button below to sign in. It expires in %d minutes and can be used once.</p>
    <p>
      <a href="%s" style="display: inline-block; padding: 10px 18px; background: #2563eb; color: #ffffff; text-decoration: none; border-radius: 6px;">Sign in</a>
    </p>
    <p style="font-size: 12px; color: #666;">Or paste this link into your browser:<br><a href="%s">%s</a></p>
    <p style="font-size: 12px; color: #666;">If you did not request this, you can ignore this email.</p>
  </body>
</html>`, html.EscapeString(appName), minutes, html.EscapeString(d.Link), html.EscapeString(d.Link), html.EscapeString(d.Link))

	return Message{
		To:       to,
		Subject:  subject,
		TextBody: text,
		HTMLBody: htmlBody,
	}
}
