// Package skills provides OpenClaw/ClawHub skill loading and management.
package skills

// SkillSource indicates where a skill was loaded from.
type SkillSource string

const (
	// SourceDirectory indicates the skill was loaded from a filesystem directory.
	SourceDirectory SkillSource = "directory"
	// SourceEmbedded indicates the skill was loaded from an embedded skill pack.
	SourceEmbedded SkillSource = "embedded"
)

// Skill represents a loaded SKILL.md file.
type Skill struct {
	// From YAML frontmatter
	Name        string    `yaml:"name"`
	Description string    `yaml:"description"`
	Homepage    string    `yaml:"homepage,omitempty"`
	Metadata    SkillMeta `yaml:"metadata"`

	// Parsed from file
	Content    string      `yaml:"-"` // Markdown body
	Path       string      `yaml:"-"` // Directory path (empty if embedded)
	Source     SkillSource `yaml:"-"` // Where the skill was loaded from
	HasHooks   bool        `yaml:"-"` // Has hooks/ directory
	HasScripts bool        `yaml:"-"` // Has scripts/ directory
}

// SkillMeta contains platform-specific metadata.
type SkillMeta struct {
	OpenClaw *OpenClawMeta `json:"openclaw,omitempty"`
}

// OpenClawMeta is the openclaw-specific metadata block.
type OpenClawMeta struct {
	Emoji    string      `json:"emoji,omitempty"`
	Requires *Requires   `json:"requires,omitempty"`
	Install  []Installer `json:"install,omitempty"`
	Always   bool        `json:"always,omitempty"`
}

// Requires specifies skill prerequisites.
type Requires struct {
	Bins    []string            `json:"bins,omitempty"`    // Required binaries on PATH
	AnyBins []string            `json:"anyBins,omitempty"` // At least one required
	Env     []string            `json:"env,omitempty"`     // Required environment variables
	Secrets []SecretRequirement `json:"secrets,omitempty"` // Declared secrets (INIT-OMNIAGENT-004)
}

// SecretRequirement is a secret a skill declares it needs, GitHub-Actions
// style. Declaration is the allowlist: a skill can only ever be injected with
// secrets it declares here. The value is bound out-of-band (operator config or
// the agent Secrets UI) and injected as an environment variable.
type SecretRequirement struct {
	// Name is the logical secret name (e.g. "GITHUB_TOKEN"). Shown in the UI.
	Name string `json:"name"`
	// Description is a human-readable hint shown alongside the input.
	Description string `json:"description,omitempty"`
	// Required gates skill availability: an unresolved required secret marks
	// the skill unavailable with an actionable message.
	Required bool `json:"required,omitempty"`
	// Env is the environment variable the value is injected as; defaults to
	// Name when empty.
	Env string `json:"env,omitempty"`
}

// EnvVar returns the environment variable the secret is injected as, defaulting
// to Name when Env is unset.
func (r SecretRequirement) EnvVar() string {
	if r.Env != "" {
		return r.Env
	}
	return r.Name
}

// DeclaredSecrets returns the secrets a skill declares in its SKILL.md
// frontmatter, or nil when it declares none.
func (s *Skill) DeclaredSecrets() []SecretRequirement {
	if s.Metadata.OpenClaw == nil || s.Metadata.OpenClaw.Requires == nil {
		return nil
	}
	return s.Metadata.OpenClaw.Requires.Secrets
}

// Installer specifies how to install a dependency.
type Installer struct {
	ID      string   `json:"id"`
	Kind    string   `json:"kind"`              // brew, apt, go, npm, etc.
	Formula string   `json:"formula,omitempty"` // For brew
	Package string   `json:"package,omitempty"` // For apt
	Module  string   `json:"module,omitempty"`  // For go install
	Bins    []string `json:"bins,omitempty"`    // Binaries provided
	Label   string   `json:"label,omitempty"`   // Human-readable label
}

// IsAvailable returns true if all requirements are met.
func (s *Skill) IsAvailable() bool {
	return len(s.CheckRequirements()) == 0
}

// Emoji returns the skill's emoji or empty string.
func (s *Skill) Emoji() string {
	if s.Metadata.OpenClaw != nil {
		return s.Metadata.OpenClaw.Emoji
	}
	return ""
}
