package auth

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/plexusone/omniagent/internal/pgtest"
	"github.com/plexusone/omniagent/team"
	"github.com/plexusone/omniagent/team/ent"
	"github.com/plexusone/omniagent/team/mail"
	"github.com/plexusone/omniagent/team/store"
)

// captureMailer records sent messages so tests can extract the magic link.
type captureMailer struct {
	mu   sync.Mutex
	sent []mail.Message
}

func (m *captureMailer) Send(_ context.Context, msg mail.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = append(m.sent, msg)
	return nil
}

func (m *captureMailer) last() (mail.Message, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.sent) == 0 {
		return mail.Message{}, false
	}
	return m.sent[len(m.sent)-1], true
}

func (m *captureMailer) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.sent)
}

type fixture struct {
	svc     *Service
	team    *team.Service
	store   *store.Store
	mailer  *captureMailer
	rootAct team.Actor
}

func setup(t *testing.T) *fixture {
	t.Helper()
	ownerDSN, appDSN := pgtest.DSNs(t)
	ctx := context.Background()

	cfg := store.Config{AppDSN: appDSN, MigrateDSN: ownerDSN}
	if err := store.Migrate(ctx, cfg); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	st, err := store.Open(ctx, cfg)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	teamSvc, err := team.NewService(st, team.Config{SuperadminEmail: "root@example.com"})
	if err != nil {
		t.Fatalf("team.NewService: %v", err)
	}
	mailer := &captureMailer{}
	svc, err := NewService(st, teamSvc, mailer, Config{
		BaseURL:  "https://team.example.com",
		TokenTTL: 15 * time.Minute,
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	// Bootstrap the superadmin so it can manage the allowlist.
	root, _, err := teamSvc.EnsureUser(ctx, "root@example.com")
	if err != nil {
		t.Fatalf("bootstrap root: %v", err)
	}
	return &fixture{
		svc: svc, team: teamSvc, store: st, mailer: mailer,
		rootAct: team.Actor{UserID: root.ID, Superadmin: true},
	}
}

// linkToken extracts the raw token from the most recent magic-link email.
func (f *fixture) linkToken(t *testing.T) string {
	t.Helper()
	msg, ok := f.mailer.last()
	if !ok {
		t.Fatal("no email captured")
	}
	i := strings.Index(msg.TextBody, "token=")
	if i < 0 {
		t.Fatalf("no token in email body: %q", msg.TextBody)
	}
	tok := msg.TextBody[i+len("token="):]
	// Token runs to end-of-line.
	if nl := strings.IndexAny(tok, "\r\n"); nl >= 0 {
		tok = tok[:nl]
	}
	return strings.TrimSpace(tok)
}

func TestMagicLinkFlow(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	// Non-allowlisted email: uniform success, but NO email sent.
	if err := f.svc.RequestMagicLink(ctx, "stranger@example.com", "1.2.3.4"); err != nil {
		t.Fatalf("RequestMagicLink(stranger): %v", err)
	}
	if f.mailer.count() != 0 {
		t.Fatal("email sent to non-allowlisted address")
	}

	// Malformed email is the only case that errors.
	if err := f.svc.RequestMagicLink(ctx, "not-an-email", "1.2.3.4"); err != team.ErrInvalidEmail {
		t.Errorf("malformed email err = %v, want ErrInvalidEmail", err)
	}

	// Allowlist a member.
	if _, err := f.team.AllowlistAdd(ctx, f.rootAct, "member@example.com", ""); err != nil {
		t.Fatalf("AllowlistAdd: %v", err)
	}

	// Request → email sent.
	if err := f.svc.RequestMagicLink(ctx, "member@example.com", "1.2.3.4"); err != nil {
		t.Fatalf("RequestMagicLink(member): %v", err)
	}
	if f.mailer.count() != 1 {
		t.Fatalf("emails sent = %d, want 1", f.mailer.count())
	}
	token := f.linkToken(t)

	// Verify → session + first-login user creation.
	sessionToken, user, err := f.svc.VerifyMagicLink(ctx, token)
	if err != nil {
		t.Fatalf("VerifyMagicLink: %v", err)
	}
	if user.Email != "member@example.com" {
		t.Errorf("verified user email = %q", user.Email)
	}
	if sessionToken == "" {
		t.Fatal("empty session token")
	}

	// The token is single-use.
	if _, _, err := f.svc.VerifyMagicLink(ctx, token); err != ErrInvalidToken {
		t.Errorf("reused token err = %v, want ErrInvalidToken", err)
	}

	// Session authenticates to the member principal.
	p, err := f.svc.Authenticate(ctx, sessionToken)
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if p.Email != "member@example.com" || p.Superadmin {
		t.Errorf("principal = %+v, want member@example.com non-superadmin", p)
	}

	// Logout revokes the session.
	if err := f.svc.Logout(ctx, sessionToken); err != nil {
		t.Fatalf("Logout: %v", err)
	}
	if _, err := f.svc.Authenticate(ctx, sessionToken); err != ErrInvalidSession {
		t.Errorf("post-logout authenticate err = %v, want ErrInvalidSession", err)
	}
}

func TestVerify_ExpiredToken(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	if _, err := f.team.AllowlistAdd(ctx, f.rootAct, "exp@example.com", ""); err != nil {
		t.Fatalf("AllowlistAdd: %v", err)
	}
	if err := f.svc.RequestMagicLink(ctx, "exp@example.com", ""); err != nil {
		t.Fatalf("RequestMagicLink: %v", err)
	}
	token := f.linkToken(t)

	// Force the token to be expired via the system context.
	if err := f.store.AsSystem(ctx, func(ctx context.Context, tx *ent.Tx) error {
		_, err := tx.MagicLinkToken.Update().
			SetExpiresAt(time.Now().Add(-time.Minute)).Save(ctx)
		return err
	}); err != nil {
		t.Fatalf("expire token: %v", err)
	}

	if _, _, err := f.svc.VerifyMagicLink(ctx, token); err != ErrInvalidToken {
		t.Errorf("expired token err = %v, want ErrInvalidToken", err)
	}
}

func TestVerify_UnknownToken(t *testing.T) {
	f := setup(t)
	if _, _, err := f.svc.VerifyMagicLink(context.Background(), "totally-made-up"); err != ErrInvalidToken {
		t.Errorf("unknown token err = %v, want ErrInvalidToken", err)
	}
}

func TestSuperadminBootstrapViaMagicLink(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	// A fresh service whose superadmin has never logged in.
	if err := f.svc.RequestMagicLink(ctx, "root@example.com", ""); err != nil {
		t.Fatalf("RequestMagicLink(root): %v", err)
	}
	// The superadmin email is allowed even without an allowlist entry.
	if f.mailer.count() != 1 {
		t.Fatalf("superadmin email not sent (count=%d)", f.mailer.count())
	}
	token := f.linkToken(t)
	_, user, err := f.svc.VerifyMagicLink(ctx, token)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if user.Email != "root@example.com" {
		t.Errorf("user = %q", user.Email)
	}

	p, err := f.svc.Authenticate(ctx, mustSession(t, f, "root@example.com"))
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	if !p.Superadmin {
		t.Error("root did not resolve as superadmin")
	}
}

// mustSession requests+verifies a fresh link for email and returns the
// session token.
func mustSession(t *testing.T, f *fixture, email string) string {
	t.Helper()
	ctx := context.Background()
	if err := f.svc.RequestMagicLink(ctx, email, ""); err != nil {
		t.Fatalf("RequestMagicLink(%s): %v", email, err)
	}
	tok := f.linkToken(t)
	session, _, err := f.svc.VerifyMagicLink(ctx, tok)
	if err != nil {
		t.Fatalf("VerifyMagicLink(%s): %v", email, err)
	}
	return session
}
