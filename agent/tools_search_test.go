package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/plexusone/omniserp"
)

// TestNewSearchTool_NoAPIKeysConfigured documents NewSearchTool's behavior
// in an environment with no search-engine API keys (the default in CI/tests):
// it errors rather than panicking or silently returning a tool that would
// fail on first use. When keys *are* configured (e.g. a developer's local
// env), construction succeeds and the tool satisfies the Tool interface.
func TestNewSearchTool_NoAPIKeysConfigured(t *testing.T) {
	tool, err := NewSearchTool()
	if err != nil {
		if !strings.Contains(err.Error(), "create search client") {
			t.Errorf("NewSearchTool() error = %v, want a wrapped 'create search client' error", err)
		}
		return
	}
	// A key happened to be configured in this environment — still exercise
	// the always-safe metadata surface.
	var _ Tool = tool
}

// TestSearchTool_Metadata exercises the static metadata methods directly on
// a bare SearchTool (no live client needed since these never touch it),
// keeping the assertions independent of whether search API keys are
// configured in the test environment.
func TestSearchTool_Metadata(t *testing.T) {
	tool := &SearchTool{}
	if tool.Name() != "web_search" {
		t.Errorf("Name() = %q, want web_search", tool.Name())
	}
	if tool.Description() == "" {
		t.Error("Description() should not be empty")
	}
	params := tool.Parameters()
	if params["type"] != "object" {
		t.Errorf("Parameters()[type] = %v, want object", params["type"])
	}
}

// TestSearchTool_Execute_ValidationErrors covers the argument-validation
// paths that return before the (possibly nil, in this environment) client
// is ever touched.
func TestSearchTool_Execute_ValidationErrors(t *testing.T) {
	tool := &SearchTool{}
	ctx := context.Background()

	t.Run("invalid JSON", func(t *testing.T) {
		_, err := tool.Execute(ctx, json.RawMessage(`not-json`))
		if err == nil || !strings.Contains(err.Error(), "parse arguments") {
			t.Errorf("Execute() error = %v, want parse-arguments error", err)
		}
	})

	t.Run("empty query", func(t *testing.T) {
		raw, _ := json.Marshal(SearchArgs{Query: ""})
		_, err := tool.Execute(ctx, raw)
		if err == nil || !strings.Contains(err.Error(), "query is required") {
			t.Errorf("Execute() error = %v, want query-required error", err)
		}
	})
}

func TestFormatSearchResults(t *testing.T) {
	result := &omniserp.NormalizedSearchResult{
		SearchMetadata: omniserp.SearchMetadata{Query: "golang testing"},
		AnswerBox: &omniserp.AnswerBox{
			Answer: "Use the testing package.",
		},
		KnowledgeGraph: &omniserp.KnowledgeGraph{
			Title:       "Go (programming language)",
			Description: "Go is an open source language.",
		},
		OrganicResults: []omniserp.OrganicResult{
			{Title: "Result 1", Link: "https://example.com/1", Snippet: "snippet one"},
			{Title: "Result 2", Link: "https://example.com/2"},
			{Title: "Result 3", Link: "https://example.com/3"},
			{Title: "Result 4", Link: "https://example.com/4"},
			{Title: "Result 5", Link: "https://example.com/5"},
			{Title: "Result 6 (beyond the top-5 cap)", Link: "https://example.com/6"},
		},
		NewsResults: []omniserp.NewsResult{
			{Title: "News 1", Source: "Example News", Date: "today", Link: "https://news.example.com/1"},
			{Title: "News 2", Source: "Example News", Date: "today", Link: "https://news.example.com/2"},
			{Title: "News 3", Source: "Example News", Date: "today", Link: "https://news.example.com/3"},
			{Title: "News 4 (beyond the top-3 cap)", Source: "Example News", Date: "today", Link: "https://news.example.com/4"},
		},
	}

	got := formatSearchResults(result)

	for _, want := range []string{
		"Search results for: golang testing",
		"Direct Answer:",
		"Use the testing package.",
		"Knowledge Panel:",
		"Go (programming language)",
		"Go is an open source language.",
		"1. Result 1",
		"snippet one",
		"5. Result 5",
		"News: News 1",
		"News: News 3",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("formatSearchResults() missing %q, got:\n%s", want, got)
		}
	}

	if strings.Contains(got, "Result 6") {
		t.Error("organic results must be capped to the top 5")
	}
	if strings.Contains(got, "News 4") {
		t.Error("news results must be capped to the top 3")
	}
}

func TestFormatSearchResults_MinimalResult(t *testing.T) {
	result := &omniserp.NormalizedSearchResult{
		SearchMetadata: omniserp.SearchMetadata{Query: "bare query"},
	}
	got := formatSearchResults(result)

	if !strings.Contains(got, "Search results for: bare query") {
		t.Errorf("formatSearchResults() = %q, missing the query header", got)
	}
	for _, unwanted := range []string{"Direct Answer:", "Knowledge Panel:", "News:"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("formatSearchResults() unexpectedly contains %q for a minimal result", unwanted)
		}
	}
}
