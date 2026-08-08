package mail

import (
	"strings"
	"testing"
	"time"
)

func TestMagicLinkMessage(t *testing.T) {
	msg := MagicLinkMessage("user@example.com", MagicLinkData{
		AppName: "FamilyAgent",
		Link:    "https://team.example.com/api/auth/verify?token=abc123",
		TTL:     15 * time.Minute,
	})

	if msg.To != "user@example.com" {
		t.Errorf("To = %q", msg.To)
	}
	if !strings.Contains(msg.Subject, "FamilyAgent") {
		t.Errorf("Subject = %q, want app name", msg.Subject)
	}
	for _, body := range []string{msg.TextBody, msg.HTMLBody} {
		if !strings.Contains(body, "https://team.example.com/api/auth/verify?token=abc123") {
			t.Errorf("body missing link: %q", body)
		}
		if !strings.Contains(body, "15 minutes") {
			t.Errorf("body missing TTL: %q", body)
		}
	}
}

func TestMagicLinkMessage_Defaults(t *testing.T) {
	msg := MagicLinkMessage("u@e.com", MagicLinkData{Link: "https://x/verify?token=t"})
	if !strings.Contains(msg.Subject, "OmniAgent") {
		t.Errorf("default app name missing: %q", msg.Subject)
	}
	if !strings.Contains(msg.TextBody, "1 minutes") {
		t.Errorf("zero TTL should floor to 1 minute: %q", msg.TextBody)
	}
}

func TestMagicLinkMessage_HTMLEscaped(t *testing.T) {
	// A crafted app name must not break out of HTML context.
	msg := MagicLinkMessage("u@e.com", MagicLinkData{
		AppName: `<script>x</script>`,
		Link:    "https://x/verify?token=t",
		TTL:     time.Minute,
	})
	if strings.Contains(msg.HTMLBody, "<script>x</script>") {
		t.Errorf("app name not HTML-escaped: %q", msg.HTMLBody)
	}
	if !strings.Contains(msg.HTMLBody, "&lt;script&gt;") {
		t.Errorf("expected escaped app name in %q", msg.HTMLBody)
	}
}
