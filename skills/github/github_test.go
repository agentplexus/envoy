package github

import (
	"context"
	"testing"
)

func TestSkillInterface(t *testing.T) {
	skill := NewSkill(Config{})

	if skill.Name() != SkillName {
		t.Errorf("expected name %q, got %q", SkillName, skill.Name())
	}

	if skill.Description() == "" {
		t.Error("description should not be empty")
	}

	tools := skill.Tools()
	if len(tools) == 0 {
		t.Error("should have at least one tool")
	}

	// Check expected tools
	expectedTools := map[string]bool{
		"github_list_issues":        false,
		"github_get_issue":          false,
		"github_create_issue":       false,
		"github_update_issue":       false,
		"github_add_issue_comment":  false,
		"github_list_pull_requests": false,
		"github_get_pull_request":   false,
		"github_add_pr_comment":     false,
		"github_search_code":        false,
		"github_search_issues":      false,
	}

	for _, tool := range tools {
		if _, ok := expectedTools[tool.Name()]; ok {
			expectedTools[tool.Name()] = true
		}
	}

	for name, found := range expectedTools {
		if !found {
			t.Errorf("expected tool %q not found", name)
		}
	}
}

func TestToolParameters(t *testing.T) {
	skill := NewSkill(Config{})
	tools := skill.Tools()

	for _, tool := range tools {
		params := tool.Parameters()
		if params == nil {
			t.Errorf("tool %q should have parameters", tool.Name())
		}

		if tool.Description() == "" {
			t.Errorf("tool %q should have description", tool.Name())
		}
	}
}

func TestInit(t *testing.T) {
	skill := NewSkill(Config{})

	// Init should succeed even without a token (public API access)
	if err := skill.Init(context.Background()); err != nil {
		t.Errorf("Init should succeed: %v", err)
	}

	if err := skill.Close(); err != nil {
		t.Errorf("Close should succeed: %v", err)
	}
}

func TestInitWithToken(t *testing.T) {
	skill := NewSkill(Config{
		Token: "test-token",
	})

	if err := skill.Init(context.Background()); err != nil {
		t.Errorf("Init with token should succeed: %v", err)
	}

	if err := skill.Close(); err != nil {
		t.Errorf("Close should succeed: %v", err)
	}
}

func TestInitWithEnterprise(t *testing.T) {
	skill := NewSkill(Config{
		Token:   "test-token",
		BaseURL: "https://github.example.com/api/v3",
	})

	if err := skill.Init(context.Background()); err != nil {
		t.Errorf("Init with enterprise URL should succeed: %v", err)
	}

	if err := skill.Close(); err != nil {
		t.Errorf("Close should succeed: %v", err)
	}
}

func TestMapToStruct(t *testing.T) {
	tests := []struct {
		name    string
		input   map[string]any
		want    listIssuesInput
		wantErr bool
	}{
		{
			name: "valid input",
			input: map[string]any{
				"owner":    "plexusone",
				"repo":     "omniagent",
				"state":    "open",
				"per_page": 10,
			},
			want: listIssuesInput{
				Owner:   "plexusone",
				Repo:    "omniagent",
				State:   "open",
				PerPage: 10,
			},
		},
		{
			name: "partial input",
			input: map[string]any{
				"owner": "plexusone",
				"repo":  "omniagent",
			},
			want: listIssuesInput{
				Owner: "plexusone",
				Repo:  "omniagent",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got listIssuesInput
			err := mapToStruct(tt.input, &got)
			if (err != nil) != tt.wantErr {
				t.Errorf("mapToStruct() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("mapToStruct() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
