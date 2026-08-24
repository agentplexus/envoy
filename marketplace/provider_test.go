package marketplace

import (
	"context"
	"errors"
	"testing"

	"github.com/plexusone/omniagent/agent/registry"
	"github.com/plexusone/omniagent/skills"
)

func TestStaticProviderCatalogFiltersAgents(t *testing.T) {
	provider := NewStaticProvider([]AgentListing{
		{
			ID:          "analytics",
			Name:        "Analytics Agent",
			Description: "Builds GrokifyQL reports",
			Tags:        []string{"analytics", "sql"},
			Visibility:  VisibilityListed,
			Featured:    true,
			Enabled:     true,
			Capabilities: []CapabilityRef{{
				Name: "uiforge.query.run",
			}},
		},
		{
			ID:         "private",
			Name:       "Private Agent",
			Visibility: VisibilityPrivate,
			Enabled:    true,
		},
		{
			ID:         "ops",
			Name:       "Ops Agent",
			Tags:       []string{"ops"},
			Visibility: VisibilityListed,
			Enabled:    true,
		},
	}, nil)

	catalog, err := provider.Catalog(context.Background(), Filter{
		Query:        "grokifyql",
		Tags:         []string{"analytics"},
		Capabilities: []string{"uiforge.query.run"},
	})
	if err != nil {
		t.Fatalf("Catalog: %v", err)
	}
	if got := len(catalog.FeaturedAgents); got != 1 {
		t.Fatalf("featured agents len = %d, want 1", got)
	}
	if catalog.FeaturedAgents[0].ID != "analytics" {
		t.Fatalf("featured agent = %q, want analytics", catalog.FeaturedAgents[0].ID)
	}
	if got := len(catalog.Agents); got != 0 {
		t.Fatalf("listed agents len = %d, want 0", got)
	}
}

func TestStaticProviderIncludePrivate(t *testing.T) {
	provider := NewStaticProvider([]AgentListing{{
		ID:         "private",
		Name:       "Private Agent",
		Visibility: VisibilityPrivate,
		Enabled:    true,
	}}, nil)

	publicCatalog, err := provider.Catalog(context.Background(), Filter{})
	if err != nil {
		t.Fatalf("Catalog public: %v", err)
	}
	if len(publicCatalog.Agents) != 0 {
		t.Fatalf("public catalog agents len = %d, want 0", len(publicCatalog.Agents))
	}

	privateCatalog, err := provider.Catalog(context.Background(), Filter{IncludePrivate: true})
	if err != nil {
		t.Fatalf("Catalog private: %v", err)
	}
	if len(privateCatalog.Agents) != 1 {
		t.Fatalf("private catalog agents len = %d, want 1", len(privateCatalog.Agents))
	}
}

func TestStaticProviderSkillsAndDefensiveCopies(t *testing.T) {
	provider := NewStaticProvider(nil, []SkillListing{{
		Name:            "grokifyql",
		DisplayName:     "GrokifyQL",
		Tags:            []string{"analytics"},
		Available:       true,
		RequiredSecrets: []SecretRef{{Name: "TOKEN"}},
		Capabilities:    []CapabilityRef{{Name: "uiforge.query.plan"}},
	}})

	catalog, err := provider.Catalog(context.Background(), Filter{
		IncludeSkills: true,
		Tags:          []string{"analytics"},
	})
	if err != nil {
		t.Fatalf("Catalog: %v", err)
	}
	if len(catalog.Skills) != 1 {
		t.Fatalf("skills len = %d, want 1", len(catalog.Skills))
	}
	catalog.Skills[0].Tags[0] = "changed"

	got, err := provider.GetSkill(context.Background(), "grokifyql")
	if err != nil {
		t.Fatalf("GetSkill: %v", err)
	}
	if got.Tags[0] != "analytics" {
		t.Fatalf("stored tag = %q, want analytics", got.Tags[0])
	}
}

func TestStaticProviderGetMissing(t *testing.T) {
	provider := NewStaticProvider(nil, nil)
	if _, err := provider.GetAgent(context.Background(), "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetAgent err = %v, want ErrNotFound", err)
	}
	if _, err := provider.GetSkill(context.Background(), "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetSkill err = %v, want ErrNotFound", err)
	}
}

func TestAdaptersDoNotExposeSecretsOrPrompts(t *testing.T) {
	cfg := &registry.AgentConfig{
		ID:           "agent-1",
		Name:         "Agent One",
		Description:  "Useful agent",
		Provider:     "ollama",
		Model:        "llama3.1",
		APIKey:       "secret",
		SystemPrompt: "private prompt",
		AllowedTools: []string{"search"},
	}

	agent := AgentListingFromConfig(cfg)
	if agent.Provider != "ollama" || agent.Model != "llama3.1" {
		t.Fatalf("agent provider/model = %q/%q", agent.Provider, agent.Model)
	}
	if len(agent.Tools) != 1 || agent.Tools[0].Name != "search" {
		t.Fatalf("agent tools = %#v, want search", agent.Tools)
	}
	if agent.Metadata != nil {
		t.Fatalf("metadata = %#v, want nil", agent.Metadata)
	}

	skill := SkillListingFromSkill(&skills.Skill{
		Name:        "jira",
		Description: "Jira helper",
		Source:      skills.SourceDirectory,
		Metadata: skills.SkillMeta{OpenClaw: &skills.OpenClawMeta{Requires: &skills.Requires{
			Bins: []string{"jira"},
			Env:  []string{"JIRA_HOST"},
			Secrets: []skills.SecretRequirement{{
				Name:        "JIRA_TOKEN",
				Description: "Jira API token",
				Required:    true,
			}},
		}}},
	})
	if len(skill.RequiredSecrets) != 1 || skill.RequiredSecrets[0].Name != "JIRA_TOKEN" {
		t.Fatalf("skill secrets = %#v, want JIRA_TOKEN", skill.RequiredSecrets)
	}
	if len(skill.Requirements) != 2 {
		t.Fatalf("requirements len = %d, want 2", len(skill.Requirements))
	}
}
