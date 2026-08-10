package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    *Skill
		wantErr bool
	}{
		{
			name: "simple skill",
			content: `---
name: test-skill
description: A test skill
---

# Test Skill

This is the body.
`,
			want: &Skill{
				Name:        "test-skill",
				Description: "A test skill",
				Content:     "# Test Skill\n\nThis is the body.",
			},
		},
		{
			name: "skill with metadata",
			content: `---
name: sonoscli
description: Control Sonos speakers
metadata:
  {
    "openclaw":
      {
        "emoji": "🔊",
        "requires": { "bins": ["sonos"] },
      },
  }
---

# Sonos CLI

Use sonos to control speakers.
`,
			want: &Skill{
				Name:        "sonoscli",
				Description: "Control Sonos speakers",
				Content:     "# Sonos CLI\n\nUse sonos to control speakers.",
				Metadata: SkillMeta{
					OpenClaw: &OpenClawMeta{
						Emoji: "🔊",
						Requires: &Requires{
							Bins: []string{"sonos"},
						},
					},
				},
			},
		},
		{
			name:    "missing frontmatter",
			content: "# Just markdown\n\nNo frontmatter here.",
			wantErr: true,
		},
		{
			name: "empty frontmatter",
			content: `---
---

# Body only
`,
			want: &Skill{
				Content: "# Body only",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.content)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			if got.Name != tt.want.Name {
				t.Errorf("Name = %q, want %q", got.Name, tt.want.Name)
			}
			if got.Description != tt.want.Description {
				t.Errorf("Description = %q, want %q", got.Description, tt.want.Description)
			}
			if got.Content != tt.want.Content {
				t.Errorf("Content = %q, want %q", got.Content, tt.want.Content)
			}
			if tt.want.Metadata.OpenClaw != nil {
				if got.Metadata.OpenClaw == nil {
					t.Error("Metadata.OpenClaw is nil, want non-nil")
				} else {
					if got.Metadata.OpenClaw.Emoji != tt.want.Metadata.OpenClaw.Emoji {
						t.Errorf("Emoji = %q, want %q", got.Metadata.OpenClaw.Emoji, tt.want.Metadata.OpenClaw.Emoji)
					}
				}
			}
		})
	}
}

func TestDeclaredSecrets(t *testing.T) {
	content := `---
name: github
description: GitHub skill
metadata:
  {
    "openclaw":
      {
        "requires":
          {
            "secrets":
              [
                { "name": "GITHUB_TOKEN", "description": "A PAT", "required": true },
                { "name": "GITHUB_ENTERPRISE_URL", "env": "GH_ENTERPRISE_URL" },
              ],
          },
      },
  }
---

# GitHub
`
	skill, err := Parse(content)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	secrets := skill.DeclaredSecrets()
	if len(secrets) != 2 {
		t.Fatalf("DeclaredSecrets len = %d, want 2 (%+v)", len(secrets), secrets)
	}

	if secrets[0].Name != "GITHUB_TOKEN" || !secrets[0].Required || secrets[0].Description != "A PAT" {
		t.Errorf("secret[0] = %+v", secrets[0])
	}
	// Env defaults to Name when unset.
	if got := secrets[0].EnvVar(); got != "GITHUB_TOKEN" {
		t.Errorf("secret[0].EnvVar() = %q, want GITHUB_TOKEN (defaults to Name)", got)
	}
	// Explicit Env wins.
	if got := secrets[1].EnvVar(); got != "GH_ENTERPRISE_URL" {
		t.Errorf("secret[1].EnvVar() = %q, want GH_ENTERPRISE_URL", got)
	}
	if secrets[1].Required {
		t.Error("secret[1] should not be required")
	}
}

func TestDeclaredSecrets_None(t *testing.T) {
	skill, err := Parse("---\nname: bare\n---\n\n# Bare\n")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got := skill.DeclaredSecrets(); got != nil {
		t.Errorf("DeclaredSecrets = %+v, want nil for a skill with no declarations", got)
	}
}

func TestLoad(t *testing.T) {
	// Get the testdata directory relative to this test file
	testdataDir := filepath.Join("..", "testdata", "skills")

	tests := []struct {
		name      string
		skillDir  string
		wantName  string
		wantEmoji string
		wantBins  []string
	}{
		{
			name:      "sonoscli",
			skillDir:  filepath.Join(testdataDir, "sonoscli"),
			wantName:  "sonoscli",
			wantEmoji: "🔊",
			wantBins:  []string{"sonos"},
		},
		{
			name:      "github",
			skillDir:  filepath.Join(testdataDir, "github"),
			wantName:  "github",
			wantEmoji: "🐙",
			wantBins:  []string{"gh"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Skip if testdata doesn't exist
			if _, err := os.Stat(tt.skillDir); os.IsNotExist(err) {
				t.Skipf("testdata not found: %s", tt.skillDir)
			}

			skill, err := Load(tt.skillDir)
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}

			if skill.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", skill.Name, tt.wantName)
			}

			if skill.Emoji() != tt.wantEmoji {
				t.Errorf("Emoji() = %q, want %q", skill.Emoji(), tt.wantEmoji)
			}

			if skill.Metadata.OpenClaw != nil && skill.Metadata.OpenClaw.Requires != nil {
				gotBins := skill.Metadata.OpenClaw.Requires.Bins
				if len(gotBins) != len(tt.wantBins) {
					t.Errorf("Requires.Bins = %v, want %v", gotBins, tt.wantBins)
				}
			}

			if skill.Path != tt.skillDir {
				t.Errorf("Path = %q, want %q", skill.Path, tt.skillDir)
			}
		})
	}
}

func TestLoadSelfImprovingAgent(t *testing.T) {
	skillDir := filepath.Join("..", "testdata", "skills", "self-improving-agent")
	skillFile := filepath.Join(skillDir, "SKILL.md")

	// Check for the actual SKILL.md file, not just the directory
	// (directory may exist as a git submodule reference without content)
	if _, err := os.Stat(skillFile); os.IsNotExist(err) {
		t.Skipf("testdata not found (submodule not initialized?): %s", skillFile)
	}

	skill, err := Load(skillDir)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if skill.Name != "self-improvement" {
		t.Errorf("Name = %q, want %q", skill.Name, "self-improvement")
	}

	if !skill.HasHooks {
		t.Error("HasHooks = false, want true")
	}

	if !skill.HasScripts {
		t.Error("HasScripts = false, want true")
	}
}

func TestDiscover(t *testing.T) {
	testdataDir := filepath.Join("..", "testdata", "skills")

	if _, err := os.Stat(testdataDir); os.IsNotExist(err) {
		t.Skipf("testdata not found: %s", testdataDir)
	}

	skills, err := Discover([]string{testdataDir})
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}

	if len(skills) < 2 {
		t.Errorf("Discover() found %d skills, want at least 2", len(skills))
	}

	// Check that we found expected skills
	names := make(map[string]bool)
	for _, s := range skills {
		names[s.Name] = true
	}

	for _, want := range []string{"sonoscli", "github"} {
		if !names[want] {
			t.Errorf("Discover() missing skill %q", want)
		}
	}
}

func TestCheckRequirements(t *testing.T) {
	// Test skill with missing binary
	skill := &Skill{
		Name: "test",
		Metadata: SkillMeta{
			OpenClaw: &OpenClawMeta{
				Requires: &Requires{
					Bins: []string{"nonexistent-binary-12345"},
				},
			},
		},
	}

	errs := skill.CheckRequirements()
	if len(errs) == 0 {
		t.Error("CheckRequirements() returned no errors for missing binary")
	}

	// Test skill with existing binary (ls should exist on all systems)
	skill2 := &Skill{
		Name: "test2",
		Metadata: SkillMeta{
			OpenClaw: &OpenClawMeta{
				Requires: &Requires{
					Bins: []string{"ls"},
				},
			},
		},
	}

	errs2 := skill2.CheckRequirements()
	if len(errs2) > 0 {
		t.Errorf("CheckRequirements() returned errors for existing binary: %v", errs2)
	}

	// Test skill with missing env var
	skill3 := &Skill{
		Name: "test3",
		Metadata: SkillMeta{
			OpenClaw: &OpenClawMeta{
				Requires: &Requires{
					Env: []string{"NONEXISTENT_ENV_VAR_12345"},
				},
			},
		},
	}

	errs3 := skill3.CheckRequirements()
	if len(errs3) == 0 {
		t.Error("CheckRequirements() returned no errors for missing env var")
	}

	// Test skill with no requirements
	skill4 := &Skill{Name: "test4"}
	errs4 := skill4.CheckRequirements()
	if len(errs4) > 0 {
		t.Errorf("CheckRequirements() returned errors for skill with no requirements: %v", errs4)
	}
}

func TestInjectIntoPrompt(t *testing.T) {
	skills := []*Skill{
		{
			Name:        "skill1",
			Description: "First skill",
			Content:     "Use skill1 to do things.",
			Metadata: SkillMeta{
				OpenClaw: &OpenClawMeta{
					Emoji: "🔧",
				},
			},
		},
		{
			Name:        "skill2",
			Description: "Second skill",
			Content:     "Use skill2 to do other things.",
		},
	}

	systemPrompt := "You are a helpful assistant."
	result := InjectIntoPrompt(systemPrompt, skills, DefaultInjectConfig())

	// Check that system prompt is preserved
	if !strings.HasPrefix(result, systemPrompt) {
		t.Error("InjectIntoPrompt() did not preserve system prompt")
	}

	// Check that skills are injected
	if !strings.Contains(result, "skill1") {
		t.Error("InjectIntoPrompt() missing skill1")
	}
	if !strings.Contains(result, "skill2") {
		t.Error("InjectIntoPrompt() missing skill2")
	}

	// Check that emoji is included
	if !strings.Contains(result, "🔧") {
		t.Error("InjectIntoPrompt() missing emoji")
	}

	// Check that content is included
	if !strings.Contains(result, "Use skill1 to do things.") {
		t.Error("InjectIntoPrompt() missing skill content")
	}
}

func TestFilterAvailable(t *testing.T) {
	skills := []*Skill{
		{
			Name: "available",
			// No requirements - should be available
		},
		{
			Name: "unavailable",
			Metadata: SkillMeta{
				OpenClaw: &OpenClawMeta{
					Requires: &Requires{
						Bins: []string{"nonexistent-binary-12345"},
					},
				},
			},
		},
	}

	available := FilterAvailable(skills)
	if len(available) != 1 {
		t.Errorf("FilterAvailable() returned %d skills, want 1", len(available))
	}

	if available[0].Name != "available" {
		t.Errorf("FilterAvailable() returned wrong skill: %q", available[0].Name)
	}
}
