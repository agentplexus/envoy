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
	Bins    []string `json:"bins,omitempty"`    // Required binaries on PATH
	AnyBins []string `json:"anyBins,omitempty"` // At least one required
	Env     []string `json:"env,omitempty"`     // Required environment variables
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
