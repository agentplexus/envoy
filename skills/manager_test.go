package skills

import (
	"io/fs"
	"testing"
	"testing/fstest"
)

func TestNewManager(t *testing.T) {
	cfg := ManagerConfig{Dirs: []string{"somedir"}}
	m := NewManager(cfg)

	if m.Count() != 0 {
		t.Errorf("Count() = %d before Load(), want 0", m.Count())
	}
	if got := m.Get("anything"); got != nil {
		t.Errorf("Get() before Load() = %+v, want nil", got)
	}
	if got := m.All(); got != nil {
		t.Errorf("All() before Load() = %+v, want nil", got)
	}
}

func TestManager_Load_FromDirectories(t *testing.T) {
	dir := t.TempDir()
	writeSkillDir(t, dir, "alpha", "", "# Alpha")
	writeSkillDir(t, dir, "beta", "", "# Beta")

	m := NewManager(ManagerConfig{Dirs: []string{dir}})
	if err := m.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if m.Count() != 2 {
		t.Fatalf("Count() = %d, want 2", m.Count())
	}
	if got := m.Get("alpha"); got == nil || got.Name != "alpha" {
		t.Errorf("Get(alpha) = %+v", got)
	}
	if got := m.Get("missing"); got != nil {
		t.Errorf("Get(missing) = %+v, want nil", got)
	}
	if len(m.All()) != 2 {
		t.Errorf("All() = %+v, want 2 entries", m.All())
	}
}

func TestManager_Load_DefaultDirsWhenUnset(t *testing.T) {
	// No Dirs configured: falls back to DefaultSearchPaths(), none of which
	// should exist relative to the package test working directory, so this
	// should succeed with zero skills rather than error.
	m := NewManager(ManagerConfig{})
	if err := m.Load(); err != nil {
		t.Fatalf("Load() error = %v, want nil even when default search paths are absent", err)
	}
}

func TestManager_Load_EmbeddedPack(t *testing.T) {
	pack := fstest.MapFS{
		"skills/gamma/SKILL.md": {Data: []byte("---\nname: gamma\ndescription: from a pack\n---\n\n# Gamma\n")},
	}

	// Pin Dirs to an empty temp dir so this doesn't pick up any real skills
	// installed under the host's default search paths (e.g. ~/.omniagent/skills).
	m := NewManager(ManagerConfig{Dirs: []string{t.TempDir()}, Packs: []fs.FS{pack}})
	if err := m.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if m.Count() != 1 {
		t.Fatalf("Count() = %d, want 1", m.Count())
	}
	got := m.Get("gamma")
	if got == nil {
		t.Fatal("Get(gamma) = nil")
	}
	if got.Source != SourceEmbedded {
		t.Errorf("Source = %q, want %q", got.Source, SourceEmbedded)
	}
}

func TestManager_Load_DirOverridesEmbedded(t *testing.T) {
	dir := t.TempDir()
	writeSkillDir(t, dir, "dup", "", "# Directory version")

	pack := fstest.MapFS{
		"skills/dup/SKILL.md": {Data: []byte("---\nname: dup\ndescription: embedded\n---\n\n# Embedded version\n")},
	}

	m := NewManager(ManagerConfig{Dirs: []string{dir}, Packs: []fs.FS{pack}})
	if err := m.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if m.Count() != 1 {
		t.Fatalf("Count() = %d, want 1 (deduped)", m.Count())
	}
	got := m.Get("dup")
	if got == nil {
		t.Fatal("Get(dup) = nil")
	}
	if got.Source != SourceDirectory {
		t.Errorf("Source = %q, want %q (directory skills override embedded)", got.Source, SourceDirectory)
	}
}

func TestManager_Load_PackErrorsAreIgnored(t *testing.T) {
	dir := t.TempDir()
	writeSkillDir(t, dir, "solo", "", "# Solo")

	// A pack lacking a top-level "skills" directory fails DiscoverFromFS;
	// Load() should tolerate that and keep going (packs are optional).
	badPack := fstest.MapFS{"unrelated/file.txt": {Data: []byte("x")}}

	m := NewManager(ManagerConfig{Dirs: []string{dir}, Packs: []fs.FS{badPack}})
	if err := m.Load(); err != nil {
		t.Fatalf("Load() error = %v, want nil (bad packs are skipped)", err)
	}
	if m.Count() != 1 {
		t.Errorf("Count() = %d, want 1", m.Count())
	}
}

func TestManager_Load_Includes(t *testing.T) {
	dir := t.TempDir()
	writeSkillDir(t, dir, "a", "", "# A")
	writeSkillDir(t, dir, "b", "", "# B")
	writeSkillDir(t, dir, "c", "", "# C")

	m := NewManager(ManagerConfig{Dirs: []string{dir}, Includes: []string{"a", "c"}})
	if err := m.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if m.Count() != 2 {
		t.Fatalf("Count() = %d, want 2", m.Count())
	}
	if m.Get("b") != nil {
		t.Error("Get(b) should be nil, excluded by Includes")
	}
}

func TestManager_Load_Excludes(t *testing.T) {
	dir := t.TempDir()
	writeSkillDir(t, dir, "a", "", "# A")
	writeSkillDir(t, dir, "b", "", "# B")
	writeSkillDir(t, dir, "c", "", "# C")

	m := NewManager(ManagerConfig{Dirs: []string{dir}, Excludes: []string{"b"}})
	if err := m.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if m.Count() != 2 {
		t.Fatalf("Count() = %d, want 2", m.Count())
	}
	if m.Get("b") != nil {
		t.Error("Get(b) should be nil, removed by Excludes")
	}
}

func TestManager_Load_IncludesThenExcludes(t *testing.T) {
	dir := t.TempDir()
	writeSkillDir(t, dir, "a", "", "# A")
	writeSkillDir(t, dir, "b", "", "# B")
	writeSkillDir(t, dir, "c", "", "# C")

	// Excludes is applied after includes: "b" is in Includes but also
	// Excludes, so it must not survive.
	m := NewManager(ManagerConfig{Dirs: []string{dir}, Includes: []string{"a", "b"}, Excludes: []string{"b"}})
	if err := m.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if m.Count() != 1 {
		t.Fatalf("Count() = %d, want 1", m.Count())
	}
	if m.Get("a") == nil {
		t.Error("Get(a) = nil, want present")
	}
}

func TestManager_Available(t *testing.T) {
	dir := t.TempDir()
	writeSkillDir(t, dir, "available", "", "# Available")
	writeSkillDir(t, dir, "unavailable", "metadata: {\"openclaw\": {\"requires\": {\"bins\": [\"nonexistent-binary-12345\"]}}}\n", "# Unavailable")

	m := NewManager(ManagerConfig{Dirs: []string{dir}})
	if err := m.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if m.Count() != 2 {
		t.Fatalf("Count() = %d, want 2", m.Count())
	}
	available := m.Available()
	if len(available) != 1 {
		t.Fatalf("Available() = %+v, want 1 entry", available)
	}
	if available[0].Name != "available" {
		t.Errorf("Available()[0].Name = %q, want %q", available[0].Name, "available")
	}
	if m.AvailableCount() != 1 {
		t.Errorf("AvailableCount() = %d, want 1", m.AvailableCount())
	}
}
