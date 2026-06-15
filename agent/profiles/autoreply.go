// Copyright 2025 John Wang. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package profiles

import (
	"context"
	"time"
)

// AutoReplyConfig configures the auto-reply behavior.
type AutoReplyConfig struct {
	// Enabled turns on auto-reply mode.
	Enabled bool

	// CommentaryMode controls inter-tool commentary verbosity.
	CommentaryMode CommentaryMode

	// InterToolDelay is the delay between tool executions for readability.
	InterToolDelay time.Duration

	// PersistCommentary saves commentary to the session for later review.
	PersistCommentary bool

	// MaxRounds limits the number of autonomous rounds before pausing.
	MaxRounds int

	// PauseOnError pauses auto-reply on tool errors.
	PauseOnError bool

	// PauseOnAmbiguity pauses when the agent is uncertain.
	PauseOnAmbiguity bool

	// NotifyOnCompletion sends a notification when auto-reply completes.
	NotifyOnCompletion bool

	// Callbacks for various auto-reply events.
	OnStart      func(ctx context.Context)
	OnRound      func(ctx context.Context, round int)
	OnPause      func(ctx context.Context, reason string)
	OnComplete   func(ctx context.Context)
	OnCommentary func(ctx context.Context, c *Commentary)
}

// DefaultAutoReplyConfig returns the default auto-reply configuration.
func DefaultAutoReplyConfig() AutoReplyConfig {
	return AutoReplyConfig{
		Enabled:            false,
		CommentaryMode:     CommentaryBrief,
		InterToolDelay:     100 * time.Millisecond,
		PersistCommentary:  true,
		MaxRounds:          50,
		PauseOnError:       true,
		PauseOnAmbiguity:   true,
		NotifyOnCompletion: true,
	}
}

// AutoReplyState tracks the state of an auto-reply session.
type AutoReplyState struct {
	// Active indicates if auto-reply is currently running.
	Active bool

	// StartTime is when the auto-reply session started.
	StartTime time.Time

	// CurrentRound is the current round number.
	CurrentRound int

	// TotalRounds is the total number of rounds executed.
	TotalRounds int

	// ToolsExecuted counts the tools executed.
	ToolsExecuted int

	// LastError holds the last error encountered.
	LastError error

	// PauseReason explains why auto-reply is paused.
	PauseReason string

	// Commentary contains the accumulated commentary.
	Commentary []*Commentary
}

// Duration returns the duration of the auto-reply session.
func (s *AutoReplyState) Duration() time.Duration {
	return time.Since(s.StartTime)
}

// AutoReplyManager manages auto-reply behavior for an agent.
type AutoReplyManager struct {
	config   AutoReplyConfig
	state    AutoReplyState
	emitter  *CommentaryEmitter
	progress *ProgressReporter
}

// NewAutoReplyManager creates a new auto-reply manager.
func NewAutoReplyManager(config AutoReplyConfig, progress *ProgressReporter) *AutoReplyManager {
	emitter := NewCommentaryEmitter(config.CommentaryMode)

	// Wire up commentary dispatch to config callback
	if config.OnCommentary != nil {
		emitter.SetDispatch(func(c *Commentary) {
			config.OnCommentary(context.Background(), c)
		})
	}

	return &AutoReplyManager{
		config:   config,
		emitter:  emitter,
		progress: progress,
	}
}

// Start begins an auto-reply session.
func (m *AutoReplyManager) Start(ctx context.Context) {
	m.state = AutoReplyState{
		Active:    true,
		StartTime: time.Now(),
	}

	if m.config.OnStart != nil {
		m.config.OnStart(ctx)
	}

	m.emitter.EmitProgress(ctx, "Auto-reply session started")
}

// BeginRound marks the start of a new round.
func (m *AutoReplyManager) BeginRound(ctx context.Context) bool {
	if !m.state.Active {
		return false
	}

	m.state.CurrentRound++
	m.state.TotalRounds++

	// Check round limit
	if m.config.MaxRounds > 0 && m.state.TotalRounds > m.config.MaxRounds {
		m.Pause(ctx, "Maximum rounds reached")
		return false
	}

	if m.config.OnRound != nil {
		m.config.OnRound(ctx, m.state.CurrentRound)
	}

	return true
}

// OnToolStart records a tool starting.
func (m *AutoReplyManager) OnToolStart(ctx context.Context, toolName string) {
	m.emitter.EmitProgress(ctx, "Executing "+toolName)
}

// OnToolComplete records a tool completing.
func (m *AutoReplyManager) OnToolComplete(ctx context.Context, toolName string, result any, err error) {
	m.state.ToolsExecuted++

	if err != nil {
		m.state.LastError = err
		m.emitter.EmitToolSummary(ctx, toolName, "error: "+err.Error())

		if m.config.PauseOnError {
			m.Pause(ctx, "Tool error: "+err.Error())
		}
		return
	}

	m.emitter.EmitToolSummary(ctx, toolName, result)

	// Apply inter-tool delay for readability
	if m.config.InterToolDelay > 0 {
		time.Sleep(m.config.InterToolDelay)
	}
}

// OnThinking records agent thinking/reasoning.
func (m *AutoReplyManager) OnThinking(ctx context.Context, thought string) {
	m.emitter.EmitThinking(ctx, thought)
}

// OnDecision records a decision point.
func (m *AutoReplyManager) OnDecision(ctx context.Context, decision string, options []string) {
	m.emitter.EmitDecision(ctx, decision, options)
}

// OnAmbiguity handles uncertainty in the agent.
func (m *AutoReplyManager) OnAmbiguity(ctx context.Context, description string) {
	if m.config.PauseOnAmbiguity {
		m.Pause(ctx, "Ambiguity: "+description)
	}
}

// Pause pauses the auto-reply session.
func (m *AutoReplyManager) Pause(ctx context.Context, reason string) {
	m.state.Active = false
	m.state.PauseReason = reason

	m.emitter.EmitProgress(ctx, "Auto-reply paused: "+reason)

	if m.config.OnPause != nil {
		m.config.OnPause(ctx, reason)
	}
}

// Resume resumes a paused auto-reply session.
func (m *AutoReplyManager) Resume(ctx context.Context) {
	m.state.Active = true
	m.state.PauseReason = ""
	m.state.CurrentRound = 0

	m.emitter.EmitProgress(ctx, "Auto-reply resumed")
}

// Complete ends the auto-reply session.
func (m *AutoReplyManager) Complete(ctx context.Context) {
	m.state.Active = false

	m.emitter.EmitProgress(ctx, "Auto-reply completed")

	// Persist commentary if configured
	if m.config.PersistCommentary {
		m.state.Commentary = m.emitter.Buffer()
	}

	if m.config.OnComplete != nil {
		m.config.OnComplete(ctx)
	}
}

// State returns the current auto-reply state.
func (m *AutoReplyManager) State() AutoReplyState {
	return m.state
}

// IsActive returns whether auto-reply is currently active.
func (m *AutoReplyManager) IsActive() bool {
	return m.state.Active
}

// Emitter returns the commentary emitter.
func (m *AutoReplyManager) Emitter() *CommentaryEmitter {
	return m.emitter
}

// GetCommentary returns all commentary from the session.
func (m *AutoReplyManager) GetCommentary() []*Commentary {
	return m.emitter.Buffer()
}

// Config returns the auto-reply configuration.
func (m *AutoReplyManager) Config() AutoReplyConfig {
	return m.config
}

// UpdateConfig updates the auto-reply configuration.
func (m *AutoReplyManager) UpdateConfig(config AutoReplyConfig) {
	m.config = config
	m.emitter.SetMode(config.CommentaryMode)
}
