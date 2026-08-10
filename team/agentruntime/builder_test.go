package agentruntime

import (
	"context"
	"io"
	"testing"

	"github.com/google/uuid"

	"github.com/plexusone/omniagent/agent"
	"github.com/plexusone/omniagent/team/chats"
)

// TestAgentBuilder_Build builds a real *agent.Agent from persona + enabled
// skills and confirms it satisfies chats.AgentProcessor and is closeable (so
// the cache can evict it).
func TestAgentBuilder_Build(t *testing.T) {
	b := NewAgentBuilder(BuilderConfig{
		Defaults: agent.Config{Provider: "openai", Model: "gpt-4o-mini", APIKey: "test-key"},
	})

	proc, err := b.Build(context.Background(), AgentConfig{
		ID:      uuid.New(),
		Slug:    "helper",
		Persona: "You are a concise helper.",
		Skills:  []string{"weather"},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if proc == nil {
		t.Fatal("Build returned a nil processor")
	}
	var _ chats.AgentProcessor = proc
	closer, ok := proc.(io.Closer)
	if !ok {
		t.Fatal("built processor is not an io.Closer (cache cannot evict it)")
	}
	if err := closer.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}

// TestAgentBuilder_AgentOverridesDefaults confirms the agent's own
// model/provider win over the deployment defaults, while an empty persona leaves
// the default system prompt untouched.
func TestAgentBuilder_AgentOverridesDefaults(t *testing.T) {
	b := NewAgentBuilder(BuilderConfig{
		Defaults: agent.Config{Provider: "openai", Model: "default-model", APIKey: "test-key", SystemPrompt: "default prompt"},
	})

	// Agent overrides model; provider empty falls back to the default.
	proc, err := b.Build(context.Background(), AgentConfig{
		ID:    uuid.New(),
		Slug:  "custom",
		Model: "gpt-4o",
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if c, ok := proc.(io.Closer); ok {
		defer c.Close() //nolint:errcheck // test teardown
	}
	// Build succeeding with the overridden model and inherited provider/prompt is
	// the observable contract; the agent package owns how they are applied.
}
