package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

// writeSkillDir writes a minimal SKILL.md for name under parent, returning
// the skill directory path.
func writeSkillDir(t *testing.T, parent, name, extraFrontmatter, body string) string {
	t.Helper()
	dir := filepath.Join(parent, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	content := "---\nname: " + name + "\ndescription: test skill " + name + "\n" + extraFrontmatter + "---\n\n" + body + "\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return dir
}

func TestDefaultSearchPaths(t *testing.T) {
	paths := DefaultSearchPaths()

	// "skills" and ".skills" must always be present.
	foundSkills, foundDotSkills := false, false
	for _, p := range paths {
		if p == "skills" {
			foundSkills = true
		}
		if p == ".skills" {
			foundDotSkills = true
		}
	}
	if !foundSkills || !foundDotSkills {
		t.Errorf("DefaultSearchPaths() = %v, want to contain %q and %q", paths, "skills", ".skills")
	}

	// When the home directory resolves, it is prepended so it's searched
	// (and can be overridden by) the relative paths.
	if home, err := os.UserHomeDir(); err == nil {
		want := filepath.Join(home, ".omniagent", "skills")
		if len(paths) == 0 || paths[0] != want {
			t.Errorf("DefaultSearchPaths()[0] = %q, want %q (home-relative path first)", paths[0], want)
		}
	}
}

func TestDiscover_MalformedManifestSkipped(t *testing.T) {
	dir := t.TempDir()

	// Valid skill.
	writeSkillDir(t, dir, "good", "", "# Good\n\nBody.")

	// Malformed: no frontmatter delimiters at all.
	badDir := filepath.Join(dir, "bad")
	if err := os.MkdirAll(badDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(badDir, "SKILL.md"), []byte("# Not frontmatter at all"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Directory without a SKILL.md at all - should be silently skipped.
	noManifestDir := filepath.Join(dir, "no-manifest")
	if err := os.MkdirAll(noManifestDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	// A plain file alongside the skill directories - must not be treated as a skill.
	if err := os.WriteFile(filepath.Join(dir, "README.txt"), []byte("not a skill"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	skills, err := Discover([]string{dir})
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("Discover() found %d skills, want 1 (malformed/missing manifests and files must be skipped): %+v", len(skills), skills)
	}
	if skills[0].Name != "good" {
		t.Errorf("Discover()[0].Name = %q, want %q", skills[0].Name, "good")
	}
}

func TestDiscover_MissingDirsSkippedSilently(t *testing.T) {
	skills, err := Discover([]string{filepath.Join(t.TempDir(), "does-not-exist")})
	if err != nil {
		t.Fatalf("Discover() error = %v, want nil (missing dirs are skipped, not fatal)", err)
	}
	if len(skills) != 0 {
		t.Errorf("Discover() = %+v, want empty", skills)
	}
}

func TestDiscover_DedupeFirstDirWins(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	writeSkillDir(t, dirA, "shared", "", "# From A")
	writeSkillDir(t, dirB, "shared", "", "# From B")

	skills, err := Discover([]string{dirA, dirB})
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("Discover() found %d skills, want 1 deduped", len(skills))
	}
	if skills[0].Path != filepath.Join(dirA, "shared") {
		t.Errorf("Discover() kept %q, want the first directory's copy (%q)", skills[0].Path, filepath.Join(dirA, "shared"))
	}
}

func TestDiscoverFromFS(t *testing.T) {
	validContent := "---\nname: valid\ndescription: valid skill\n---\n\n# Valid\n"
	invalidContent := "no frontmatter here"

	fsys := fstest.MapFS{
		"skills/valid/SKILL.md":       {Data: []byte(validContent)},
		"skills/valid/hooks/setup.sh": {Data: []byte("#!/bin/sh")},
		"skills/valid/scripts/run.sh": {Data: []byte("#!/bin/sh")},
		"skills/invalid/SKILL.md":     {Data: []byte(invalidContent)},
		"skills/no-manifest/other.md": {Data: []byte("not a skill file")},
		"skills/readme.txt":           {Data: []byte("not a skill dir")}, // top-level file, not a dir
	}

	skills, err := DiscoverFromFS(fsys, nil)
	if err != nil {
		t.Fatalf("DiscoverFromFS() error = %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("DiscoverFromFS() found %d skills, want 1: %+v", len(skills), skills)
	}

	got := skills[0]
	if got.Name != "valid" {
		t.Errorf("Name = %q, want %q", got.Name, "valid")
	}
	if got.Source != SourceEmbedded {
		t.Errorf("Source = %q, want %q", got.Source, SourceEmbedded)
	}
	if got.Path != "" {
		t.Errorf("Path = %q, want empty for embedded skill", got.Path)
	}
	if !got.HasHooks {
		t.Error("HasHooks = false, want true")
	}
	if !got.HasScripts {
		t.Error("HasScripts = false, want true")
	}
}

func TestDiscoverFromFS_DedupeAgainstSeen(t *testing.T) {
	fsys := fstest.MapFS{
		"skills/valid/SKILL.md": {Data: []byte("---\nname: valid\n---\n\nBody\n")},
	}

	seen := map[string]bool{"valid": true}
	skills, err := DiscoverFromFS(fsys, seen)
	if err != nil {
		t.Fatalf("DiscoverFromFS() error = %v", err)
	}
	if len(skills) != 0 {
		t.Errorf("DiscoverFromFS() = %+v, want empty (already seen)", skills)
	}
}

func TestDiscoverFromFS_MissingSkillsDir(t *testing.T) {
	fsys := fstest.MapFS{
		"other/file.txt": {Data: []byte("nothing relevant")},
	}

	_, err := DiscoverFromFS(fsys, nil)
	if err == nil {
		t.Fatal("DiscoverFromFS() error = nil, want error for missing skills/ directory")
	}
	if !strings.Contains(err.Error(), "reading skills directory") {
		t.Errorf("DiscoverFromFS() error = %v, want it to mention the missing skills directory", err)
	}
}

func TestLoad_MissingSkillMD(t *testing.T) {
	dir := t.TempDir()
	if _, err := Load(dir); err == nil {
		t.Fatal("Load() error = nil, want error when SKILL.md is missing")
	}
}

func TestLoad_MalformedManifest(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("no frontmatter"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := Load(dir); err == nil {
		t.Fatal("Load() error = nil, want error for malformed SKILL.md")
	}
}
