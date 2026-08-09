package gateway

import (
	"context"
	"testing"
	"time"

	"github.com/plexusone/omniagent/sessions"
)

// fakeClockLimiter returns a limiter with a controllable clock anchored at
// the Unix epoch, so computed deadlines are always in the (real) past and
// recordFailureAndDelay returns without sleeping.
func fakeClockLimiter() (*authFailureLimiter, *time.Time) {
	now := time.Unix(0, 0)
	l := newAuthFailureLimiter()
	l.now = func() time.Time { return now }
	return l, &now
}

func (l *authFailureLimiter) penaltyState(key string) (time.Duration, time.Time, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	p, ok := l.penalties[key]
	if !ok {
		return 0, time.Time{}, false
	}
	return p.delay, p.until, true
}

func TestAuthFailureLimiter_Escalation(t *testing.T) {
	l, now := fakeClockLimiter()
	ctx := context.Background()

	want := []time.Duration{
		250 * time.Millisecond,
		500 * time.Millisecond,
		time.Second,
		2 * time.Second,
		4 * time.Second,
		5 * time.Second, // capped
		5 * time.Second, // stays capped
	}

	for i, wantDelay := range want {
		if err := l.recordFailureAndDelay(ctx, "10.0.0.1"); err != nil {
			t.Fatalf("recordFailureAndDelay #%d: %v", i+1, err)
		}
		delay, until, ok := l.penaltyState("10.0.0.1")
		if !ok {
			t.Fatalf("failure #%d: no penalty state", i+1)
		}
		if delay != wantDelay {
			t.Errorf("failure #%d: delay = %v, want %v", i+1, delay, wantDelay)
		}
		// Advance the clock past the window so the next failure escalates.
		*now = until.Add(time.Millisecond)
	}
}

func TestAuthFailureLimiter_ConcurrentFailuresShareWindow(t *testing.T) {
	l, now := fakeClockLimiter()
	ctx := context.Background()

	if err := l.recordFailureAndDelay(ctx, "10.0.0.2"); err != nil {
		t.Fatalf("recordFailureAndDelay: %v", err)
	}
	_, until1, _ := l.penaltyState("10.0.0.2")

	// A second failure inside the active window adopts the same deadline
	// instead of escalating — a concurrent burst pays one penalty.
	*now = now.Add(100 * time.Millisecond) // still inside the 250ms window
	if err := l.recordFailureAndDelay(ctx, "10.0.0.2"); err != nil {
		t.Fatalf("recordFailureAndDelay: %v", err)
	}
	delay, until2, _ := l.penaltyState("10.0.0.2")

	if delay != 250*time.Millisecond {
		t.Errorf("in-window failure escalated delay to %v, want 250ms", delay)
	}
	if !until2.Equal(until1) {
		t.Errorf("in-window failure moved deadline from %v to %v", until1, until2)
	}
}

func TestAuthFailureLimiter_ResetOnSuccess(t *testing.T) {
	l, now := fakeClockLimiter()
	ctx := context.Background()

	for range 3 {
		if err := l.recordFailureAndDelay(ctx, "10.0.0.3"); err != nil {
			t.Fatalf("recordFailureAndDelay: %v", err)
		}
		_, until, _ := l.penaltyState("10.0.0.3")
		*now = until.Add(time.Millisecond)
	}

	l.reset("10.0.0.3")
	if _, _, ok := l.penaltyState("10.0.0.3"); ok {
		t.Fatal("reset did not clear penalty state")
	}

	// Next failure starts back at the base delay.
	if err := l.recordFailureAndDelay(ctx, "10.0.0.3"); err != nil {
		t.Fatalf("recordFailureAndDelay: %v", err)
	}
	if delay, _, _ := l.penaltyState("10.0.0.3"); delay != 250*time.Millisecond {
		t.Errorf("delay after reset = %v, want 250ms", delay)
	}
}

func TestAuthFailureLimiter_RecordFailureAndDelayAll(t *testing.T) {
	l := newAuthFailureLimiter() // real clock: verifying concurrency needs real timing
	l.baseDelay = 5 * time.Millisecond
	l.maxDelay = 50 * time.Millisecond
	ctx := context.Background()

	// Escalate the IP key alone so its delay (10ms) exceeds the email key's
	// base delay (5ms): the combined wait must be the max, not the sum.
	if err := l.recordFailureAndDelay(ctx, "magic:ip:1.2.3.4"); err != nil {
		t.Fatalf("recordFailureAndDelay: %v", err)
	}

	start := time.Now()
	if err := l.recordFailureAndDelayAll(ctx, "magic:ip:1.2.3.4", "magic:email:a@example.com"); err != nil {
		t.Fatalf("recordFailureAndDelayAll: %v", err)
	}
	elapsed := time.Since(start)

	ipDelay, _, _ := l.penaltyState("magic:ip:1.2.3.4")
	emailDelay, _, _ := l.penaltyState("magic:email:a@example.com")
	if ipDelay <= emailDelay {
		t.Fatalf("test setup: ip delay %v should exceed email delay %v", ipDelay, emailDelay)
	}
	// Sequential waits would take ipDelay+emailDelay; concurrent waits take
	// ~ipDelay. Allow generous slack for scheduler jitter.
	if elapsed > ipDelay+20*time.Millisecond {
		t.Errorf("recordFailureAndDelayAll took %v, want ~%v (max, not sum)", elapsed, ipDelay)
	}
}

func TestAuthFailureLimiter_ContextCancelled(t *testing.T) {
	l := newAuthFailureLimiter() // real clock: the wait is real
	l.baseDelay = 5 * time.Second

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- l.recordFailureAndDelay(ctx, "10.0.0.4") }()

	cancel()
	select {
	case err := <-errCh:
		if err != context.Canceled {
			t.Errorf("recordFailureAndDelay = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("recordFailureAndDelay did not honor context cancellation")
	}
}

func TestAuthFailureLimiter_Prune(t *testing.T) {
	l, now := fakeClockLimiter()
	ctx := context.Background()

	if err := l.recordFailureAndDelay(ctx, "10.0.0.5"); err != nil {
		t.Fatalf("recordFailureAndDelay: %v", err)
	}

	// Idle for longer than twice the max delay: the entry is pruned on the
	// next recordFailure for any key.
	*now = now.Add(2*l.maxDelay + time.Minute)
	if err := l.recordFailureAndDelay(ctx, "10.0.0.6"); err != nil {
		t.Fatalf("recordFailureAndDelay: %v", err)
	}

	if _, _, ok := l.penaltyState("10.0.0.5"); ok {
		t.Error("stale penalty entry was not pruned")
	}
	if _, _, ok := l.penaltyState("10.0.0.6"); !ok {
		t.Error("fresh penalty entry missing")
	}
}

// configurableAgent is an AgentProcessor that also records per-session tool
// override updates.
type configurableAgent struct {
	lastSessionID string
	lastOverrides *sessions.ToolOverrides
	lastModel     string
	lastSticky    bool
	setCalls      int
}

func (a *configurableAgent) Process(ctx context.Context, sessionID, content string) (string, error) {
	return "ok", nil
}

func (a *configurableAgent) SetSessionToolOverrides(ctx context.Context, sessionID string, overrides *sessions.ToolOverrides) error {
	a.setCalls++
	a.lastSessionID = sessionID
	a.lastOverrides = overrides
	return nil
}

func (a *configurableAgent) SetSessionModel(ctx context.Context, sessionID, model string, sticky bool) error {
	a.setCalls++
	a.lastSessionID = sessionID
	a.lastModel = model
	a.lastSticky = sticky
	return nil
}

func TestHandleSessionTools_SetAndClear(t *testing.T) {
	agent := &configurableAgent{}
	gw, err := New(Config{Agent: agent})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	handler := NewDefaultMessageHandler(gw)
	client := newClient(nil, gw)
	ctx := context.Background()

	// Set overrides.
	resp, err := handler.handleSessionTools(ctx, client, &Message{
		ID:   "m1",
		Type: MessageTypeSessionTools,
		Data: map[string]interface{}{
			"tool_overrides": map[string]interface{}{
				"tools":          map[string]interface{}{"web_search": false},
				"mcp_servers":    map[string]interface{}{"github": false},
				"mcp_tools_deny": map[string]interface{}{"jira": []interface{}{"delete_issue"}},
			},
		},
	})
	if err != nil {
		t.Fatalf("handleSessionTools: %v", err)
	}
	if resp.Type != MessageTypeResponse {
		t.Fatalf("response type = %v, want response (data: %v)", resp.Type, resp.Data)
	}
	if agent.setCalls != 1 || agent.lastSessionID != client.ID {
		t.Fatalf("set calls = %d for session %q, want 1 for %q", agent.setCalls, agent.lastSessionID, client.ID)
	}
	ov := agent.lastOverrides
	if ov == nil || ov.Tools["web_search"] || len(ov.MCPServers) != 1 || len(ov.MCPToolsDeny["jira"]) != 1 {
		t.Fatalf("parsed overrides = %+v", ov)
	}

	// Clear overrides with null data.
	resp, err = handler.handleSessionTools(ctx, client, &Message{
		ID:   "m2",
		Type: MessageTypeSessionTools,
		Data: map[string]interface{}{"tool_overrides": nil},
	})
	if err != nil {
		t.Fatalf("handleSessionTools clear: %v", err)
	}
	if resp.Type != MessageTypeResponse {
		t.Fatalf("clear response type = %v, want response", resp.Type)
	}
	if agent.setCalls != 2 || agent.lastOverrides != nil {
		t.Fatalf("clear did not pass nil overrides (calls=%d, overrides=%+v)", agent.setCalls, agent.lastOverrides)
	}
}

func TestHandleSessionModel_SetAndUnsupported(t *testing.T) {
	agent := &configurableAgent{}
	gw, err := New(Config{Agent: agent})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	handler := NewDefaultMessageHandler(gw)
	client := newClient(nil, gw)
	ctx := context.Background()

	resp, err := handler.handleSessionModel(ctx, client, &Message{
		ID:   "m1",
		Type: MessageTypeSessionModel,
		Data: map[string]interface{}{"model": "smart-model", "sticky": true},
	})
	if err != nil {
		t.Fatalf("handleSessionModel: %v", err)
	}
	if resp.Type != MessageTypeResponse {
		t.Fatalf("response type = %v, want response (data: %v)", resp.Type, resp.Data)
	}
	if agent.lastSessionID != client.ID || agent.lastModel != "smart-model" || !agent.lastSticky {
		t.Fatalf("recorded call = session %q model %q sticky %v", agent.lastSessionID, agent.lastModel, agent.lastSticky)
	}

	// Agent without SessionModelConfigurator gets an error response.
	gw2, err := New(Config{Agent: &mockAgent{response: "hi"}})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	handler2 := NewDefaultMessageHandler(gw2)
	resp, err = handler2.handleSessionModel(ctx, newClient(nil, gw2), &Message{
		ID:   "m2",
		Type: MessageTypeSessionModel,
		Data: map[string]interface{}{"model": "x"},
	})
	if err != nil {
		t.Fatalf("handleSessionModel: %v", err)
	}
	if resp.Type != MessageTypeError {
		t.Fatalf("unsupported agent response type = %v, want error", resp.Type)
	}
}

func TestHandleSessionTools_UnsupportedAgent(t *testing.T) {
	// mockAgent implements only AgentProcessor, not SessionToolConfigurator.
	gw, err := New(Config{Agent: &mockAgent{response: "hi"}})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	handler := NewDefaultMessageHandler(gw)
	client := newClient(nil, gw)

	resp, err := handler.handleSessionTools(context.Background(), client, &Message{
		ID:   "m1",
		Type: MessageTypeSessionTools,
		Data: map[string]interface{}{"tool_overrides": map[string]interface{}{}},
	})
	if err != nil {
		t.Fatalf("handleSessionTools: %v", err)
	}
	if resp.Type != MessageTypeError {
		t.Fatalf("response type = %v, want error for unsupported agent", resp.Type)
	}
}

func TestHandleAuth_FailureDelaysAndSuccessResets(t *testing.T) {
	gw, err := New(Config{
		APIKeys: []string{"secret-key"},
		Agent:   nil,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	// Tiny delays so the test doesn't sleep noticeably.
	gw.authLimiter.baseDelay = time.Millisecond
	gw.authLimiter.maxDelay = 4 * time.Millisecond

	handler := NewDefaultMessageHandler(gw)
	client := newClient(nil, gw)
	ctx := context.Background()

	// Failed attempt records a penalty for the client's source key.
	resp, err := handler.handleAuth(ctx, client, &Message{
		ID:   "m1",
		Type: MessageTypeAuth,
		Data: map[string]interface{}{"api_key": "wrong"},
	})
	if err != nil {
		t.Fatalf("handleAuth: %v", err)
	}
	if resp.Type != MessageTypeError {
		t.Fatalf("wrong key: response type = %v, want error", resp.Type)
	}
	if _, _, ok := gw.authLimiter.penaltyState(client.RemoteIP()); !ok {
		t.Fatal("failed auth did not record a penalty")
	}

	// Correct credentials are never delayed and reset the penalty.
	start := time.Now()
	resp, err = handler.handleAuth(ctx, client, &Message{
		ID:   "m2",
		Type: MessageTypeAuth,
		Data: map[string]interface{}{"api_key": "secret-key"},
	})
	if err != nil {
		t.Fatalf("handleAuth: %v", err)
	}
	if resp.Type != MessageTypeResponse {
		t.Fatalf("correct key: response type = %v, want response", resp.Type)
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Errorf("correct credentials were delayed %v", elapsed)
	}
	if _, _, ok := gw.authLimiter.penaltyState(client.RemoteIP()); ok {
		t.Error("successful auth did not reset the penalty state")
	}
}
