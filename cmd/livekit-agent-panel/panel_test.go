package main

import (
	"context"
	"testing"
)

// MockLLMClient is a mock LLM client for testing.
type MockLLMClient struct {
	Response string
	Err      error
}

func (m *MockLLMClient) Generate(_ context.Context, _, _ string, _ int) (string, error) {
	return m.Response, m.Err
}

func TestTranscript_AddAndFormat(t *testing.T) {
	tr := NewTranscript("AI Ethics")

	if tr.Topic() != "AI Ethics" {
		t.Errorf("Topic() = %q, want %q", tr.Topic(), "AI Ethics")
	}

	if tr.Len() != 0 {
		t.Errorf("Len() = %d, want 0", tr.Len())
	}

	// Empty transcript
	formatted := tr.Format()
	if formatted != "(No previous discussion yet)" {
		t.Errorf("Format() empty = %q, want %q", formatted, "(No previous discussion yet)")
	}

	// Add entries
	tr.Add("Moderator", "What are the risks?")
	tr.Add("Alex", "I see opportunities here.")
	tr.Add("Jordan", "But we must consider the downsides.")

	if tr.Len() != 3 {
		t.Errorf("Len() = %d, want 3", tr.Len())
	}

	formatted = tr.Format()
	expected := "Moderator: What are the risks?\nAlex: I see opportunities here.\nJordan: But we must consider the downsides.\n"
	if formatted != expected {
		t.Errorf("Format() = %q, want %q", formatted, expected)
	}

	// Test Entries
	entries := tr.Entries()
	if len(entries) != 3 {
		t.Errorf("Entries() len = %d, want 3", len(entries))
	}
	if entries[0].Speaker != "Moderator" {
		t.Errorf("entries[0].Speaker = %q, want %q", entries[0].Speaker, "Moderator")
	}
}

func TestTranscript_LastModeratorEntry(t *testing.T) {
	tr := NewTranscript("Test Topic")

	// No moderator entry
	_, found := tr.LastModeratorEntry()
	if found {
		t.Error("LastModeratorEntry() found = true, want false")
	}

	tr.Add("Alex", "Hello")
	tr.Add("Moderator", "First question")
	tr.Add("Jordan", "Response")
	tr.Add("Moderator", "Second question")
	tr.Add("Morgan", "Another response")

	entry, found := tr.LastModeratorEntry()
	if !found {
		t.Error("LastModeratorEntry() found = false, want true")
	}
	if entry.Text != "Second question" {
		t.Errorf("LastModeratorEntry().Text = %q, want %q", entry.Text, "Second question")
	}
}

func TestParseRankingResponse(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []string
		wantErr bool
	}{
		{
			name:  "valid JSON array",
			input: `["Alex", "Jordan", "Morgan"]`,
			want:  []string{"Alex", "Jordan", "Morgan"},
		},
		{
			name:  "valid with whitespace",
			input: `  ["Casey", "Alex"]  `,
			want:  []string{"Casey", "Alex"},
		},
		{
			name:    "invalid JSON",
			input:   `Alex, Jordan, Morgan`,
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   ``,
			wantErr: true,
		},
		{
			name:  "empty array",
			input: `[]`,
			want:  []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseRankingResponse(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseRankingResponse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if len(got) != len(tt.want) {
					t.Errorf("ParseRankingResponse() len = %d, want %d", len(got), len(tt.want))
					return
				}
				for i, name := range tt.want {
					if got[i] != name {
						t.Errorf("ParseRankingResponse()[%d] = %q, want %q", i, got[i], name)
					}
				}
			}
		})
	}
}

func TestMapNamesToOrder(t *testing.T) {
	panelists := []*Panelist{
		{Name: "Alex", Personality: "Optimist"},
		{Name: "Jordan", Personality: "Skeptic"},
		{Name: "Morgan", Personality: "Academic"},
	}

	tests := []struct {
		name   string
		names  []string
		expect []string
	}{
		{
			name:   "all names in order",
			names:  []string{"Jordan", "Morgan", "Alex"},
			expect: []string{"Jordan", "Morgan", "Alex"},
		},
		{
			name:   "partial names - missing added at end",
			names:  []string{"Morgan"},
			expect: []string{"Morgan", "Alex", "Jordan"},
		},
		{
			name:   "unknown names ignored",
			names:  []string{"Unknown", "Jordan", "FakeName", "Alex"},
			expect: []string{"Jordan", "Alex", "Morgan"},
		},
		{
			name:   "duplicate names handled",
			names:  []string{"Alex", "Alex", "Jordan"},
			expect: []string{"Alex", "Jordan", "Morgan"},
		},
		{
			name:   "empty names",
			names:  []string{},
			expect: []string{"Alex", "Jordan", "Morgan"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MapNamesToOrder(tt.names, panelists)
			if len(got) != len(tt.expect) {
				t.Errorf("MapNamesToOrder() len = %d, want %d", len(got), len(tt.expect))
				return
			}
			for i, name := range tt.expect {
				if got[i].Name != name {
					t.Errorf("MapNamesToOrder()[%d].Name = %q, want %q", i, got[i].Name, name)
				}
			}
		})
	}
}

func TestCoordinator_FallbackOrder(t *testing.T) {
	panelists := []*Panelist{
		{Name: "Alex"},
		{Name: "Jordan"},
		{Name: "Morgan"},
	}
	transcript := NewTranscript("Test")
	mockLLM := &MockLLMClient{}
	coord := NewCoordinator(panelists, transcript, "Test", mockLLM)

	// Round 1: starts at index 0
	coord.CurrentRound = 1
	order := coord.FallbackOrder()
	expectOrder(t, order, []string{"Alex", "Jordan", "Morgan"})

	// Round 2: starts at index 1
	coord.CurrentRound = 2
	order = coord.FallbackOrder()
	expectOrder(t, order, []string{"Jordan", "Morgan", "Alex"})

	// Round 3: starts at index 2
	coord.CurrentRound = 3
	order = coord.FallbackOrder()
	expectOrder(t, order, []string{"Morgan", "Alex", "Jordan"})

	// Round 4: wraps back to index 0
	coord.CurrentRound = 4
	order = coord.FallbackOrder()
	expectOrder(t, order, []string{"Alex", "Jordan", "Morgan"})
}

func TestCoordinator_SelectSpeakingOrder_WithMock(t *testing.T) {
	panelists := []*Panelist{
		{Name: "Alex", Personality: "Optimist"},
		{Name: "Jordan", Personality: "Skeptic"},
		{Name: "Morgan", Personality: "Academic"},
	}
	transcript := NewTranscript("Test")

	// Test with successful ranking
	mockLLM := &MockLLMClient{
		Response: `["Jordan", "Morgan", "Alex"]`,
	}
	coord := NewCoordinator(panelists, transcript, "Test", mockLLM)
	coord.CurrentRound = 1

	order := coord.SelectSpeakingOrder(context.Background(), "What are the risks?")
	expectOrder(t, order, []string{"Jordan", "Morgan", "Alex"})
}

func TestCoordinator_SelectSpeakingOrder_FallbackOnError(t *testing.T) {
	panelists := []*Panelist{
		{Name: "Alex"},
		{Name: "Jordan"},
		{Name: "Morgan"},
	}
	transcript := NewTranscript("Test")

	// Test fallback when LLM returns invalid JSON
	mockLLM := &MockLLMClient{
		Response: `invalid json`,
	}
	coord := NewCoordinator(panelists, transcript, "Test", mockLLM)
	coord.CurrentRound = 1

	order := coord.SelectSpeakingOrder(context.Background(), "What are the risks?")
	// Should fall back to rotation order
	expectOrder(t, order, []string{"Alex", "Jordan", "Morgan"})
}

func TestBuildRankingPrompt(t *testing.T) {
	panelists := []*Panelist{
		{Name: "Alex", Personality: "Tech enthusiast"},
		{Name: "Jordan", Personality: "Skeptic"},
	}

	prompt := BuildRankingPrompt("What about AI safety?", panelists)

	// Check that prompt contains key elements
	if !contains(prompt, "What about AI safety?") {
		t.Error("Prompt should contain the question")
	}
	if !contains(prompt, "Alex: Tech enthusiast") {
		t.Error("Prompt should contain Alex's description")
	}
	if !contains(prompt, "Jordan: Skeptic") {
		t.Error("Prompt should contain Jordan's description")
	}
	if !contains(prompt, "JSON array") {
		t.Error("Prompt should mention JSON array format")
	}
}

func TestBuildPanelistPrompt(t *testing.T) {
	transcript := NewTranscript("AI Future")
	transcript.Add("Moderator", "What do you think?")

	prompt := BuildPanelistPrompt("Alex", "Optimistic tech enthusiast", transcript, "What about risks?")

	if !contains(prompt, "You are Alex") {
		t.Error("Prompt should contain panelist name")
	}
	if !contains(prompt, "AI Future") {
		t.Error("Prompt should contain topic")
	}
	if !contains(prompt, "Optimistic tech enthusiast") {
		t.Error("Prompt should contain personality")
	}
	if !contains(prompt, "What about risks?") {
		t.Error("Prompt should contain the question")
	}
	if !contains(prompt, "2-4 sentences") {
		t.Error("Prompt should mention response length guideline")
	}
}

func TestFormatNames(t *testing.T) {
	panelists := []*Panelist{
		{Name: "Alex"},
		{Name: "Jordan"},
		{Name: "Morgan"},
	}

	result := FormatNames(panelists)
	expected := "Alex -> Jordan -> Morgan"
	if result != expected {
		t.Errorf("FormatNames() = %q, want %q", result, expected)
	}

	// Empty list
	result = FormatNames([]*Panelist{})
	if result != "" {
		t.Errorf("FormatNames([]) = %q, want empty", result)
	}
}

func TestDefaultPanelists(t *testing.T) {
	defaults := DefaultPanelists()

	if len(defaults) != 4 {
		t.Errorf("DefaultPanelists() len = %d, want 4", len(defaults))
	}

	expectedNames := []string{"Alex", "Jordan", "Morgan", "Casey"}
	expectedVoices := []string{"alloy", "echo", "onyx", "nova"}

	for i, d := range defaults {
		if d.Name != expectedNames[i] {
			t.Errorf("defaults[%d].Name = %q, want %q", i, d.Name, expectedNames[i])
		}
		if d.Voice != expectedVoices[i] {
			t.Errorf("defaults[%d].Voice = %q, want %q", i, d.Voice, expectedVoices[i])
		}
		if d.Personality == "" {
			t.Errorf("defaults[%d].Personality should not be empty", i)
		}
	}
}

// Helper functions

func expectOrder(t *testing.T, got []*Panelist, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("order len = %d, want %d", len(got), len(want))
		return
	}
	for i, name := range want {
		if got[i].Name != name {
			t.Errorf("order[%d].Name = %q, want %q", i, got[i].Name, name)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
