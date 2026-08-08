package sessions

import "time"

// RolloverReason identifies why a session automatically rolled over.
type RolloverReason string

const (
	// RolloverReasonIdle marks a rollover caused by the idle timeout.
	RolloverReasonIdle RolloverReason = "idle"

	// RolloverReasonDaily marks a rollover caused by crossing a calendar-day
	// boundary in the configured timezone.
	RolloverReasonDaily RolloverReason = "daily"
)

// RolloverPolicy decides when a session automatically rolls over.
// A rollover ends the session's current conversation (its context can be
// persisted to memory by a hook) and starts fresh under the same session ID.
type RolloverPolicy struct {
	// IdleTimeout rolls a session over when more than this duration has
	// passed since its last update. Zero disables idle rollover.
	IdleTimeout time.Duration

	// Daily rolls a session over when a calendar-day boundary has been
	// crossed since its last update.
	Daily bool

	// Location resolves the day boundary for Daily rollovers. Nil falls
	// back to the caller's timezone (the agent uses its configured user
	// timezone, defaulting to UTC).
	Location *time.Location
}

// ShouldRollover reports whether the session should roll over at now, and
// why. Sessions with no messages never roll over — there is nothing to end.
// Idle takes precedence over daily when both apply.
func (p *RolloverPolicy) ShouldRollover(session *Session, now time.Time, fallback *time.Location) (RolloverReason, bool) {
	if p == nil || session == nil || len(session.Messages) == 0 {
		return "", false
	}

	if p.IdleTimeout > 0 && now.Sub(session.UpdatedAt) > p.IdleTimeout {
		return RolloverReasonIdle, true
	}

	if p.Daily {
		loc := p.Location
		if loc == nil {
			loc = fallback
		}
		if loc == nil {
			loc = time.UTC
		}
		last := session.UpdatedAt.In(loc)
		cur := now.In(loc)
		if last.Year() != cur.Year() || last.YearDay() != cur.YearDay() {
			return RolloverReasonDaily, true
		}
	}

	return "", false
}
