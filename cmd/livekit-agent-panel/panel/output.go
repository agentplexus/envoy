package panel

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// OutputVersion is the current output format version.
const OutputVersion = "1.0"

// Output captures the full panel discussion for analysis and replay.
type Output struct {
	// Version is the schema version (e.g., "1.0").
	Version string `json:"version"`

	// SessionID uniquely identifies this panel session.
	SessionID string `json:"session_id"`

	// ScheduleFile is the path to the schedule that was used (if any).
	ScheduleFile string `json:"schedule_file,omitempty"`

	// Metadata contains session information.
	Metadata OutputMetadata `json:"metadata"`

	// Participants lists all panel participants.
	Participants OutputParticipants `json:"participants"`

	// Segments contains the structured discussion flow.
	Segments []OutputSegment `json:"segments"`

	// Transcript contains all entries in chronological order.
	Transcript []OutputEntry `json:"transcript"`

	mu sync.Mutex
}

// OutputMetadata contains session-level information.
type OutputMetadata struct {
	// Topic is the discussion topic.
	Topic string `json:"topic"`

	// StartedAt is when the panel began.
	StartedAt time.Time `json:"started_at"`

	// EndedAt is when the panel ended.
	EndedAt *time.Time `json:"ended_at,omitempty"`

	// DurationSeconds is the total duration.
	DurationSeconds float64 `json:"duration_seconds,omitempty"`

	// RoomName is the LiveKit room name.
	RoomName string `json:"room_name,omitempty"`

	// RecordingURL is the URL to the recording (if available).
	RecordingURL string `json:"recording_url,omitempty"`

	// TotalRounds is the number of discussion rounds completed.
	TotalRounds int `json:"total_rounds"`

	// TotalEntries is the number of transcript entries.
	TotalEntries int `json:"total_entries"`
}

// OutputParticipants lists all panel participants.
type OutputParticipants struct {
	// Moderator is the panel moderator.
	Moderator OutputParticipant `json:"moderator"`

	// Panelists are the panel participants.
	Panelists []OutputParticipant `json:"panelists"`
}

// OutputParticipant describes a participant.
type OutputParticipant struct {
	// Name is the participant's display name.
	Name string `json:"name"`

	// Role is "moderator" or "panelist".
	Role string `json:"role"`

	// AvatarID is the HeyGen avatar ID (if used).
	AvatarID string `json:"avatar_id,omitempty"`

	// Voice is the TTS voice ID.
	Voice string `json:"voice,omitempty"`

	// Background is the participant's background.
	Background string `json:"background,omitempty"`

	// EntryCount is how many times they spoke.
	EntryCount int `json:"entry_count"`

	// TotalWords is the total word count.
	TotalWords int `json:"total_words"`
}

// OutputSegment captures a structured portion of the discussion.
type OutputSegment struct {
	// Type is the segment type.
	Type SegmentType `json:"type"`

	// RoundNumber is the round number (for discussion_round).
	RoundNumber int `json:"round_number,omitempty"`

	// Question is the moderator's question (for discussion_round).
	Question string `json:"question,omitempty"`

	// StartedAt is when the segment began.
	StartedAt time.Time `json:"started_at"`

	// EndedAt is when the segment ended.
	EndedAt *time.Time `json:"ended_at,omitempty"`

	// DurationSeconds is the segment duration.
	DurationSeconds float64 `json:"duration_seconds,omitempty"`

	// SpeakingOrder lists panelists in order they spoke.
	SpeakingOrder []string `json:"speaking_order,omitempty"`

	// Entries are the transcript entries in this segment.
	Entries []OutputEntry `json:"entries"`
}

// OutputEntry is a single transcript entry.
type OutputEntry struct {
	// Speaker is the participant name.
	Speaker string `json:"speaker"`

	// Role is "moderator" or "panelist".
	Role string `json:"role"`

	// Text is what was said.
	Text string `json:"text"`

	// Timestamp is when it was said.
	Timestamp time.Time `json:"timestamp"`

	// DurationSeconds is how long it took to speak.
	DurationSeconds float64 `json:"duration_seconds,omitempty"`

	// WordCount is the number of words.
	WordCount int `json:"word_count"`

	// Segment is the segment type this entry belongs to.
	Segment SegmentType `json:"segment"`

	// RoundNumber is the round number (for discussion entries).
	RoundNumber int `json:"round_number,omitempty"`
}

// NewOutput creates a new output for a panel session.
func NewOutput(sessionID, topic, roomName string) *Output {
	return &Output{
		Version:   OutputVersion,
		SessionID: sessionID,
		Metadata: OutputMetadata{
			Topic:     topic,
			StartedAt: time.Now(),
			RoomName:  roomName,
		},
		Segments:   make([]OutputSegment, 0),
		Transcript: make([]OutputEntry, 0),
	}
}

// SetScheduleFile sets the schedule file path.
func (o *Output) SetScheduleFile(path string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.ScheduleFile = path
}

// SetParticipants sets the participant information.
func (o *Output) SetParticipants(moderator OutputParticipant, panelists []OutputParticipant) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.Participants = OutputParticipants{
		Moderator: moderator,
		Panelists: panelists,
	}
}

// StartSegment begins a new segment.
func (o *Output) StartSegment(segType SegmentType, roundNumber int, question string) {
	o.mu.Lock()
	defer o.mu.Unlock()

	seg := OutputSegment{
		Type:        segType,
		RoundNumber: roundNumber,
		Question:    question,
		StartedAt:   time.Now(),
		Entries:     make([]OutputEntry, 0),
	}
	o.Segments = append(o.Segments, seg)
}

// EndSegment closes the current segment.
func (o *Output) EndSegment(speakingOrder []string) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if len(o.Segments) == 0 {
		return
	}

	seg := &o.Segments[len(o.Segments)-1]
	now := time.Now()
	seg.EndedAt = &now
	seg.DurationSeconds = now.Sub(seg.StartedAt).Seconds()
	seg.SpeakingOrder = speakingOrder
}

// AddEntry adds a transcript entry to the current segment and full transcript.
func (o *Output) AddEntry(speaker, role, text string, segType SegmentType, roundNumber int) {
	o.mu.Lock()
	defer o.mu.Unlock()

	entry := OutputEntry{
		Speaker:     speaker,
		Role:        role,
		Text:        text,
		Timestamp:   time.Now(),
		WordCount:   countWords(text),
		Segment:     segType,
		RoundNumber: roundNumber,
	}

	// Add to full transcript
	o.Transcript = append(o.Transcript, entry)

	// Add to current segment
	if len(o.Segments) > 0 {
		seg := &o.Segments[len(o.Segments)-1]
		seg.Entries = append(seg.Entries, entry)
	}
}

// Finalize completes the output with final statistics.
func (o *Output) Finalize() {
	o.mu.Lock()
	defer o.mu.Unlock()

	now := time.Now()
	o.Metadata.EndedAt = &now
	o.Metadata.DurationSeconds = now.Sub(o.Metadata.StartedAt).Seconds()
	o.Metadata.TotalEntries = len(o.Transcript)

	// Count rounds
	for _, seg := range o.Segments {
		if seg.Type == SegmentDiscussionRound {
			o.Metadata.TotalRounds++
		}
	}

	// Update participant stats
	speakerStats := make(map[string]*struct {
		count int
		words int
	})
	for _, entry := range o.Transcript {
		stats, ok := speakerStats[entry.Speaker]
		if !ok {
			stats = &struct {
				count int
				words int
			}{}
			speakerStats[entry.Speaker] = stats
		}
		stats.count++
		stats.words += entry.WordCount
	}

	if stats, ok := speakerStats[o.Participants.Moderator.Name]; ok {
		o.Participants.Moderator.EntryCount = stats.count
		o.Participants.Moderator.TotalWords = stats.words
	}

	for i := range o.Participants.Panelists {
		name := o.Participants.Panelists[i].Name
		if stats, ok := speakerStats[name]; ok {
			o.Participants.Panelists[i].EntryCount = stats.count
			o.Participants.Panelists[i].TotalWords = stats.words
		}
	}
}

// Save writes the output to a JSON file.
func (o *Output) Save(path string) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	data, err := json.MarshalIndent(o, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal output: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write output file: %w", err)
	}

	return nil
}

// JSON returns the output as formatted JSON.
func (o *Output) JSON() ([]byte, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	return json.MarshalIndent(o, "", "  ")
}

// countWords counts words in a string.
func countWords(s string) int {
	if s == "" {
		return 0
	}
	count := 0
	inWord := false
	for _, r := range s {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			inWord = false
		} else if !inWord {
			inWord = true
			count++
		}
	}
	return count
}

// CurrentRound returns the current round number based on segments.
func (o *Output) CurrentRound() int {
	o.mu.Lock()
	defer o.mu.Unlock()

	round := 0
	for _, seg := range o.Segments {
		if seg.Type == SegmentDiscussionRound {
			round++
		}
	}
	return round
}
