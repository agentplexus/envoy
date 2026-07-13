// Package panel provides JSON IR types for panel discussion scheduling and output.
package panel

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// ScheduleVersion is the current schedule format version.
const ScheduleVersion = "1.0"

// Schedule defines the structure of a panel discussion.
type Schedule struct {
	// Version is the schema version (e.g., "1.0").
	Version string `json:"version"`

	// Topic is the main discussion topic.
	Topic string `json:"topic"`

	// DurationMinutes is the target duration (advisory, not enforced).
	DurationMinutes int `json:"duration_minutes,omitempty"`

	// Moderator configures the AI moderator.
	Moderator ModeratorSchedule `json:"moderator"`

	// Panelists configures the AI panelists.
	Panelists []PanelistSchedule `json:"panelists"`

	// Segments defines the discussion flow.
	Segments []Segment `json:"segments"`

	// Settings contains optional configuration.
	Settings *ScheduleSettings `json:"settings,omitempty"`
}

// ModeratorSchedule configures the panel moderator.
type ModeratorSchedule struct {
	// Name is the moderator's display name.
	Name string `json:"name"`

	// Personality describes the moderator's style.
	Personality string `json:"personality"`

	// Voice is the TTS voice ID.
	Voice string `json:"voice"`

	// AvatarID is the HeyGen avatar ID (optional).
	AvatarID string `json:"avatar_id,omitempty"`

	// Background is the moderator's professional background.
	Background string `json:"background,omitempty"`
}

// PanelistSchedule configures a panel participant.
type PanelistSchedule struct {
	// Name is the panelist's display name.
	Name string `json:"name"`

	// Personality describes the panelist's perspective and style.
	Personality string `json:"personality"`

	// Voice is the TTS voice ID.
	Voice string `json:"voice"`

	// AvatarID is the HeyGen avatar ID (optional).
	AvatarID string `json:"avatar_id,omitempty"`

	// Background is the panelist's professional background for introductions.
	Background string `json:"background,omitempty"`

	// Expertise lists areas of expertise for relevance-based ordering.
	Expertise []string `json:"expertise,omitempty"`

	// Slides contains slides this panelist can share (optional).
	Slides []Slide `json:"slides,omitempty"`
}

// Slide represents a visual asset a panelist can share.
type Slide struct {
	// ID is a unique identifier for the slide.
	ID string `json:"id"`

	// Title is the slide title (for reference).
	Title string `json:"title"`

	// ImagePath is the local path to the slide image.
	ImagePath string `json:"image_path,omitempty"`

	// ImageURL is a URL to fetch the slide image.
	ImageURL string `json:"image_url,omitempty"`

	// Keywords trigger automatic slide sharing when mentioned.
	Keywords []string `json:"keywords,omitempty"`

	// ShowDuring specifies segment types when this slide can be shown.
	ShowDuring []SegmentType `json:"show_during,omitempty"`
}

// SegmentType identifies the type of discussion segment.
type SegmentType string

const (
	// SegmentModeratorIntro is the moderator's opening statement.
	SegmentModeratorIntro SegmentType = "moderator_intro"

	// SegmentPanelistBackgrounds has panelists introduce themselves.
	SegmentPanelistBackgrounds SegmentType = "panelist_backgrounds"

	// SegmentDiscussionRound is a question-and-answer round.
	SegmentDiscussionRound SegmentType = "discussion_round"

	// SegmentOpenDiscussion allows free-form discussion.
	SegmentOpenDiscussion SegmentType = "open_discussion"

	// SegmentClosing is the moderator's closing statement.
	SegmentClosing SegmentType = "closing"
)

// ResponseOrder specifies how panelist speaking order is determined.
type ResponseOrder string

const (
	// OrderRelevance orders panelists by relevance to the question.
	OrderRelevance ResponseOrder = "relevance"

	// OrderRotation rotates through panelists.
	OrderRotation ResponseOrder = "rotation"

	// OrderRandom randomizes the order.
	OrderRandom ResponseOrder = "random"

	// OrderFixed uses the order specified in FixedOrder.
	OrderFixed ResponseOrder = "fixed"
)

// BackgroundStyle specifies how panelist introductions are delivered.
type BackgroundStyle string

const (
	// StyleBrief is a 1-sentence introduction.
	StyleBrief BackgroundStyle = "brief"

	// StyleDetailed is a 2-3 sentence introduction with background.
	StyleDetailed BackgroundStyle = "detailed"

	// StyleFull is a comprehensive introduction.
	StyleFull BackgroundStyle = "full"
)

// Segment defines a portion of the panel discussion.
type Segment struct {
	// Type identifies what kind of segment this is.
	Type SegmentType `json:"type"`

	// DurationSeconds is the target duration (advisory).
	DurationSeconds int `json:"duration_seconds,omitempty"`

	// Question is the moderator's question (for discussion_round).
	// If nil/empty with GenerateFromContext=true, question is generated.
	Question string `json:"question,omitempty"`

	// GenerateFromContext generates the question from transcript context.
	GenerateFromContext bool `json:"generate_from_context,omitempty"`

	// ResponseOrder specifies how panelists are ordered.
	ResponseOrder ResponseOrder `json:"response_order,omitempty"`

	// FixedOrder specifies panelist names in order (for OrderFixed).
	FixedOrder []string `json:"fixed_order,omitempty"`

	// Style specifies the introduction style (for panelist_backgrounds).
	Style BackgroundStyle `json:"style,omitempty"`

	// Prompt is a custom prompt for the segment (optional).
	Prompt string `json:"prompt,omitempty"`
}

// ScheduleSettings contains optional schedule configuration.
type ScheduleSettings struct {
	// SpeakerPauseMs is the pause between speakers in milliseconds.
	SpeakerPauseMs int `json:"speaker_pause_ms,omitempty"`

	// MaxResponseWords limits panelist response length.
	MaxResponseWords int `json:"max_response_words,omitempty"`

	// OutputFile specifies where to write the output JSON.
	OutputFile string `json:"output_file,omitempty"`

	// Recording configures video recording.
	Recording *RecordingSettings `json:"recording,omitempty"`
}

// RecordingSettings configures panel recording.
type RecordingSettings struct {
	// Enabled turns recording on/off.
	Enabled bool `json:"enabled"`

	// Format is the output format ("mp4" or "webm").
	Format string `json:"format,omitempty"`

	// Layout is the recording layout ("grid", "speaker", "single-speaker").
	Layout string `json:"layout,omitempty"`

	// FilePath is the local file path for the recording.
	FilePath string `json:"file_path,omitempty"`

	// S3Bucket is the S3 bucket for upload (optional).
	S3Bucket string `json:"s3_bucket,omitempty"`

	// S3Region is the S3 region.
	S3Region string `json:"s3_region,omitempty"`
}

// LoadSchedule reads a schedule from a JSON file.
func LoadSchedule(path string) (*Schedule, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read schedule file: %w", err)
	}

	var schedule Schedule
	if err := json.Unmarshal(data, &schedule); err != nil {
		return nil, fmt.Errorf("parse schedule: %w", err)
	}

	if err := schedule.Validate(); err != nil {
		return nil, fmt.Errorf("validate schedule: %w", err)
	}

	return &schedule, nil
}

// Validate checks the schedule for errors.
func (s *Schedule) Validate() error {
	if s.Topic == "" {
		return fmt.Errorf("topic is required")
	}
	if s.Moderator.Name == "" {
		return fmt.Errorf("moderator name is required")
	}
	if len(s.Panelists) == 0 {
		return fmt.Errorf("at least one panelist is required")
	}
	for i, p := range s.Panelists {
		if p.Name == "" {
			return fmt.Errorf("panelist %d: name is required", i+1)
		}
	}
	if len(s.Segments) == 0 {
		return fmt.Errorf("at least one segment is required")
	}
	for i, seg := range s.Segments {
		if seg.Type == "" {
			return fmt.Errorf("segment %d: type is required", i+1)
		}
	}
	return nil
}

// SpeakerPause returns the configured speaker pause duration.
func (s *Schedule) SpeakerPause() time.Duration {
	if s.Settings != nil && s.Settings.SpeakerPauseMs > 0 {
		return time.Duration(s.Settings.SpeakerPauseMs) * time.Millisecond
	}
	return 1500 * time.Millisecond // default
}

// DefaultSchedule creates a default schedule for a topic.
func DefaultSchedule(topic string, numRounds int) *Schedule {
	segments := []Segment{
		{Type: SegmentModeratorIntro, DurationSeconds: 30},
		{Type: SegmentPanelistBackgrounds, Style: StyleDetailed, DurationSeconds: 120},
	}

	for i := 0; i < numRounds; i++ {
		segments = append(segments, Segment{
			Type:                SegmentDiscussionRound,
			GenerateFromContext: true,
			ResponseOrder:       OrderRelevance,
		})
	}

	segments = append(segments, Segment{Type: SegmentClosing, DurationSeconds: 60})

	return &Schedule{
		Version:  ScheduleVersion,
		Topic:    topic,
		Segments: segments,
	}
}

// PanelistNames returns all panelist names.
func (s *Schedule) PanelistNames() []string {
	names := make([]string, len(s.Panelists))
	for i, p := range s.Panelists {
		names[i] = p.Name
	}
	return names
}
