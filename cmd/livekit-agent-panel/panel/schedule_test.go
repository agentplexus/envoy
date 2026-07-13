package panel

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScheduleValidation(t *testing.T) {
	tests := []struct {
		name    string
		sched   Schedule
		wantErr bool
	}{
		{
			name: "valid schedule",
			sched: Schedule{
				Version: "1.0",
				Topic:   "Test Topic",
				Moderator: ModeratorSchedule{
					Name: "Sam",
				},
				Panelists: []PanelistSchedule{
					{Name: "Alex"},
				},
				Segments: []Segment{
					{Type: SegmentModeratorIntro},
				},
			},
			wantErr: false,
		},
		{
			name: "missing topic",
			sched: Schedule{
				Moderator: ModeratorSchedule{Name: "Sam"},
				Panelists: []PanelistSchedule{{Name: "Alex"}},
				Segments:  []Segment{{Type: SegmentModeratorIntro}},
			},
			wantErr: true,
		},
		{
			name: "missing moderator name",
			sched: Schedule{
				Topic:     "Test",
				Panelists: []PanelistSchedule{{Name: "Alex"}},
				Segments:  []Segment{{Type: SegmentModeratorIntro}},
			},
			wantErr: true,
		},
		{
			name: "no panelists",
			sched: Schedule{
				Topic:     "Test",
				Moderator: ModeratorSchedule{Name: "Sam"},
				Segments:  []Segment{{Type: SegmentModeratorIntro}},
			},
			wantErr: true,
		},
		{
			name: "no segments",
			sched: Schedule{
				Topic:     "Test",
				Moderator: ModeratorSchedule{Name: "Sam"},
				Panelists: []PanelistSchedule{{Name: "Alex"}},
			},
			wantErr: true,
		},
		{
			name: "panelist missing name",
			sched: Schedule{
				Topic:     "Test",
				Moderator: ModeratorSchedule{Name: "Sam"},
				Panelists: []PanelistSchedule{{Name: ""}},
				Segments:  []Segment{{Type: SegmentModeratorIntro}},
			},
			wantErr: true,
		},
		{
			name: "segment missing type",
			sched: Schedule{
				Topic:     "Test",
				Moderator: ModeratorSchedule{Name: "Sam"},
				Panelists: []PanelistSchedule{{Name: "Alex"}},
				Segments:  []Segment{{}},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.sched.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLoadSchedule(t *testing.T) {
	// Create a temporary schedule file
	schedJSON := `{
		"version": "1.0",
		"topic": "AI Agents",
		"moderator": {"name": "Sam", "voice": "shimmer"},
		"panelists": [
			{"name": "Alex", "voice": "alloy", "personality": "Optimistic"},
			{"name": "Jordan", "voice": "echo", "personality": "Skeptic"}
		],
		"segments": [
			{"type": "moderator_intro"},
			{"type": "panelist_backgrounds", "style": "detailed"},
			{"type": "discussion_round", "question": "What excites you?"},
			{"type": "closing"}
		]
	}`

	tmpDir := t.TempDir()
	schedPath := filepath.Join(tmpDir, "schedule.json")
	if err := os.WriteFile(schedPath, []byte(schedJSON), 0600); err != nil { //nolint:gosec // G306: test file
		t.Fatalf("write temp file: %v", err)
	}

	sched, err := LoadSchedule(schedPath)
	if err != nil {
		t.Fatalf("LoadSchedule() error = %v", err)
	}

	if sched.Topic != "AI Agents" {
		t.Errorf("Topic = %q, want %q", sched.Topic, "AI Agents")
	}
	if sched.Moderator.Name != "Sam" {
		t.Errorf("Moderator.Name = %q, want %q", sched.Moderator.Name, "Sam")
	}
	if len(sched.Panelists) != 2 {
		t.Errorf("len(Panelists) = %d, want 2", len(sched.Panelists))
	}
	if len(sched.Segments) != 4 {
		t.Errorf("len(Segments) = %d, want 4", len(sched.Segments))
	}
}

func TestLoadSchedule_FileNotFound(t *testing.T) {
	_, err := LoadSchedule("/nonexistent/path.json")
	if err == nil {
		t.Error("LoadSchedule() expected error for missing file")
	}
}

func TestLoadSchedule_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "invalid.json")
	if err := os.WriteFile(path, []byte("not json"), 0600); err != nil { //nolint:gosec // G306: test file
		t.Fatalf("write temp file: %v", err)
	}

	_, err := LoadSchedule(path)
	if err == nil {
		t.Error("LoadSchedule() expected error for invalid JSON")
	}
}

func TestDefaultSchedule(t *testing.T) {
	sched := DefaultSchedule("Test Topic", 3)

	if sched.Version != ScheduleVersion {
		t.Errorf("Version = %q, want %q", sched.Version, ScheduleVersion)
	}
	if sched.Topic != "Test Topic" {
		t.Errorf("Topic = %q, want %q", sched.Topic, "Test Topic")
	}

	// Should have: intro, backgrounds, 3 rounds, closing = 6 segments
	if len(sched.Segments) != 6 {
		t.Errorf("len(Segments) = %d, want 6", len(sched.Segments))
	}

	// Count discussion rounds
	rounds := 0
	for _, seg := range sched.Segments {
		if seg.Type == SegmentDiscussionRound {
			rounds++
		}
	}
	if rounds != 3 {
		t.Errorf("discussion rounds = %d, want 3", rounds)
	}
}

func TestScheduleSpeakerPause(t *testing.T) {
	// Default pause
	sched := &Schedule{}
	if pause := sched.SpeakerPause(); pause.Milliseconds() != 1500 {
		t.Errorf("default SpeakerPause() = %v, want 1500ms", pause)
	}

	// Custom pause
	sched.Settings = &ScheduleSettings{SpeakerPauseMs: 2000}
	if pause := sched.SpeakerPause(); pause.Milliseconds() != 2000 {
		t.Errorf("custom SpeakerPause() = %v, want 2000ms", pause)
	}
}

func TestSchedulePanelistNames(t *testing.T) {
	sched := &Schedule{
		Panelists: []PanelistSchedule{
			{Name: "Alex"},
			{Name: "Jordan"},
			{Name: "Morgan"},
		},
	}

	names := sched.PanelistNames()
	if len(names) != 3 {
		t.Fatalf("len(names) = %d, want 3", len(names))
	}
	if names[0] != "Alex" || names[1] != "Jordan" || names[2] != "Morgan" {
		t.Errorf("names = %v, want [Alex Jordan Morgan]", names)
	}
}
