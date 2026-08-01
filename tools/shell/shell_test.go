package shell

import "testing"

func TestIsAllowedCaseInsensitive(t *testing.T) {
	tool := &Tool{
		allowlist: []string{"git", "npm", "Docker*"},
	}

	tests := []struct {
		name    string
		command string
		want    bool
	}{
		{"exact match lowercase", "git status", true},
		{"exact match uppercase", "GIT status", true},
		{"exact match mixed case", "Git status", true},
		{"prefix match lowercase", "docker-compose up", true},
		{"prefix match uppercase", "DOCKER-COMPOSE up", true},
		{"prefix match mixed case", "Docker-compose up", true},
		{"not in allowlist", "rm -rf /", false},
		{"partial match not prefix", "git-lfs", false},
		{"empty command", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tool.isAllowed(tt.command)
			if got != tt.want {
				t.Errorf("isAllowed(%q) = %v, want %v", tt.command, got, tt.want)
			}
		})
	}
}

func TestIsAllowedEmptyAllowlist(t *testing.T) {
	tool := &Tool{
		allowlist: []string{},
	}

	if tool.isAllowed("git status") {
		t.Error("empty allowlist should not allow any command")
	}
}
