package skills

import "testing"

func TestRequirementError_Error(t *testing.T) {
	err := &RequirementError{Type: "binary", Name: "sonos", Skill: "sonoscli"}
	want := `skill "sonoscli" requires binary "sonos"`
	if got := err.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestRequirementError_InstallHint(t *testing.T) {
	tests := []struct {
		name    string
		install []Installer
		want    string
	}{
		{
			name: "no installers",
			want: "",
		},
		{
			name:    "brew",
			install: []Installer{{Kind: "brew", Formula: "sonos-cli"}},
			want:    "brew install sonos-cli",
		},
		{
			name:    "apt",
			install: []Installer{{Kind: "apt", Package: "sonos-cli"}},
			want:    "apt install sonos-cli",
		},
		{
			name:    "go",
			install: []Installer{{Kind: "go", Module: "example.com/sonos-cli@latest"}},
			want:    "go install example.com/sonos-cli@latest",
		},
		{
			name:    "npm",
			install: []Installer{{Kind: "npm", Package: "sonos-cli"}},
			want:    "npm install -g sonos-cli",
		},
		{
			name:    "unknown kind with label",
			install: []Installer{{Kind: "manual", Label: "see docs"}},
			want:    "see docs",
		},
		{
			name:    "unknown kind without label yields no hint",
			install: []Installer{{Kind: "manual"}},
			want:    "",
		},
		{
			name: "multiple installers joined with or",
			install: []Installer{
				{Kind: "brew", Formula: "sonos-cli"},
				{Kind: "npm", Package: "sonos-cli"},
			},
			want: "brew install sonos-cli or npm install -g sonos-cli",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &RequirementError{Type: "binary", Name: "sonos", Skill: "sonoscli", Install: tt.install}
			if got := err.InstallHint(); got != tt.want {
				t.Errorf("InstallHint() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCheckRequirements_AnyBins(t *testing.T) {
	// At least one of anyBins must exist; none present should error.
	skill := &Skill{
		Name: "test",
		Metadata: SkillMeta{OpenClaw: &OpenClawMeta{Requires: &Requires{
			AnyBins: []string{"nonexistent-binary-a", "nonexistent-binary-b"},
		}}},
	}
	errs := skill.CheckRequirements()
	if len(errs) != 1 {
		t.Fatalf("CheckRequirements() = %v, want 1 error for unmet anyBins", errs)
	}
	re, ok := errs[0].(*RequirementError)
	if !ok {
		t.Fatalf("error type = %T, want *RequirementError", errs[0])
	}
	if re.Type != "binary (any of)" {
		t.Errorf("Type = %q, want %q", re.Type, "binary (any of)")
	}

	// One of anyBins present (ls should exist on all systems): no error.
	skill2 := &Skill{
		Name: "test2",
		Metadata: SkillMeta{OpenClaw: &OpenClawMeta{Requires: &Requires{
			AnyBins: []string{"nonexistent-binary-a", "ls"},
		}}},
	}
	if errs2 := skill2.CheckRequirements(); len(errs2) != 0 {
		t.Errorf("CheckRequirements() = %v, want none when one anyBins entry is present", errs2)
	}
}

func TestCheckRequirements_NoOpenClawMetadata(t *testing.T) {
	skill := &Skill{Name: "bare"}
	if errs := skill.CheckRequirements(); len(errs) != 0 {
		t.Errorf("CheckRequirements() = %v, want none for a skill with no OpenClaw metadata", errs)
	}
}
