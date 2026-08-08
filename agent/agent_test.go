package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/plexusone/omnillm/provider"
	"github.com/plexusone/omnimemory/core"
	_ "github.com/plexusone/omnimemory/provider/kvs" // registers the KVS memory provider
	"github.com/plexusone/omniskill/skill"
	"github.com/plexusone/omnistorage-core/kvs"
	"github.com/plexusone/omnistorage-core/kvs/backend/memory"

	"github.com/plexusone/omniagent/hooks"
	"github.com/plexusone/omniagent/sessions"
)

// newMemoryBackend returns an in-memory KVS backend closed at test cleanup.
func newMemoryBackend(t *testing.T) kvs.Store {
	t.Helper()
	backend := memory.New()
	t.Cleanup(func() { _ = backend.Close() })
	return backend
}

func TestJoinAssistantSegments(t *testing.T) {
	tests := []struct {
		name     string
		segments []string
		want     string
	}{
		{
			name:     "empty",
			segments: nil,
			want:     "",
		},
		{
			name:     "single segment unchanged",
			segments: []string{"The answer is 42."},
			want:     "The answer is 42.",
		},
		{
			name:     "pre-tool text joined with final response",
			segments: []string{"Let me check X.", "The answer is Y."},
			want:     "Let me check X.\n\nThe answer is Y.",
		},
		{
			name:     "three segments across tool turns",
			segments: []string{"First.", "Second.", "Third."},
			want:     "First.\n\nSecond.\n\nThird.",
		},
		{
			name:     "no double spacing when segments already end with newlines",
			segments: []string{"Let me check X.\n\n", "The answer is Y."},
			want:     "Let me check X.\n\nThe answer is Y.",
		},
		{
			name:     "no double spacing when segments begin with newlines",
			segments: []string{"Let me check X.", "\n\nThe answer is Y."},
			want:     "Let me check X.\n\nThe answer is Y.",
		},
		{
			name:     "blank segments skipped",
			segments: []string{"Let me check X.", "   ", "", "The answer is Y."},
			want:     "Let me check X.\n\nThe answer is Y.",
		},
		{
			name:     "internal newlines preserved",
			segments: []string{"Line one.\nLine two.", "Final."},
			want:     "Line one.\nLine two.\n\nFinal.",
		},
		{
			name:     "leading whitespace of first segment preserved",
			segments: []string{"  indented start", "end"},
			want:     "  indented start\n\nend",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := joinAssistantSegments(tt.segments); got != tt.want {
				t.Errorf("joinAssistantSegments(%q) = %q, want %q", tt.segments, got, tt.want)
			}
		})
	}
}

func TestTemporalContext(t *testing.T) {
	// 2026-08-03 01:30 UTC is 2026-08-02 18:30 in Los Angeles — the date
	// must resolve in the configured zone, not the host zone.
	instant := time.Date(2026, 8, 3, 1, 30, 0, 0, time.UTC)

	la, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Fatalf("LoadLocation: %v", err)
	}

	tests := []struct {
		name     string
		loc      *time.Location
		wantDate string
		wantDay  string
		wantZone string
	}{
		{name: "utc", loc: time.UTC, wantDate: "2026-08-03", wantDay: "Monday", wantZone: "UTC"},
		{name: "timezone shifts date", loc: la, wantDate: "2026-08-02", wantDay: "Sunday", wantZone: "America/Los_Angeles"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := temporalContext(instant, tt.loc)
			for _, want := range []string{"## Temporal Context", "Current date: " + tt.wantDate + " (" + tt.wantDay + ")", "Timezone: " + tt.wantZone} {
				if !strings.Contains(got, want) {
					t.Errorf("temporalContext() = %q, missing %q", got, want)
				}
			}
			// Coarse date only — no clock time.
			if strings.Contains(got, ":30") {
				t.Errorf("temporalContext() = %q, must not contain a clock time", got)
			}
		})
	}
}

func TestBuildSystemPrompt_TemporalContext(t *testing.T) {
	a := &Agent{config: Config{SystemPrompt: "You are a helpful assistant."}}

	got := a.buildSystemPromptWithMemories(nil)

	if !strings.HasPrefix(got, "You are a helpful assistant.") {
		t.Errorf("prompt should start with the configured base prompt, got %q", got)
	}
	if !strings.Contains(got, "## Temporal Context") {
		t.Errorf("prompt should contain the temporal context block, got %q", got)
	}
	// The temporal block must be the volatile suffix — after the base prompt.
	if strings.Index(got, "## Temporal Context") < strings.Index(got, "You are a helpful assistant.") {
		t.Errorf("temporal context must come after the stable base prompt, got %q", got)
	}
	// Nil location (zero-value Agent) falls back to UTC.
	if !strings.Contains(got, "Timezone: UTC") {
		t.Errorf("zero-value agent should default to UTC, got %q", got)
	}

	// Empty base prompt still yields date grounding.
	empty := &Agent{}
	if got := empty.buildSystemPromptWithMemories(nil); !strings.HasPrefix(got, "## Temporal Context") {
		t.Errorf("empty base prompt should return the temporal block alone, got %q", got)
	}
}

func TestClassifyAgentEnd(t *testing.T) {
	runErr := errors.New("provider exploded")

	tests := []struct {
		name        string
		ctxErr      error
		runErr      error
		wantSuccess bool
		wantErr     string
		wantAborted bool
	}{
		{
			name:        "normal completion",
			wantSuccess: true,
		},
		{
			name:    "run error",
			runErr:  runErr,
			wantErr: "provider exploded",
		},
		{
			name:        "abort via context cancellation",
			ctxErr:      context.Canceled,
			runErr:      fmt.Errorf("chat completion: %w", context.Canceled),
			wantAborted: true,
			wantErr:     "", // abort is not an error
		},
		{
			name:        "abort outranks a concurrent error",
			ctxErr:      context.Canceled,
			runErr:      runErr,
			wantAborted: true,
			wantErr:     "",
		},
		{
			name:        "timeout while aborting is not stamped as error",
			ctxErr:      context.Canceled,
			runErr:      fmt.Errorf("chat completion: %w", context.DeadlineExceeded),
			wantAborted: true,
			wantErr:     "",
		},
		{
			name:    "genuine deadline exceeded is an error",
			ctxErr:  context.DeadlineExceeded,
			runErr:  fmt.Errorf("chat completion: %w", context.DeadlineExceeded),
			wantErr: "chat completion: context deadline exceeded",
		},
		{
			name:        "successful return after late cancellation is still success",
			ctxErr:      context.Canceled,
			wantSuccess: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			success, errMsg, aborted := classifyAgentEnd(tt.ctxErr, tt.runErr)
			if success != tt.wantSuccess {
				t.Errorf("success = %v, want %v", success, tt.wantSuccess)
			}
			if errMsg != tt.wantErr {
				t.Errorf("errMsg = %q, want %q", errMsg, tt.wantErr)
			}
			if aborted != tt.wantAborted {
				t.Errorf("aborted = %v, want %v", aborted, tt.wantAborted)
			}
		})
	}
}

func TestEmitAgentEnd_FiresOnAbortedContext(t *testing.T) {
	registry := hooks.NewRegistry()
	received := make(chan hooks.Event, 1)
	registry.RegisterHandler(hooks.EventAgentEnd, "capture", func(ctx context.Context, e hooks.Event) error {
		received <- e
		return nil
	})

	a := &Agent{dispatcher: hooks.NewDispatcher(registry)}

	// Emit from an already-cancelled context — the event must still deliver.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	a.emitAgentEnd(ctx, nil, "", fmt.Errorf("chat completion: %w", context.Canceled), 42*time.Millisecond)

	select {
	case e := <-received:
		data, ok := e.Data.(hooks.AgentEndEvent)
		if !ok {
			t.Fatalf("event data type = %T, want AgentEndEvent", e.Data)
		}
		if !data.Aborted {
			t.Error("aborted run must report Aborted=true")
		}
		if data.Success {
			t.Error("aborted run must report Success=false")
		}
		if data.Error != "" {
			t.Errorf("aborted run must report empty Error, got %q", data.Error)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("agent.end was not delivered for an aborted run")
	}
}

func TestEmitAgentEnd_NilDispatcherSafe(t *testing.T) {
	a := &Agent{}
	// Must not panic.
	a.emitAgentEnd(context.Background(), nil, "ok", nil, time.Millisecond)
}

func namedTools(names ...string) []provider.Tool {
	tools := make([]provider.Tool, len(names))
	for i, name := range names {
		tools[i] = provider.Tool{
			Type:     "function",
			Function: provider.ToolSpec{Name: name},
		}
	}
	return tools
}

func toolNames(tools []provider.Tool) []string {
	names := make([]string, len(tools))
	for i, t := range tools {
		names[i] = t.Function.Name
	}
	return names
}

func TestNarrowToolsForTurn(t *testing.T) {
	ctx := context.Background()
	all := namedTools("chart", "search", "memory")

	t.Run("no hooks leaves tools unchanged", func(t *testing.T) {
		a := &Agent{}
		got := a.narrowToolsForTurn(ctx, "", "hi", 0, all)
		if len(got) != 3 {
			t.Errorf("got %d tools, want 3", len(got))
		}
	})

	t.Run("nil return leaves tools unchanged", func(t *testing.T) {
		a := &Agent{toolsAllowHooks: []hooks.ToolsAllowFunc{
			func(ctx context.Context, turn hooks.PromptTurn) []string { return nil },
		}}
		got := a.narrowToolsForTurn(ctx, "", "hi", 0, all)
		if len(got) != 3 {
			t.Errorf("got %d tools, want 3", len(got))
		}
	})

	t.Run("allow list narrows to intersection", func(t *testing.T) {
		a := &Agent{toolsAllowHooks: []hooks.ToolsAllowFunc{
			func(ctx context.Context, turn hooks.PromptTurn) []string {
				return []string{"chart", "not-registered"}
			},
		}}
		got := a.narrowToolsForTurn(ctx, "", "hi", 0, all)
		if len(got) != 1 || got[0].Function.Name != "chart" {
			t.Errorf("got %v, want [chart]", toolNames(got))
		}
	})

	t.Run("empty return removes all tools", func(t *testing.T) {
		a := &Agent{toolsAllowHooks: []hooks.ToolsAllowFunc{
			func(ctx context.Context, turn hooks.PromptTurn) []string { return []string{} },
		}}
		got := a.narrowToolsForTurn(ctx, "", "hi", 0, all)
		if len(got) != 0 {
			t.Errorf("got %v, want no tools", toolNames(got))
		}
	})

	t.Run("hooks compose by intersection", func(t *testing.T) {
		a := &Agent{toolsAllowHooks: []hooks.ToolsAllowFunc{
			func(ctx context.Context, turn hooks.PromptTurn) []string {
				return []string{"chart", "search"}
			},
			func(ctx context.Context, turn hooks.PromptTurn) []string {
				// Second hook only sees survivors of the first.
				if len(turn.Tools) != 2 {
					t.Errorf("second hook saw %v, want 2 tools", turn.Tools)
				}
				return []string{"search", "memory"}
			},
		}}
		got := a.narrowToolsForTurn(ctx, "", "hi", 0, all)
		if len(got) != 1 || got[0].Function.Name != "search" {
			t.Errorf("got %v, want [search]", toolNames(got))
		}
	})

	t.Run("narrowing is per-iteration and can widen later", func(t *testing.T) {
		a := &Agent{toolsAllowHooks: []hooks.ToolsAllowFunc{
			func(ctx context.Context, turn hooks.PromptTurn) []string {
				if turn.Iteration == 0 {
					return []string{"chart"}
				}
				return nil // later iterations unchanged
			},
		}}
		if got := a.narrowToolsForTurn(ctx, "", "hi", 0, all); len(got) != 1 {
			t.Errorf("iteration 0: got %v, want [chart]", toolNames(got))
		}
		if got := a.narrowToolsForTurn(ctx, "", "hi", 1, all); len(got) != 3 {
			t.Errorf("iteration 1: got %v, want all 3 tools", toolNames(got))
		}
	})

	t.Run("hook receives turn context", func(t *testing.T) {
		var seen hooks.PromptTurn
		a := &Agent{toolsAllowHooks: []hooks.ToolsAllowFunc{
			func(ctx context.Context, turn hooks.PromptTurn) []string {
				seen = turn
				return nil
			},
		}}
		a.narrowToolsForTurn(ctx, "sess-1", "draw a chart", 2, all)
		if seen.SessionID != "sess-1" || seen.Content != "draw a chart" || seen.Iteration != 2 || len(seen.Tools) != 3 {
			t.Errorf("hook saw %+v", seen)
		}
	})
}

func TestFilterToolsByAllow_PreservesOrder(t *testing.T) {
	all := namedTools("a", "b", "c", "d")
	got := filterToolsByAllow(all, []string{"d", "b"})
	if len(got) != 2 || got[0].Function.Name != "b" || got[1].Function.Name != "d" {
		t.Errorf("got %v, want [b d] in original order", toolNames(got))
	}
}

// mcpLikeSkill is a compiled.Skill that declares an MCP source type.
type mcpLikeSkill struct {
	name  string
	tools []skill.Tool
}

func (s *mcpLikeSkill) Name() string                   { return s.name }
func (s *mcpLikeSkill) Description() string            { return "mcp-backed test skill" }
func (s *mcpLikeSkill) Tools() []skill.Tool            { return s.tools }
func (s *mcpLikeSkill) Init(ctx context.Context) error { return nil }
func (s *mcpLikeSkill) Close() error                   { return nil }
func (s *mcpLikeSkill) SourceType() string             { return "mcp" }

// overridesAgent builds an agent with one plain tool (web_search) and two
// MCP tools from the "github" server (search_issues, create_issue).
func overridesAgent(t *testing.T) *Agent {
	t.Helper()
	a := &Agent{tools: NewToolRegistry(), logger: slog.Default()}

	a.tools.Register(NewBaseTool("web_search", "Search the web", nil,
		func(ctx context.Context, args json.RawMessage) (string, error) { return "ok", nil }))

	mcpSkill := &mcpLikeSkill{
		name: "github",
		tools: []skill.Tool{
			skill.NewTool("search_issues", "Search issues", nil,
				func(ctx context.Context, params map[string]any) (any, error) { return "ok", nil }),
			skill.NewTool("create_issue", "Create issue", nil,
				func(ctx context.Context, params map[string]any) (any, error) { return "ok", nil }),
		},
	}
	if err := a.RegisterCompiledSkill(mcpSkill); err != nil {
		t.Fatalf("RegisterCompiledSkill: %v", err)
	}
	return a
}

func sessionWithOverrides(id string, ov *sessions.ToolOverrides) *sessions.Session {
	s := sessions.NewSession(id)
	s.ToolOverrides = ov
	return s
}

func toolNameSet(tools []provider.Tool) map[string]bool {
	set := make(map[string]bool, len(tools))
	for _, t := range tools {
		set[t.Function.Name] = true
	}
	return set
}

func TestFilterToolsForSession(t *testing.T) {
	a := overridesAgent(t)
	all := a.tools.GetTools()
	if len(all) != 3 {
		t.Fatalf("setup: got %d tools, want 3", len(all))
	}

	t.Run("nil session unchanged", func(t *testing.T) {
		if got := a.filterToolsForSession(nil, all); len(got) != 3 {
			t.Errorf("got %d tools, want 3", len(got))
		}
	})

	t.Run("nil overrides unchanged", func(t *testing.T) {
		if got := a.filterToolsForSession(sessionWithOverrides("s", nil), all); len(got) != 3 {
			t.Errorf("got %d tools, want 3", len(got))
		}
	})

	t.Run("disabled tool removed", func(t *testing.T) {
		s := sessionWithOverrides("s", &sessions.ToolOverrides{
			Tools: map[string]bool{"web_search": false},
		})
		got := toolNameSet(a.filterToolsForSession(s, all))
		if got["web_search"] || !got["search_issues"] || !got["create_issue"] {
			t.Errorf("got %v, want github tools only", got)
		}
	})

	t.Run("disabled MCP server removes all its tools", func(t *testing.T) {
		s := sessionWithOverrides("s", &sessions.ToolOverrides{
			MCPServers: map[string]bool{"github": false},
		})
		got := toolNameSet(a.filterToolsForSession(s, all))
		if !got["web_search"] || got["search_issues"] || got["create_issue"] {
			t.Errorf("got %v, want web_search only", got)
		}
	})

	t.Run("per-server deny removes one MCP tool", func(t *testing.T) {
		s := sessionWithOverrides("s", &sessions.ToolOverrides{
			MCPToolsDeny: map[string][]string{"github": {"create_issue"}},
		})
		got := toolNameSet(a.filterToolsForSession(s, all))
		if !got["web_search"] || !got["search_issues"] || got["create_issue"] {
			t.Errorf("got %v, want create_issue removed only", got)
		}
	})

	t.Run("concurrent sessions get independent tool sets", func(t *testing.T) {
		s1 := sessionWithOverrides("s1", &sessions.ToolOverrides{
			Tools: map[string]bool{"web_search": false},
		})
		s2 := sessionWithOverrides("s2", &sessions.ToolOverrides{
			MCPServers: map[string]bool{"github": false},
		})

		got1 := toolNameSet(a.filterToolsForSession(s1, all))
		got2 := toolNameSet(a.filterToolsForSession(s2, all))

		if got1["web_search"] || !got1["search_issues"] {
			t.Errorf("session 1 got %v", got1)
		}
		if !got2["web_search"] || got2["search_issues"] {
			t.Errorf("session 2 got %v", got2)
		}
		// The shared source slice is untouched.
		if len(all) != 3 {
			t.Errorf("source slice mutated: %d tools", len(all))
		}
	})
}

// rolloverTestAgent builds an agent with a session store, a rollover policy,
// and a capture handler on the session.rollover event.
func rolloverTestAgent(t *testing.T, policy sessions.RolloverPolicy) (*Agent, *sessions.Store, chan hooks.Event) {
	t.Helper()

	store := sessions.NewStore(sessions.StoreConfig{Backend: newMemoryBackend(t)})
	registry := hooks.NewRegistry()
	received := make(chan hooks.Event, 4)
	registry.RegisterHandler(hooks.EventSessionRollover, "capture", func(ctx context.Context, e hooks.Event) error {
		received <- e
		return nil
	})

	a := &Agent{
		sessions:       store,
		hooks:          registry,
		dispatcher:     hooks.NewDispatcher(registry),
		logger:         slog.Default(),
		rolloverPolicy: &policy,
	}
	return a, store, received
}

func TestMaybeRolloverSession(t *testing.T) {
	ctx := context.Background()

	t.Run("idle session rolls over exactly once with transcript", func(t *testing.T) {
		a, store, received := rolloverTestAgent(t, sessions.RolloverPolicy{IdleTimeout: time.Hour})

		session, err := store.Get(ctx, "roll-1")
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		session.AddMessage(provider.RoleUser, "remember the plan")
		session.AddMessage(provider.RoleAssistant, "noted")
		session.UpdatedAt = time.Now().Add(-2 * time.Hour)
		if err := store.Save(ctx, session); err != nil {
			t.Fatalf("Save: %v", err)
		}

		a.maybeRolloverSession(ctx, session)

		select {
		case e := <-received:
			data, ok := e.Data.(hooks.SessionRolloverEvent)
			if !ok {
				t.Fatalf("event data type = %T", e.Data)
			}
			if data.Reason != string(sessions.RolloverReasonIdle) {
				t.Errorf("reason = %q, want idle", data.Reason)
			}
			if data.SessionID != "roll-1" {
				t.Errorf("session id = %q", data.SessionID)
			}
			if len(data.Transcript) != 2 || data.Transcript[0].Content != "remember the plan" {
				t.Errorf("transcript = %+v", data.Transcript)
			}
		default:
			t.Fatal("rollover event was not emitted")
		}

		// The session was reset in place.
		if len(session.Messages) != 0 {
			t.Errorf("session retained %d messages after rollover", len(session.Messages))
		}

		// A second check on the freshly reset session emits nothing.
		a.maybeRolloverSession(ctx, session)
		select {
		case <-received:
			t.Fatal("reset session must not roll over again")
		default:
		}
	})

	t.Run("active session does not roll over", func(t *testing.T) {
		a, store, received := rolloverTestAgent(t, sessions.RolloverPolicy{IdleTimeout: time.Hour})

		session, err := store.Get(ctx, "roll-2")
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		session.AddMessage(provider.RoleUser, "hello")

		a.maybeRolloverSession(ctx, session)
		select {
		case <-received:
			t.Fatal("active session must not roll over")
		default:
		}
		if len(session.Messages) != 1 {
			t.Errorf("active session was reset")
		}
	})

	t.Run("nil policy is a no-op", func(t *testing.T) {
		a, store, received := rolloverTestAgent(t, sessions.RolloverPolicy{IdleTimeout: time.Hour})
		a.rolloverPolicy = nil

		session, err := store.Get(ctx, "roll-3")
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		session.AddMessage(provider.RoleUser, "hello")
		session.UpdatedAt = time.Now().Add(-48 * time.Hour)

		a.maybeRolloverSession(ctx, session)
		select {
		case <-received:
			t.Fatal("nil policy must not roll over")
		default:
		}
	})
}

func TestRollover_WritesExactlyOneMemory(t *testing.T) {
	ctx := context.Background()

	memClient, err := core.NewClient(core.ClientConfig{
		Providers: []core.ProviderConfig{{
			Name:    core.ProviderNameKVS,
			Options: map[string]any{"store": memory.New()},
		}},
	})
	if err != nil {
		t.Fatalf("core.NewClient: %v", err)
	}
	defer memClient.Close()

	a, store, _ := rolloverTestAgent(t, sessions.RolloverPolicy{IdleTimeout: time.Hour})
	a.memory = memClient
	a.config.TenantID = "tenant-1"
	a.config.AgentID = "agent-1"
	// Register the built-in persistence hook (New() does this for
	// constructed agents; this test builds the agent by hand).
	a.hooks.RegisterHandler(hooks.EventSessionRollover, "session-memory", a.saveRolloverMemory)

	session, err := store.Get(ctx, "roll-mem")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	session.AddMessage(provider.RoleUser, "remember: ship v0.13 on Friday")
	session.AddMessage(provider.RoleAssistant, "will do")
	session.UpdatedAt = time.Now().Add(-2 * time.Hour)

	a.maybeRolloverSession(ctx, session)

	list, err := memClient.List(ctx, &core.ListRequest{
		Context: core.Context{
			TenantID:  "tenant-1",
			SubjectID: "roll-mem",
			AgentID:   "agent-1",
			SessionID: "roll-mem",
		},
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list.Memories) != 1 {
		t.Fatalf("got %d memories after rollover, want exactly 1", len(list.Memories))
	}

	mem := list.Memories[0]
	if mem.Type != core.MemoryTypeObservation {
		t.Errorf("memory type = %q, want observation", mem.Type)
	}
	if !strings.Contains(mem.Content, "ship v0.13 on Friday") {
		t.Errorf("memory content missing conversation, got %q", mem.Content)
	}
	if !strings.Contains(mem.Content, "(idle)") {
		t.Errorf("memory content missing rollover reason, got %q", mem.Content)
	}
	if mem.Metadata["reason"] != "idle" || mem.Metadata["source"] != "session_rollover" {
		t.Errorf("memory metadata = %v", mem.Metadata)
	}

	// A second rollover check on the reset session writes nothing more.
	a.maybeRolloverSession(ctx, session)
	list, err = memClient.List(ctx, &core.ListRequest{
		Context: core.Context{
			TenantID: "tenant-1", SubjectID: "roll-mem", AgentID: "agent-1", SessionID: "roll-mem",
		},
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list.Memories) != 1 {
		t.Fatalf("got %d memories after no-op check, want still 1", len(list.Memories))
	}
}

func TestSetSessionModel(t *testing.T) {
	ctx := context.Background()
	store := sessions.NewStore(sessions.StoreConfig{Backend: newMemoryBackend(t)})
	a := &Agent{sessions: store, logger: slog.Default()}
	a.config.Model = "default-model"

	session, err := store.Get(ctx, "model-sess")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	// Default resolution.
	if got := a.modelForSession(session); got != "default-model" {
		t.Errorf("modelForSession = %q, want default-model", got)
	}
	if got := a.modelForSession(nil); got != "default-model" {
		t.Errorf("modelForSession(nil) = %q, want default-model", got)
	}

	// Non-sticky override applies to the session only.
	if err := a.SetSessionModel(ctx, "model-sess", "fast-model", false); err != nil {
		t.Fatalf("SetSessionModel: %v", err)
	}
	session, err = store.Get(ctx, "model-sess")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got := a.modelForSession(session); got != "fast-model" {
		t.Errorf("modelForSession = %q, want fast-model", got)
	}
	if got := a.modelForSession(nil); got != "default-model" {
		t.Errorf("agent default changed by non-sticky override: %q", got)
	}

	// Override survives a store round trip.
	store.ClearCache()
	session, err = store.GetIfExists(ctx, "model-sess")
	if err != nil {
		t.Fatalf("GetIfExists: %v", err)
	}
	if session.Model != "fast-model" {
		t.Errorf("session model did not persist, got %q", session.Model)
	}

	// Sticky updates the agent default so new sessions inherit it.
	if err := a.SetSessionModel(ctx, "model-sess", "smart-model", true); err != nil {
		t.Fatalf("SetSessionModel sticky: %v", err)
	}
	if got := a.modelForSession(nil); got != "smart-model" {
		t.Errorf("sticky change did not update agent default: %q", got)
	}

	// Clearing the override falls back to the (updated) default.
	if err := a.SetSessionModel(ctx, "model-sess", "", false); err != nil {
		t.Fatalf("SetSessionModel clear: %v", err)
	}
	session, err = store.Get(ctx, "model-sess")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got := a.modelForSession(session); got != "smart-model" {
		t.Errorf("cleared session resolves %q, want smart-model", got)
	}

	// No session store: error.
	noStore := &Agent{logger: slog.Default()}
	if err := noStore.SetSessionModel(ctx, "x", "m", false); err == nil {
		t.Error("SetSessionModel without store must error")
	}
}

func TestResolvePrincipal(t *testing.T) {
	ctx := context.Background()
	store := sessions.NewStore(sessions.StoreConfig{Backend: newMemoryBackend(t)})
	a := &Agent{sessions: store, logger: slog.Default()}

	// A live session's principal resolves.
	if _, err := store.Get(ctx, "alive"); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !a.ResolvePrincipal(ctx, "session:alive") {
		t.Error("live session principal must resolve")
	}

	// A deleted session's principal is denied.
	if err := store.Delete(ctx, "alive"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if a.ResolvePrincipal(ctx, "session:alive") {
		t.Error("deleted session principal must be denied")
	}

	// Unknown forms and missing store are fail-closed.
	if a.ResolvePrincipal(ctx, "account:whoever") {
		t.Error("non-session principal must be denied")
	}
	if a.ResolvePrincipal(ctx, "") {
		t.Error("empty principal must be denied")
	}
	noStore := &Agent{logger: slog.Default()}
	if noStore.ResolvePrincipal(ctx, "session:alive") {
		t.Error("agent without session store must deny")
	}
}

func TestFormatRolloverMemory(t *testing.T) {
	la, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Fatalf("LoadLocation: %v", err)
	}

	data := hooks.SessionRolloverEvent{
		SessionID: "s-9",
		Reason:    "daily",
		Transcript: []hooks.MessageEvent{
			{Role: "user", Content: "what's the plan?"},
			{Role: "assistant", Content: "ship it"},
		},
		// 2026-08-03 01:30 UTC is 2026-08-02 in LA.
		EndedAt: time.Date(2026, 8, 3, 1, 30, 0, 0, time.UTC),
	}

	got := formatRolloverMemory(data, la)
	for _, want := range []string{"s-9", "(daily)", "2026-08-02", "user: what's the plan?", "assistant: ship it"} {
		if !strings.Contains(got, want) {
			t.Errorf("formatRolloverMemory missing %q in %q", want, got)
		}
	}
	if strings.Contains(got, "2026-08-03") {
		t.Error("date must resolve in the user's timezone, not UTC")
	}

	t.Run("caps transcript length", func(t *testing.T) {
		long := data
		long.Transcript = nil
		for i := 0; i < 50; i++ {
			long.Transcript = append(long.Transcript, hooks.MessageEvent{
				Role: "user", Content: fmt.Sprintf("message %d", i),
			})
		}
		got := formatRolloverMemory(long, time.UTC)
		if !strings.Contains(got, "last 20 of 50 messages") {
			t.Errorf("expected cap notice, got %q", got)
		}
		if strings.Contains(got, "message 29") || !strings.Contains(got, "message 30") {
			t.Error("cap must keep the most recent messages")
		}
	})

	t.Run("truncates long messages on rune boundary", func(t *testing.T) {
		long := data
		long.Transcript = []hooks.MessageEvent{
			{Role: "user", Content: strings.Repeat("é", 400)}, // 800 bytes of 2-byte runes
		}
		got := formatRolloverMemory(long, time.UTC)
		if strings.Contains(got, "�") {
			t.Error("truncation split a multi-byte rune")
		}
		if !strings.Contains(got, "…") {
			t.Error("expected truncation marker")
		}
	})
}

func TestToolRegistry_Describe_MCPIdentity(t *testing.T) {
	a := &Agent{tools: NewToolRegistry(), logger: slog.Default()}

	mcpSkill := &mcpLikeSkill{
		name: "github",
		tools: []skill.Tool{
			skill.NewTool("search_issues", "Search issues", nil,
				func(ctx context.Context, params map[string]any) (any, error) { return "ok", nil }),
		},
	}
	if err := a.RegisterCompiledSkill(mcpSkill); err != nil {
		t.Fatalf("RegisterCompiledSkill: %v", err)
	}

	descriptors := a.tools.Describe()
	if len(descriptors) != 1 {
		t.Fatalf("got %d descriptors, want 1", len(descriptors))
	}
	d := descriptors[0]
	if d.Name != "search_issues" {
		t.Errorf("Name = %q, want search_issues", d.Name)
	}
	if d.Source != "mcp" {
		t.Errorf("Source = %q, want mcp", d.Source)
	}
	if d.SourceName != "github" {
		t.Errorf("SourceName = %q, want github", d.SourceName)
	}
	if d.SourceTool != "search_issues" {
		t.Errorf("SourceTool = %q, want search_issues", d.SourceTool)
	}
}
