// Package main provides the livekit-agent-panel command.
package main

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// Entry represents a single transcript entry.
type Entry struct {
	Speaker   string    // "Moderator", "Alex", etc.
	Text      string    // What was said
	Timestamp time.Time // When it was said
}

// Transcript tracks the conversation for all panelists to see.
type Transcript struct {
	entries []Entry
	topic   string
	mu      sync.RWMutex
}

// NewTranscript creates a new transcript for the given topic.
func NewTranscript(topic string) *Transcript {
	return &Transcript{
		entries: make([]Entry, 0),
		topic:   topic,
	}
}

// Add adds a new entry to the transcript.
func (t *Transcript) Add(speaker, text string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.entries = append(t.entries, Entry{
		Speaker:   speaker,
		Text:      text,
		Timestamp: time.Now(),
	})
}

// Entries returns a copy of all entries.
func (t *Transcript) Entries() []Entry {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make([]Entry, len(t.entries))
	copy(result, t.entries)
	return result
}

// Format returns the transcript formatted for LLM context.
func (t *Transcript) Format() string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if len(t.entries) == 0 {
		return "(No previous discussion yet)"
	}

	var sb strings.Builder
	for _, entry := range t.entries {
		sb.WriteString(fmt.Sprintf("%s: %s\n", entry.Speaker, entry.Text))
	}
	return sb.String()
}

// LastModeratorEntry returns the most recent moderator entry, if any.
func (t *Transcript) LastModeratorEntry() (Entry, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	for i := len(t.entries) - 1; i >= 0; i-- {
		if t.entries[i].Speaker == "Moderator" {
			return t.entries[i], true
		}
	}
	return Entry{}, false
}

// Len returns the number of entries.
func (t *Transcript) Len() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.entries)
}

// Topic returns the discussion topic.
func (t *Transcript) Topic() string {
	return t.topic
}
