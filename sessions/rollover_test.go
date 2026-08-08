package sessions

import (
	"testing"
	"time"

	"github.com/plexusone/omnillm/provider"
)

func sessionUpdatedAt(updated time.Time, messages int) *Session {
	s := NewSession("roll-test")
	for i := 0; i < messages; i++ {
		s.AddMessage(provider.RoleUser, "hi")
	}
	s.UpdatedAt = updated
	return s
}

func TestRolloverPolicy_ShouldRollover(t *testing.T) {
	la, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Fatalf("LoadLocation: %v", err)
	}

	// 2026-08-03 01:30 UTC == 2026-08-02 18:30 in LA.
	now := time.Date(2026, 8, 3, 1, 30, 0, 0, time.UTC)

	tests := []struct {
		name       string
		policy     *RolloverPolicy
		session    *Session
		fallback   *time.Location
		wantReason RolloverReason
		wantRoll   bool
	}{
		{
			name:     "nil policy never rolls",
			policy:   nil,
			session:  sessionUpdatedAt(now.Add(-48*time.Hour), 3),
			wantRoll: false,
		},
		{
			name:     "empty session never rolls",
			policy:   &RolloverPolicy{IdleTimeout: time.Minute, Daily: true},
			session:  sessionUpdatedAt(now.Add(-48*time.Hour), 0),
			wantRoll: false,
		},
		{
			name:       "idle timeout exceeded",
			policy:     &RolloverPolicy{IdleTimeout: time.Hour},
			session:    sessionUpdatedAt(now.Add(-2*time.Hour), 3),
			wantReason: RolloverReasonIdle,
			wantRoll:   true,
		},
		{
			name:     "recent activity does not roll",
			policy:   &RolloverPolicy{IdleTimeout: time.Hour},
			session:  sessionUpdatedAt(now.Add(-10*time.Minute), 3),
			wantRoll: false,
		},
		{
			name:   "day boundary crossed in UTC",
			policy: &RolloverPolicy{Daily: true},
			// 2026-08-02 23:00 UTC -> now is 2026-08-03 in UTC.
			session:    sessionUpdatedAt(time.Date(2026, 8, 2, 23, 0, 0, 0, time.UTC), 3),
			wantReason: RolloverReasonDaily,
			wantRoll:   true,
		},
		{
			name:   "same LA day even though UTC day changed",
			policy: &RolloverPolicy{Daily: true, Location: la},
			// 2026-08-02 20:00 UTC == 2026-08-02 13:00 LA; now == 2026-08-02 18:30 LA.
			session:  sessionUpdatedAt(time.Date(2026, 8, 2, 20, 0, 0, 0, time.UTC), 3),
			wantRoll: false,
		},
		{
			name:   "fallback location used when policy has none",
			policy: &RolloverPolicy{Daily: true},
			// Same LA day (see above) — with LA fallback, no rollover even
			// though the UTC day changed.
			session:  sessionUpdatedAt(time.Date(2026, 8, 2, 20, 0, 0, 0, time.UTC), 3),
			fallback: la,
			wantRoll: false,
		},
		{
			name:       "idle takes precedence over daily",
			policy:     &RolloverPolicy{IdleTimeout: time.Hour, Daily: true},
			session:    sessionUpdatedAt(now.Add(-30*time.Hour), 3),
			wantReason: RolloverReasonIdle,
			wantRoll:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reason, roll := tt.policy.ShouldRollover(tt.session, now, tt.fallback)
			if roll != tt.wantRoll {
				t.Fatalf("roll = %v, want %v", roll, tt.wantRoll)
			}
			if roll && reason != tt.wantReason {
				t.Errorf("reason = %q, want %q", reason, tt.wantReason)
			}
		})
	}
}
