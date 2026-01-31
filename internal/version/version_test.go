package version

import (
	"runtime"
	"strings"
	"testing"
)

func TestGet(t *testing.T) {
	info := Get()

	if info.Version == "" {
		t.Error("Version should not be empty")
	}

	if info.GoVersion == "" {
		t.Error("GoVersion should not be empty")
	}

	if !strings.HasPrefix(info.GoVersion, "go") {
		t.Errorf("GoVersion should start with 'go', got %s", info.GoVersion)
	}

	if info.Platform == "" {
		t.Error("Platform should not be empty")
	}

	expectedPlatform := runtime.GOOS + "/" + runtime.GOARCH
	if info.Platform != expectedPlatform {
		t.Errorf("Platform = %s, want %s", info.Platform, expectedPlatform)
	}
}

func TestInfoString(t *testing.T) {
	info := Get()
	s := info.String()

	if !strings.Contains(s, "envoy") {
		t.Error("String should contain 'envoy'")
	}

	if !strings.Contains(s, info.Version) {
		t.Error("String should contain version")
	}

	if !strings.Contains(s, info.Platform) {
		t.Error("String should contain platform")
	}
}
