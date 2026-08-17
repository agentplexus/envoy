package agent

import (
	"context"
	"errors"
	"testing"

	"github.com/plexusone/omnimemory/core"
	_ "github.com/plexusone/omnimemory/provider/kvs" // registers the KVS memory provider
	"github.com/plexusone/omnistorage-core/kvs/backend/memory"

	"github.com/plexusone/omniagent/sessions"
)

// TestNew_Success constructs a real Agent via the public constructor, using
// a provider that only validates its API key locally (no network call at
// construction time), so New's full happy path — client creation, timezone
// resolution, option application, skill-manager init, and the built-in
// rollover-persistence hook registration — runs for real.
func TestNew_Success(t *testing.T) {
	a, err := New(Config{
		Provider: "openai",
		APIKey:   "test-key",
		Model:    "gpt-test",
		Timezone: "America/Los_Angeles",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() {
		if err := a.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	}()

	if a.timezone().String() != "America/Los_Angeles" {
		t.Errorf("timezone = %q, want America/Los_Angeles", a.timezone().String())
	}
	if a.tools == nil {
		t.Error("tool registry not initialized")
	}
	if a.hooks == nil || a.dispatcher == nil {
		t.Error("hook registry/dispatcher not initialized")
	}
}

// TestNew_DefaultsToUTC verifies an empty Timezone resolves to UTC rather
// than erroring or leaving the location nil.
func TestNew_DefaultsToUTC(t *testing.T) {
	a, err := New(Config{Provider: "openai", APIKey: "test-key"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer a.Close()

	if a.timezone().String() != "UTC" {
		t.Errorf("timezone = %q, want UTC", a.timezone().String())
	}
}

// TestNew_InvalidTimezone verifies a bad IANA zone name fails construction
// (rather than silently degrading to UTC) and that the partially-built LLM
// client is closed on that path.
func TestNew_InvalidTimezone(t *testing.T) {
	_, err := New(Config{
		Provider: "openai",
		APIKey:   "test-key",
		Timezone: "Not/A_Zone",
	})
	if err == nil {
		t.Fatal("expected an error for an invalid timezone")
	}
}

// TestNew_OptionError verifies a failing Option aborts construction and
// surfaces the option's error, wrapped.
func TestNew_OptionError(t *testing.T) {
	boom := errors.New("option boom")
	_, err := New(Config{Provider: "openai", APIKey: "test-key"}, func(a *Agent) error {
		return boom
	})
	if err == nil || !errors.Is(err, boom) {
		t.Fatalf("New() error = %v, want to wrap %v", err, boom)
	}
}

// TestNew_InvalidProvider exercises the primary-provider construction
// failure path (empty API key is rejected locally, no network involved).
func TestNew_InvalidProvider(t *testing.T) {
	_, err := New(Config{Provider: "openai", APIKey: ""})
	if err == nil {
		t.Fatal("expected an error for an empty API key")
	}
}

func TestClose_ClosesResources(t *testing.T) {
	a, err := New(Config{Provider: "openai", APIKey: "test-key"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	backend := newMemoryBackend(t)
	a.SetStorage(backend)

	memClient, err := core.NewClient(core.ClientConfig{
		Providers: []core.ProviderConfig{{
			Name:    core.ProviderNameKVS,
			Options: map[string]any{"store": memory.New()},
		}},
	})
	if err != nil {
		t.Fatalf("core.NewClient: %v", err)
	}
	a.SetMemory(memClient)

	if err := a.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestAgent_ToolsAndRegisterTool(t *testing.T) {
	a := &Agent{tools: NewToolRegistry()}
	tool := NewBaseTool("t1", "desc", nil, nil)
	a.RegisterTool(tool)

	if got, ok := a.Tools().Get("t1"); !ok || got.Name() != "t1" {
		t.Errorf("RegisterTool/Tools() did not round-trip the tool")
	}
}

func TestAgent_MemoryGetSet(t *testing.T) {
	a := &Agent{}
	if a.Memory() != nil {
		t.Error("Memory() should be nil by default")
	}
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

	a.SetMemory(memClient)
	if a.Memory() != memClient {
		t.Error("SetMemory did not take effect")
	}
}

// TestAgent_GetSession_MissingSessionErrors covers GetSession's delegation
// to the store's GetIfExists for a session that was never created.
//
// NOTE (reported, not fixed — out of scope for this RMI): GetSession's doc
// comment claims it "returns nil if sessions are not configured or session
// doesn't exist", but sessions.Store.GetIfExists actually returns
// sessions.ErrSessionNotFound (not a nil, nil result) for a missing
// session, so GetSession does too. The doc comment appears to be stale.
func TestAgent_GetSession_MissingSessionErrors(t *testing.T) {
	ctx := context.Background()
	store := sessions.NewStore(sessions.StoreConfig{Backend: newMemoryBackend(t)})
	a := &Agent{sessions: store}

	_, err := a.GetSession(ctx, "never-created")
	if !errors.Is(err, sessions.ErrSessionNotFound) {
		t.Errorf("GetSession(missing) error = %v, want sessions.ErrSessionNotFound", err)
	}
}

func TestAgent_ClearSession_MissingSessionErrors(t *testing.T) {
	store := sessions.NewStore(sessions.StoreConfig{Backend: newMemoryBackend(t)})
	a := &Agent{sessions: store}
	if err := a.ClearSession(context.Background(), "nope"); err == nil {
		t.Error("ClearSession on a nonexistent session should error")
	}
}

func TestAgent_SetSessionToolOverrides(t *testing.T) {
	ctx := context.Background()
	store := sessions.NewStore(sessions.StoreConfig{Backend: newMemoryBackend(t)})
	a := &Agent{sessions: store}

	ov := &sessions.ToolOverrides{Tools: map[string]bool{"web_search": false}}
	if err := a.SetSessionToolOverrides(ctx, "s1", ov); err != nil {
		t.Fatalf("SetSessionToolOverrides: %v", err)
	}

	session, err := store.GetIfExists(ctx, "s1")
	if err != nil {
		t.Fatalf("GetIfExists: %v", err)
	}
	if session.ToolOverrides == nil || session.ToolOverrides.Tools["web_search"] != false {
		t.Errorf("ToolOverrides = %+v, want the persisted override", session.ToolOverrides)
	}

	// Clearing with nil removes the override.
	if err := a.SetSessionToolOverrides(ctx, "s1", nil); err != nil {
		t.Fatalf("SetSessionToolOverrides(nil): %v", err)
	}
	session, err = store.GetIfExists(ctx, "s1")
	if err != nil {
		t.Fatalf("GetIfExists: %v", err)
	}
	if session.ToolOverrides != nil {
		t.Errorf("ToolOverrides = %+v, want nil after clearing", session.ToolOverrides)
	}
}

func TestAgent_SetSessionToolOverrides_NoStoreConfigured(t *testing.T) {
	a := &Agent{}
	if err := a.SetSessionToolOverrides(context.Background(), "x", nil); err == nil {
		t.Error("SetSessionToolOverrides should error without a session store")
	}
}
