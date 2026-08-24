package marketplace

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
)

var (
	// ErrNotFound is returned when a requested marketplace listing does not
	// exist or is not visible under the supplied filter.
	ErrNotFound = errors.New("marketplace listing not found")
)

// Provider returns marketplace listings. Implementations may be static,
// database-backed, remote, or authorization-aware.
type Provider interface {
	Catalog(ctx context.Context, filter Filter) (*Catalog, error)
	GetAgent(ctx context.Context, id string) (*AgentListing, error)
	GetSkill(ctx context.Context, name string) (*SkillListing, error)
}

// StaticProvider is an in-memory marketplace provider suitable for embedded
// catalogs and tests. It stores definitions, not user installs.
type StaticProvider struct {
	mu     sync.RWMutex
	agents map[string]AgentListing
	skills map[string]SkillListing
}

// NewStaticProvider creates a provider from optional initial listings.
func NewStaticProvider(agents []AgentListing, skills []SkillListing) *StaticProvider {
	p := &StaticProvider{
		agents: make(map[string]AgentListing),
		skills: make(map[string]SkillListing),
	}
	for _, agent := range agents {
		p.RegisterAgent(agent)
	}
	for _, skill := range skills {
		p.RegisterSkill(skill)
	}
	return p
}

// RegisterAgent adds or replaces an agent listing.
func (p *StaticProvider) RegisterAgent(agent AgentListing) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.agents == nil {
		p.agents = make(map[string]AgentListing)
	}
	p.agents[listingKey(agent.ID, agent.Slug, agent.Name)] = cloneAgent(agent)
}

// RegisterSkill adds or replaces a skill listing.
func (p *StaticProvider) RegisterSkill(skill SkillListing) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.skills == nil {
		p.skills = make(map[string]SkillListing)
	}
	p.skills[listingKey(skill.Name, skill.DisplayName)] = cloneSkill(skill)
}

// Catalog returns filtered marketplace listings.
func (p *StaticProvider) Catalog(ctx context.Context, filter Filter) (*Catalog, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	var featured, listed []AgentListing
	for _, agent := range p.agents {
		if !agentVisible(agent, filter.IncludePrivate) || !agentMatches(agent, filter) {
			continue
		}
		cloned := cloneAgent(agent)
		if agent.Featured {
			featured = append(featured, cloned)
		} else {
			listed = append(listed, cloned)
		}
	}
	sortAgents(featured)
	sortAgents(listed)

	out := &Catalog{
		FeaturedAgents: featured,
		Agents:         listed,
	}
	if filter.IncludeSkills {
		for _, skill := range p.skills {
			if skillMatches(skill, filter) {
				out.Skills = append(out.Skills, cloneSkill(skill))
			}
		}
		sortSkills(out.Skills)
	}
	return out, nil
}

// GetAgent returns one agent listing by id, slug, or name.
func (p *StaticProvider) GetAgent(ctx context.Context, id string) (*AgentListing, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	key := normalize(id)
	p.mu.RLock()
	defer p.mu.RUnlock()
	for _, agent := range p.agents {
		if normalize(agent.ID) == key || normalize(agent.Slug) == key || normalize(agent.Name) == key {
			cloned := cloneAgent(agent)
			return &cloned, nil
		}
	}
	return nil, ErrNotFound
}

// GetSkill returns one skill listing by name or display name.
func (p *StaticProvider) GetSkill(ctx context.Context, name string) (*SkillListing, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	key := normalize(name)
	p.mu.RLock()
	defer p.mu.RUnlock()
	for _, skill := range p.skills {
		if normalize(skill.Name) == key || normalize(skill.DisplayName) == key {
			cloned := cloneSkill(skill)
			return &cloned, nil
		}
	}
	return nil, ErrNotFound
}

func agentVisible(agent AgentListing, includePrivate bool) bool {
	if includePrivate {
		return true
	}
	return agent.Visibility == "" || agent.Visibility == VisibilityListed
}

func agentMatches(agent AgentListing, filter Filter) bool {
	return textMatches(filter.Query, agent.ID, agent.Slug, agent.Name, agent.Description, agent.Provider, agent.Model, agent.Category, strings.Join(agent.Tags, " ")) &&
		categoryMatches(filter.Category, agent.Category) &&
		tagsMatch(filter.Tags, agent.Tags) &&
		capabilitiesMatch(filter.Capabilities, agent.Capabilities) &&
		toolsMatch(filter.Tools, agent.Tools)
}

func skillMatches(skill SkillListing, filter Filter) bool {
	return textMatches(filter.Query, skill.Name, skill.DisplayName, skill.Description, skill.Homepage, skill.Category, strings.Join(skill.Tags, " ")) &&
		categoryMatches(filter.Category, skill.Category) &&
		tagsMatch(filter.Tags, skill.Tags) &&
		capabilitiesMatch(filter.Capabilities, skill.Capabilities) &&
		toolsMatch(filter.Tools, skill.Tools)
}

func textMatches(query string, values ...string) bool {
	query = normalize(query)
	if query == "" {
		return true
	}
	for _, value := range values {
		if strings.Contains(normalize(value), query) {
			return true
		}
	}
	return false
}

func categoryMatches(want, got string) bool {
	return normalize(want) == "" || normalize(want) == normalize(got)
}

func tagsMatch(want, got []string) bool {
	if len(want) == 0 {
		return true
	}
	have := make(map[string]bool, len(got))
	for _, tag := range got {
		have[normalize(tag)] = true
	}
	for _, tag := range want {
		if !have[normalize(tag)] {
			return false
		}
	}
	return true
}

func capabilitiesMatch(want []string, got []CapabilityRef) bool {
	if len(want) == 0 {
		return true
	}
	have := make(map[string]bool, len(got))
	for _, cap := range got {
		have[normalize(cap.Name)] = true
	}
	for _, cap := range want {
		if !have[normalize(cap)] {
			return false
		}
	}
	return true
}

func toolsMatch(want []string, got []ToolRef) bool {
	if len(want) == 0 {
		return true
	}
	have := make(map[string]bool, len(got))
	for _, tool := range got {
		have[normalize(tool.Name)] = true
	}
	for _, tool := range want {
		if !have[normalize(tool)] {
			return false
		}
	}
	return true
}

func sortAgents(agents []AgentListing) {
	sort.SliceStable(agents, func(i, j int) bool {
		return sortKey(agents[i].Name, agents[i].ID) < sortKey(agents[j].Name, agents[j].ID)
	})
}

func sortSkills(skills []SkillListing) {
	sort.SliceStable(skills, func(i, j int) bool {
		return sortKey(skills[i].DisplayName, skills[i].Name) < sortKey(skills[j].DisplayName, skills[j].Name)
	})
}

func sortKey(values ...string) string {
	for _, value := range values {
		if normalize(value) != "" {
			return normalize(value)
		}
	}
	return ""
}

func listingKey(values ...string) string {
	key := sortKey(values...)
	if key != "" {
		return key
	}
	return normalize(strings.Join(values, "-"))
}

func normalize(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func cloneAgent(agent AgentListing) AgentListing {
	agent.Tags = cloneStrings(agent.Tags)
	agent.Skills = cloneSkillRefs(agent.Skills)
	agent.Tools = cloneToolRefs(agent.Tools)
	agent.Capabilities = cloneCapabilityRefs(agent.Capabilities)
	agent.Metadata = cloneMap(agent.Metadata)
	return agent
}

func cloneSkill(skill SkillListing) SkillListing {
	skill.Tags = cloneStrings(skill.Tags)
	skill.RequiredSecrets = cloneSecretRefs(skill.RequiredSecrets)
	skill.Requirements = cloneRequirementRefs(skill.Requirements)
	skill.Tools = cloneToolRefs(skill.Tools)
	skill.Capabilities = cloneCapabilityRefs(skill.Capabilities)
	skill.Metadata = cloneMap(skill.Metadata)
	return skill
}

func cloneStrings(in []string) []string {
	if in == nil {
		return nil
	}
	out := make([]string, len(in))
	copy(out, in)
	return out
}

func cloneMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func cloneSkillRefs(in []SkillRef) []SkillRef {
	if in == nil {
		return nil
	}
	out := make([]SkillRef, len(in))
	for i, item := range in {
		item.Metadata = cloneMap(item.Metadata)
		out[i] = item
	}
	return out
}

func cloneToolRefs(in []ToolRef) []ToolRef {
	if in == nil {
		return nil
	}
	out := make([]ToolRef, len(in))
	for i, item := range in {
		item.Metadata = cloneMap(item.Metadata)
		out[i] = item
	}
	return out
}

func cloneCapabilityRefs(in []CapabilityRef) []CapabilityRef {
	if in == nil {
		return nil
	}
	out := make([]CapabilityRef, len(in))
	for i, item := range in {
		item.Metadata = cloneMap(item.Metadata)
		out[i] = item
	}
	return out
}

func cloneSecretRefs(in []SecretRef) []SecretRef {
	if in == nil {
		return nil
	}
	out := make([]SecretRef, len(in))
	copy(out, in)
	return out
}

func cloneRequirementRefs(in []RequirementRef) []RequirementRef {
	if in == nil {
		return nil
	}
	out := make([]RequirementRef, len(in))
	for i, item := range in {
		item.Any = cloneStrings(item.Any)
		out[i] = item
	}
	return out
}
