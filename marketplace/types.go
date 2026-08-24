// Package marketplace defines reusable agent and skill catalog primitives.
package marketplace

import "time"

// Visibility controls whether a listing is discoverable outside its owner or
// host application.
type Visibility string

const (
	VisibilityPrivate  Visibility = "private"
	VisibilityUnlisted Visibility = "unlisted"
	VisibilityListed   Visibility = "listed"
)

// CapabilityRef names a host- or agent-provided capability that a listing can
// use. Applications own the namespace, for example "uiforge.query.run".
type CapabilityRef struct {
	Name        string         `json:"name" yaml:"name"`
	Description string         `json:"description,omitempty" yaml:"description,omitempty"`
	Scope       string         `json:"scope,omitempty" yaml:"scope,omitempty"`
	Risk        string         `json:"risk,omitempty" yaml:"risk,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

// ToolRef names one executable tool or action that a listing can invoke.
type ToolRef struct {
	Name        string         `json:"name" yaml:"name"`
	Description string         `json:"description,omitempty" yaml:"description,omitempty"`
	Capability  string         `json:"capability,omitempty" yaml:"capability,omitempty"`
	Risk        string         `json:"risk,omitempty" yaml:"risk,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

// SkillRef attaches a skill to an agent listing.
type SkillRef struct {
	Name     string         `json:"name" yaml:"name"`
	Required bool           `json:"required,omitempty" yaml:"required,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

// SecretRef describes a secret a marketplace consumer may need to bind before
// enabling a listing. It never carries the secret value.
type SecretRef struct {
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	Required    bool   `json:"required,omitempty" yaml:"required,omitempty"`
	Env         string `json:"env,omitempty" yaml:"env,omitempty"`
}

// RequirementRef describes a local runtime prerequisite for a listing.
type RequirementRef struct {
	Type string   `json:"type" yaml:"type"`
	Name string   `json:"name" yaml:"name"`
	Any  []string `json:"any,omitempty" yaml:"any,omitempty"`
}

// AgentListing is the portable description of an agent that can be shown in an
// app marketplace or catalog.
type AgentListing struct {
	ID           string          `json:"id" yaml:"id"`
	Slug         string          `json:"slug,omitempty" yaml:"slug,omitempty"`
	Name         string          `json:"name" yaml:"name"`
	Description  string          `json:"description,omitempty" yaml:"description,omitempty"`
	Version      string          `json:"version,omitempty" yaml:"version,omitempty"`
	Provider     string          `json:"provider,omitempty" yaml:"provider,omitempty"`
	Model        string          `json:"model,omitempty" yaml:"model,omitempty"`
	Category     string          `json:"category,omitempty" yaml:"category,omitempty"`
	Icon         string          `json:"icon,omitempty" yaml:"icon,omitempty"`
	Tags         []string        `json:"tags,omitempty" yaml:"tags,omitempty"`
	Visibility   Visibility      `json:"visibility,omitempty" yaml:"visibility,omitempty"`
	Featured     bool            `json:"featured,omitempty" yaml:"featured,omitempty"`
	Enabled      bool            `json:"enabled" yaml:"enabled"`
	Skills       []SkillRef      `json:"skills,omitempty" yaml:"skills,omitempty"`
	Tools        []ToolRef       `json:"tools,omitempty" yaml:"tools,omitempty"`
	Capabilities []CapabilityRef `json:"capabilities,omitempty" yaml:"capabilities,omitempty"`
	Metadata     map[string]any  `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	CreatedAt    time.Time       `json:"createdAt,omitempty" yaml:"created_at,omitempty"`
	UpdatedAt    time.Time       `json:"updatedAt,omitempty" yaml:"updated_at,omitempty"`
}

// SkillListing is the portable description of a reusable skill.
type SkillListing struct {
	Name            string           `json:"name" yaml:"name"`
	DisplayName     string           `json:"displayName,omitempty" yaml:"display_name,omitempty"`
	Description     string           `json:"description,omitempty" yaml:"description,omitempty"`
	Homepage        string           `json:"homepage,omitempty" yaml:"homepage,omitempty"`
	Version         string           `json:"version,omitempty" yaml:"version,omitempty"`
	Source          string           `json:"source,omitempty" yaml:"source,omitempty"`
	Category        string           `json:"category,omitempty" yaml:"category,omitempty"`
	Icon            string           `json:"icon,omitempty" yaml:"icon,omitempty"`
	Tags            []string         `json:"tags,omitempty" yaml:"tags,omitempty"`
	Available       bool             `json:"available" yaml:"available"`
	RequiredSecrets []SecretRef      `json:"requiredSecrets,omitempty" yaml:"required_secrets,omitempty"`
	Requirements    []RequirementRef `json:"requirements,omitempty" yaml:"requirements,omitempty"`
	Tools           []ToolRef        `json:"tools,omitempty" yaml:"tools,omitempty"`
	Capabilities    []CapabilityRef  `json:"capabilities,omitempty" yaml:"capabilities,omitempty"`
	Metadata        map[string]any   `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

// Catalog is a marketplace view that host applications can render directly.
type Catalog struct {
	FeaturedAgents []AgentListing `json:"featuredAgents" yaml:"featured_agents"`
	Agents         []AgentListing `json:"agents" yaml:"agents"`
	Skills         []SkillListing `json:"skills,omitempty" yaml:"skills,omitempty"`
}

// Filter narrows a marketplace catalog lookup.
type Filter struct {
	Query          string   `json:"query,omitempty" yaml:"query,omitempty"`
	Tags           []string `json:"tags,omitempty" yaml:"tags,omitempty"`
	Category       string   `json:"category,omitempty" yaml:"category,omitempty"`
	Capabilities   []string `json:"capabilities,omitempty" yaml:"capabilities,omitempty"`
	Tools          []string `json:"tools,omitempty" yaml:"tools,omitempty"`
	IncludePrivate bool     `json:"includePrivate,omitempty" yaml:"include_private,omitempty"`
	IncludeSkills  bool     `json:"includeSkills,omitempty" yaml:"include_skills,omitempty"`
}
