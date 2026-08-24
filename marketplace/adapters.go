package marketplace

import (
	"github.com/plexusone/omniagent/agent/registry"
	"github.com/plexusone/omniagent/skills"
)

// AgentListingFromConfig converts a runtime agent config into a marketplace
// listing. API keys and prompts are intentionally omitted.
func AgentListingFromConfig(cfg *registry.AgentConfig) AgentListing {
	if cfg == nil {
		return AgentListing{}
	}
	tools := make([]ToolRef, 0, len(cfg.AllowedTools))
	for _, name := range cfg.AllowedTools {
		tools = append(tools, ToolRef{Name: name})
	}
	return AgentListing{
		ID:          cfg.ID,
		Name:        cfg.Name,
		Description: cfg.Description,
		Provider:    cfg.Provider,
		Model:       cfg.Model,
		Visibility:  VisibilityListed,
		Enabled:     cfg.IsEnabled(),
		Tools:       tools,
		CreatedAt:   cfg.CreatedAt,
		UpdatedAt:   cfg.UpdatedAt,
	}
}

// SkillListingFromSkill converts a loaded markdown skill into a marketplace
// listing. Secret declarations are copied by name only; values are never read.
func SkillListingFromSkill(skill *skills.Skill) SkillListing {
	if skill == nil {
		return SkillListing{}
	}
	listing := SkillListing{
		Name:        skill.Name,
		DisplayName: skill.Name,
		Description: skill.Description,
		Homepage:    skill.Homepage,
		Source:      string(skill.Source),
		Icon:        skill.Emoji(),
		Available:   skill.IsAvailable(),
	}
	if meta := skill.Metadata.OpenClaw; meta != nil && meta.Requires != nil {
		for _, bin := range meta.Requires.Bins {
			listing.Requirements = append(listing.Requirements, RequirementRef{Type: "binary", Name: bin})
		}
		if len(meta.Requires.AnyBins) > 0 {
			listing.Requirements = append(listing.Requirements, RequirementRef{Type: "binary_any", Any: cloneStrings(meta.Requires.AnyBins)})
		}
		for _, env := range meta.Requires.Env {
			listing.Requirements = append(listing.Requirements, RequirementRef{Type: "env", Name: env})
		}
		for _, secret := range meta.Requires.Secrets {
			listing.RequiredSecrets = append(listing.RequiredSecrets, SecretRef{
				Name:        secret.Name,
				Description: secret.Description,
				Required:    secret.Required,
				Env:         secret.EnvVar(),
			})
		}
	}
	return listing
}

// SkillListingsFromManager converts all loaded skills in a manager.
func SkillListingsFromManager(manager *skills.Manager) []SkillListing {
	if manager == nil {
		return nil
	}
	loaded := manager.All()
	out := make([]SkillListing, 0, len(loaded))
	for _, skill := range loaded {
		out = append(out, SkillListingFromSkill(skill))
	}
	return out
}
