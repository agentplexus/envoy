package panel

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestNewOutput(t *testing.T) {
	out := NewOutput("session-123", "AI Agents", "room-456")

	if out.Version != OutputVersion {
		t.Errorf("Version = %q, want %q", out.Version, OutputVersion)
	}
	if out.SessionID != "session-123" {
		t.Errorf("SessionID = %q, want %q", out.SessionID, "session-123")
	}
	if out.Metadata.Topic != "AI Agents" {
		t.Errorf("Metadata.Topic = %q, want %q", out.Metadata.Topic, "AI Agents")
	}
	if out.Metadata.RoomName != "room-456" {
		t.Errorf("Metadata.RoomName = %q, want %q", out.Metadata.RoomName, "room-456")
	}
}

func TestOutputAddEntry(t *testing.T) {
	out := NewOutput("test", "Topic", "room")

	out.StartSegment(SegmentModeratorIntro, 0, "")
	out.AddEntry("Sam", "moderator", "Welcome everyone!", SegmentModeratorIntro, 0)
	out.EndSegment(nil)

	if len(out.Transcript) != 1 {
		t.Fatalf("len(Transcript) = %d, want 1", len(out.Transcript))
	}

	entry := out.Transcript[0]
	if entry.Speaker != "Sam" {
		t.Errorf("Speaker = %q, want %q", entry.Speaker, "Sam")
	}
	if entry.Role != "moderator" {
		t.Errorf("Role = %q, want %q", entry.Role, "moderator")
	}
	if entry.WordCount != 2 {
		t.Errorf("WordCount = %d, want 2", entry.WordCount)
	}

	if len(out.Segments) != 1 {
		t.Fatalf("len(Segments) = %d, want 1", len(out.Segments))
	}
	if len(out.Segments[0].Entries) != 1 {
		t.Errorf("len(Segment.Entries) = %d, want 1", len(out.Segments[0].Entries))
	}
}

func TestOutputDiscussionRound(t *testing.T) {
	out := NewOutput("test", "Topic", "room")

	out.StartSegment(SegmentDiscussionRound, 1, "What do you think?")
	out.AddEntry("Sam", "moderator", "What do you think?", SegmentDiscussionRound, 1)
	out.AddEntry("Alex", "panelist", "I think it's great.", SegmentDiscussionRound, 1)
	out.AddEntry("Jordan", "panelist", "I have concerns.", SegmentDiscussionRound, 1)
	out.EndSegment([]string{"Alex", "Jordan"})

	if len(out.Transcript) != 3 {
		t.Errorf("len(Transcript) = %d, want 3", len(out.Transcript))
	}

	seg := out.Segments[0]
	if seg.Type != SegmentDiscussionRound {
		t.Errorf("Type = %q, want %q", seg.Type, SegmentDiscussionRound)
	}
	if seg.RoundNumber != 1 {
		t.Errorf("RoundNumber = %d, want 1", seg.RoundNumber)
	}
	if seg.Question != "What do you think?" {
		t.Errorf("Question = %q, want %q", seg.Question, "What do you think?")
	}
	if len(seg.SpeakingOrder) != 2 {
		t.Fatalf("len(SpeakingOrder) = %d, want 2", len(seg.SpeakingOrder))
	}
	if seg.SpeakingOrder[0] != "Alex" || seg.SpeakingOrder[1] != "Jordan" {
		t.Errorf("SpeakingOrder = %v, want [Alex Jordan]", seg.SpeakingOrder)
	}
}

func TestOutputFinalize(t *testing.T) {
	out := NewOutput("test", "Topic", "room")
	out.SetParticipants(
		OutputParticipant{Name: "Sam", Role: "moderator"},
		[]OutputParticipant{
			{Name: "Alex", Role: "panelist"},
			{Name: "Jordan", Role: "panelist"},
		},
	)

	// Add some entries
	out.StartSegment(SegmentModeratorIntro, 0, "")
	out.AddEntry("Sam", "moderator", "Welcome to the panel.", SegmentModeratorIntro, 0)
	out.EndSegment(nil)

	out.StartSegment(SegmentDiscussionRound, 1, "Question?")
	out.AddEntry("Sam", "moderator", "What are your thoughts?", SegmentDiscussionRound, 1)
	out.AddEntry("Alex", "panelist", "I think this is great and exciting.", SegmentDiscussionRound, 1)
	out.AddEntry("Jordan", "panelist", "I have some concerns about this approach.", SegmentDiscussionRound, 1)
	out.EndSegment([]string{"Alex", "Jordan"})

	out.Finalize()

	if out.Metadata.TotalEntries != 4 {
		t.Errorf("TotalEntries = %d, want 4", out.Metadata.TotalEntries)
	}
	if out.Metadata.TotalRounds != 1 {
		t.Errorf("TotalRounds = %d, want 1", out.Metadata.TotalRounds)
	}
	if out.Metadata.EndedAt == nil {
		t.Error("EndedAt should be set")
	}

	// Check participant stats
	if out.Participants.Moderator.EntryCount != 2 {
		t.Errorf("Moderator.EntryCount = %d, want 2", out.Participants.Moderator.EntryCount)
	}
	if out.Participants.Panelists[0].EntryCount != 1 {
		t.Errorf("Alex.EntryCount = %d, want 1", out.Participants.Panelists[0].EntryCount)
	}
}

func TestOutputSave(t *testing.T) {
	out := NewOutput("test-session", "Test Topic", "test-room")
	out.SetParticipants(
		OutputParticipant{Name: "Sam", Role: "moderator"},
		[]OutputParticipant{{Name: "Alex", Role: "panelist"}},
	)
	out.StartSegment(SegmentModeratorIntro, 0, "")
	out.AddEntry("Sam", "moderator", "Hello!", SegmentModeratorIntro, 0)
	out.EndSegment(nil)
	out.Finalize()

	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "output.json")

	if err := out.Save(outPath); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Read and verify
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output file: %v", err)
	}

	var loaded Output
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}

	if loaded.SessionID != "test-session" {
		t.Errorf("SessionID = %q, want %q", loaded.SessionID, "test-session")
	}
	if len(loaded.Transcript) != 1 {
		t.Errorf("len(Transcript) = %d, want 1", len(loaded.Transcript))
	}
}

func TestOutputJSON(t *testing.T) {
	out := NewOutput("test", "Topic", "room")
	out.AddEntry("Sam", "moderator", "Hello", SegmentModeratorIntro, 0)

	data, err := out.JSON()
	if err != nil {
		t.Fatalf("JSON() error = %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal JSON: %v", err)
	}

	if parsed["session_id"] != "test" {
		t.Errorf("session_id = %v, want %q", parsed["session_id"], "test")
	}
}

func TestCountWords(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"", 0},
		{"hello", 1},
		{"hello world", 2},
		{"  hello   world  ", 2},
		{"one two three four five", 5},
		{"word\twith\ttabs", 3},
		{"word\nwith\nnewlines", 3},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := countWords(tt.input)
			if got != tt.want {
				t.Errorf("countWords(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestOutputCurrentRound(t *testing.T) {
	out := NewOutput("test", "Topic", "room")

	if round := out.CurrentRound(); round != 0 {
		t.Errorf("initial CurrentRound() = %d, want 0", round)
	}

	out.StartSegment(SegmentModeratorIntro, 0, "")
	out.EndSegment(nil)

	if round := out.CurrentRound(); round != 0 {
		t.Errorf("after intro CurrentRound() = %d, want 0", round)
	}

	out.StartSegment(SegmentDiscussionRound, 1, "Q1")
	out.EndSegment(nil)

	if round := out.CurrentRound(); round != 1 {
		t.Errorf("after round 1 CurrentRound() = %d, want 1", round)
	}

	out.StartSegment(SegmentDiscussionRound, 2, "Q2")
	out.EndSegment(nil)

	if round := out.CurrentRound(); round != 2 {
		t.Errorf("after round 2 CurrentRound() = %d, want 2", round)
	}
}
