package chats

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/plexusone/omniagent/team/store"
)

// fakeRuntime is a stub AgentRuntime (RMI-113): it returns a fixed slug and
// processor and records how many times each was resolved, so tests can assert
// the mention policy resolves the slug for groups and the processor only when
// the agent actually takes a turn.
type fakeRuntime struct {
	slug      string
	proc      AgentProcessor
	slugCalls int
	procCalls int
}

func (r *fakeRuntime) Slug(_ context.Context, _ uuid.UUID) (string, error) {
	r.slugCalls++
	return r.slug, nil
}

func (r *fakeRuntime) Processor(_ context.Context, _ uuid.UUID) (AgentProcessor, error) {
	r.procCalls++
	return r.proc, nil
}

// setupTurn wires a chats service with an allow-all gate (so agent-bound chats
// can be created) and the given runtime, returning the service, store, and the
// seeded user.
func setupTurn(t *testing.T, rt AgentRuntime) (*Service, *store.Store, uuid.UUID) {
	t.Helper()
	st, userID := openTestStore(t)
	svc, err := NewService(st, Config{Agents: &fakeGate{allow: true}, Runtime: rt})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	return svc, st, userID
}

func TestMentionsAgent(t *testing.T) {
	cases := []struct {
		name    string
		content string
		slug    string
		want    bool
	}{
		{"leading mention", "@helper what's up", "helper", true},
		{"mid-sentence", "hey @helper can you", "helper", true},
		{"trailing punctuation", "ping @helper!", "helper", true},
		{"case-insensitive", "@Helper hi", "helper", true},
		{"hyphenated slug", "@code-bot run it", "code-bot", true},
		{"underscore slug", "@code_bot go", "code_bot", true},
		{"prefix is not a match", "@helperbot hi", "helper", false},
		{"email is not a mention", "mail me at me@helper", "helper", false},
		{"no at sign", "helper please", "helper", false},
		{"different slug", "@other hi", "helper", false},
		{"empty slug never matches", "@ hi", "", false},
		{"bare at", "just @ here", "helper", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := mentionsAgent(tc.content, tc.slug); got != tc.want {
				t.Errorf("mentionsAgent(%q, %q) = %v, want %v", tc.content, tc.slug, got, tc.want)
			}
		})
	}
}

func TestAgentTurn_PrivateAlwaysResponds(t *testing.T) {
	rt := &fakeRuntime{slug: "helper", proc: &echoAgent{reply: "hi back"}}
	svc, st, userID := setupTurn(t, rt)
	ctx := context.Background()
	agentID := seedAgent(t, st, userID, "helper")

	c, err := svc.StartAgentDM(ctx, Actor{UserID: userID}, agentID)
	if err != nil {
		t.Fatalf("StartAgentDM: %v", err)
	}
	msg, responded, err := svc.AgentTurn(ctx, c, "anything at all")
	if err != nil {
		t.Fatalf("AgentTurn: %v", err)
	}
	if !responded {
		t.Fatal("private chat did not respond; want always-respond")
	}
	if msg.Content != "hi back" {
		t.Errorf("reply = %q, want %q", msg.Content, "hi back")
	}
	// Private never needs the slug (no mention gate), but does build the runtime.
	if rt.slugCalls != 0 {
		t.Errorf("slug resolved %d times for a private chat, want 0", rt.slugCalls)
	}
	if rt.procCalls != 1 {
		t.Errorf("processor resolved %d times, want 1", rt.procCalls)
	}
}

func TestAgentTurn_GroupRespondsOnlyOnMention(t *testing.T) {
	ctx := context.Background()

	t.Run("mention triggers a turn", func(t *testing.T) {
		rt := &fakeRuntime{slug: "helper", proc: &echoAgent{reply: "yes?"}}
		svc, st, userID := setupTurn(t, rt)
		agentID := seedAgent(t, st, userID, "helper")
		c, err := svc.CreateGroupWithAgent(ctx, Actor{UserID: userID}, "Room", agentID)
		if err != nil {
			t.Fatalf("CreateGroupWithAgent: %v", err)
		}
		msg, responded, err := svc.AgentTurn(ctx, c, "hey @helper look at this")
		if err != nil {
			t.Fatalf("AgentTurn: %v", err)
		}
		if !responded {
			t.Fatal("mentioned agent did not respond")
		}
		if msg.Content != "yes?" {
			t.Errorf("reply = %q, want %q", msg.Content, "yes?")
		}
		if rt.slugCalls != 1 || rt.procCalls != 1 {
			t.Errorf("resolves slug=%d proc=%d, want 1/1", rt.slugCalls, rt.procCalls)
		}
	})

	t.Run("no mention stays silent", func(t *testing.T) {
		rt := &fakeRuntime{slug: "helper", proc: &echoAgent{reply: "should not run"}}
		svc, st, userID := setupTurn(t, rt)
		agentID := seedAgent(t, st, userID, "helper")
		c, err := svc.CreateGroupWithAgent(ctx, Actor{UserID: userID}, "Room", agentID)
		if err != nil {
			t.Fatalf("CreateGroupWithAgent: %v", err)
		}
		msg, responded, err := svc.AgentTurn(ctx, c, "just chatting amongst ourselves")
		if err != nil {
			t.Fatalf("AgentTurn: %v", err)
		}
		if responded || msg != nil {
			t.Fatalf("group responded without a mention: msg=%v", msg)
		}
		// Slug is checked; the processor must NOT be built for a non-mention.
		if rt.slugCalls != 1 {
			t.Errorf("slug resolved %d times, want 1", rt.slugCalls)
		}
		if rt.procCalls != 0 {
			t.Errorf("processor built %d times without a mention, want 0", rt.procCalls)
		}
	})
}

func TestAgentTurn_AgentBoundNoRuntimeStaysSilent(t *testing.T) {
	// Runtime nil: an agent-bound chat cannot resolve its agent, so it must not
	// answer as the wrong agent — it stays silent.
	svc, st, userID := setupTurn(t, nil)
	ctx := context.Background()
	agentID := seedAgent(t, st, userID, "helper")
	c, err := svc.StartAgentDM(ctx, Actor{UserID: userID}, agentID)
	if err != nil {
		t.Fatalf("StartAgentDM: %v", err)
	}
	msg, responded, err := svc.AgentTurn(ctx, c, "hello")
	if err != nil {
		t.Fatalf("AgentTurn: %v", err)
	}
	if responded || msg != nil {
		t.Fatalf("agent-bound chat responded with no runtime wired: msg=%v", msg)
	}
}

func TestAgentTurn_AgentlessPrivateUsesFallback(t *testing.T) {
	// No agent binding and no runtime: the personal-mode path — a private DM
	// always responds via the service-wide fallback processor.
	svc, userID := setupChats(t, &echoAgent{reply: "fallback reply"})
	ctx := context.Background()
	c, err := svc.PrivateChat(ctx, userID)
	if err != nil {
		t.Fatalf("PrivateChat: %v", err)
	}
	msg, responded, err := svc.AgentTurn(ctx, c, "hi")
	if err != nil {
		t.Fatalf("AgentTurn: %v", err)
	}
	if !responded {
		t.Fatal("agent-less private DM did not respond; want fallback reply")
	}
	if msg.Content != "fallback reply" {
		t.Errorf("reply = %q, want %q", msg.Content, "fallback reply")
	}
}

func TestAgentTurn_AgentlessGroupStaysSilent(t *testing.T) {
	// A group with no bound agent has no responder.
	svc, userID := setupChats(t, &echoAgent{reply: "should not run"})
	ctx := context.Background()
	c, err := svc.CreateGroup(ctx, Actor{UserID: userID}, "Humans Only")
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	msg, responded, err := svc.AgentTurn(ctx, c, "@anyone there")
	if err != nil {
		t.Fatalf("AgentTurn: %v", err)
	}
	if responded || msg != nil {
		t.Fatalf("agent-less group responded: msg=%v", msg)
	}
}
