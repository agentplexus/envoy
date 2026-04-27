package skills

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// DefaultSearchPaths returns the default skill directories to search.
func DefaultSearchPaths() []string {
	paths := []string{
		"skills",
		".skills",
	}

	if home, err := os.UserHomeDir(); err == nil {
		paths = append([]string{filepath.Join(home, ".omniagent", "skills")}, paths...)
	}

	return paths
}

// DiscoverFromFS finds all skills in an embedded filesystem.
// Skills are expected at skills/<name>/SKILL.md.
// The seen map is used to track already-loaded skills for deduplication.
func DiscoverFromFS(fsys fs.FS, seen map[string]bool) ([]*Skill, error) {
	var skills []*Skill

	entries, err := fs.ReadDir(fsys, "skills")
	if err != nil {
		return nil, fmt.Errorf("reading skills directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		skillPath := "skills/" + entry.Name()
		skillMD := skillPath + "/SKILL.md"

		content, err := fs.ReadFile(fsys, skillMD)
		if err != nil {
			continue // No SKILL.md or read error
		}

		skill, err := Parse(string(content))
		if err != nil {
			continue // Invalid skill
		}

		// Skip if already seen (directory skills override embedded)
		if seen != nil && seen[skill.Name] {
			continue
		}
		if seen != nil {
			seen[skill.Name] = true
		}

		// Mark source as embedded (no filesystem path)
		skill.Path = ""
		skill.Source = SourceEmbedded

		// Check for hooks/scripts in embedded FS
		if _, err := fs.Stat(fsys, skillPath+"/hooks"); err == nil {
			skill.HasHooks = true
		}
		if _, err := fs.Stat(fsys, skillPath+"/scripts"); err == nil {
			skill.HasScripts = true
		}

		skills = append(skills, skill)
	}

	return skills, nil
}

// Discover finds all skills in the given directories.
// Skills are deduplicated by name (first occurrence wins).
func Discover(dirs []string) ([]*Skill, error) {
	var skills []*Skill
	seen := make(map[string]bool)

	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue // Skip missing directories
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			skillPath := filepath.Join(dir, entry.Name())
			skillMD := filepath.Join(skillPath, "SKILL.md")

			if _, err := os.Stat(skillMD); err != nil {
				continue // No SKILL.md
			}

			skill, err := Load(skillPath)
			if err != nil {
				continue // Invalid skill
			}

			// Dedupe by name (first wins)
			if seen[skill.Name] {
				continue
			}
			seen[skill.Name] = true

			skills = append(skills, skill)
		}
	}

	return skills, nil
}

// Load parses a single skill from its directory.
func Load(skillDir string) (*Skill, error) {
	content, err := os.ReadFile(filepath.Join(skillDir, "SKILL.md"))
	if err != nil {
		return nil, fmt.Errorf("reading SKILL.md: %w", err)
	}

	skill, err := Parse(string(content))
	if err != nil {
		return nil, fmt.Errorf("parsing SKILL.md: %w", err)
	}

	skill.Path = skillDir
	skill.Source = SourceDirectory

	// Check for hooks/scripts directories
	if _, err := os.Stat(filepath.Join(skillDir, "hooks")); err == nil {
		skill.HasHooks = true
	}
	if _, err := os.Stat(filepath.Join(skillDir, "scripts")); err == nil {
		skill.HasScripts = true
	}

	return skill, nil
}

// Parse extracts skill data from SKILL.md content.
func Parse(content string) (*Skill, error) {
	// Split on YAML frontmatter delimiters
	parts := strings.SplitN(content, "---", 3)
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid SKILL.md: missing frontmatter delimiters")
	}

	frontmatter := strings.TrimSpace(parts[1])
	body := strings.TrimSpace(parts[2])

	// Parse basic YAML fields
	var skill Skill
	if err := yaml.Unmarshal([]byte(frontmatter), &skill); err != nil {
		return nil, fmt.Errorf("parsing frontmatter YAML: %w", err)
	}

	// Handle metadata field specially - it may contain JSON
	skill.Metadata = parseMetadata(frontmatter)
	skill.Content = body

	return &skill, nil
}

// parseMetadata extracts the metadata field.
// The metadata field in SKILL.md uses YAML syntax that looks like JSON.
func parseMetadata(frontmatter string) SkillMeta {
	// First, parse the raw metadata as a generic map
	var raw struct {
		Metadata map[string]any `yaml:"metadata"`
	}

	if err := yaml.Unmarshal([]byte(frontmatter), &raw); err != nil {
		return SkillMeta{}
	}

	if raw.Metadata == nil {
		return SkillMeta{}
	}

	// Convert to JSON and back to get proper struct
	jsonBytes, err := json.Marshal(raw.Metadata)
	if err != nil {
		return SkillMeta{}
	}

	var meta SkillMeta
	if err := json.Unmarshal(jsonBytes, &meta); err != nil {
		return SkillMeta{}
	}

	return meta
}
