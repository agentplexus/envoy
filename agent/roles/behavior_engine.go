// Copyright 2025 John Wang. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package roles

import (
	"context"
	"sort"

	"github.com/plexusone/omniskill/role"
)

// BehaviorEngine matches behaviors to the current context.
//
// The engine evaluates behaviors defined in a role's RoleSpec and
// determines which behaviors are active based on the current context
// (meeting, chat, autonomous) and any triggered events.
type BehaviorEngine struct {
	behaviors []role.Behavior
	context   role.BehaviorContext
}

// NewBehaviorEngine creates a new BehaviorEngine with the given behaviors.
func NewBehaviorEngine(behaviors []role.Behavior) *BehaviorEngine {
	return &BehaviorEngine{
		behaviors: behaviors,
		context:   role.BehaviorContextAlways,
	}
}

// SetContext updates the current behavior context.
func (e *BehaviorEngine) SetContext(ctx role.BehaviorContext) {
	e.context = ctx
}

// Context returns the current behavior context.
func (e *BehaviorEngine) Context() role.BehaviorContext {
	return e.context
}

// GetActive returns all behaviors that are active in the current context.
// Behaviors are sorted by priority (highest first).
func (e *BehaviorEngine) GetActive(ctx context.Context) []role.Behavior {
	var active []role.Behavior

	for _, b := range e.behaviors {
		if !b.Enabled {
			continue
		}
		if e.matchesContext(b.Context) {
			active = append(active, b)
		}
	}

	// Sort by priority (highest first)
	sort.Slice(active, func(i, j int) bool {
		return active[i].Priority > active[j].Priority
	})

	return active
}

// MatchTrigger returns behaviors that match a specific event trigger.
// Only returns behaviors that are both active in the current context
// and have a matching event trigger.
func (e *BehaviorEngine) MatchTrigger(ctx context.Context, event string) []role.Behavior {
	var matched []role.Behavior

	for _, b := range e.behaviors {
		if !b.Enabled {
			continue
		}
		if !e.matchesContext(b.Context) {
			continue
		}
		if e.matchesEvent(b.Trigger, event) {
			matched = append(matched, b)
		}
	}

	// Sort by priority (highest first)
	sort.Slice(matched, func(i, j int) bool {
		return matched[i].Priority > matched[j].Priority
	})

	return matched
}

// HasBehavior checks if a behavior with the given ID exists and is enabled.
func (e *BehaviorEngine) HasBehavior(id string) bool {
	for _, b := range e.behaviors {
		if b.ID == id && b.Enabled {
			return true
		}
	}
	return false
}

// GetBehavior returns a specific behavior by ID.
// Returns nil if the behavior is not found.
func (e *BehaviorEngine) GetBehavior(id string) *role.Behavior {
	for i := range e.behaviors {
		if e.behaviors[i].ID == id {
			return &e.behaviors[i]
		}
	}
	return nil
}

// matchesContext checks if a behavior context matches the current context.
func (e *BehaviorEngine) matchesContext(behaviorCtx role.BehaviorContext) bool {
	// "always" behaviors match any context
	if behaviorCtx == role.BehaviorContextAlways {
		return true
	}
	return behaviorCtx == e.context
}

// matchesEvent checks if a trigger matches a specific event.
func (e *BehaviorEngine) matchesEvent(trigger role.BehaviorTrigger, event string) bool {
	if trigger.Type != role.TriggerTypeEvent {
		return false
	}
	return trigger.Event == event
}

// BehaviorExecution represents the execution of a behavior.
type BehaviorExecution struct {
	BehaviorID string
	Actions    []ActionExecution
	Completed  bool
	Error      error
}

// ActionExecution represents the execution of a single action.
type ActionExecution struct {
	ActionID  string
	Type      string
	Completed bool
	Result    map[string]any
	Error     error
}

// ExecuteBehavior runs the actions defined in a behavior.
// This is a placeholder that returns the actions to be executed.
// The actual execution happens in the agent runtime.
func (e *BehaviorEngine) ExecuteBehavior(ctx context.Context, behavior role.Behavior) *BehaviorExecution {
	exec := &BehaviorExecution{
		BehaviorID: behavior.ID,
		Actions:    make([]ActionExecution, len(behavior.Actions)),
	}

	for i, action := range behavior.Actions {
		exec.Actions[i] = ActionExecution{
			ActionID:  action.ID,
			Type:      action.Type,
			Completed: false,
		}
	}

	return exec
}
