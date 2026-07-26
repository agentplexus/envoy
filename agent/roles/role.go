// Package roles provides role support for omniagent.
//
// Roles are high-level agent personas that combine skills, workflows,
// and system prompts into cohesive behaviors. They provide a way to
// configure agents for specific use cases like "Meeting PM" or
// "Investment Analyst".
//
// # Role Architecture
//
// A role defines:
//   - Required and optional skills it needs
//   - System prompt modifications for the persona
//   - Workflows for multi-step operations
//
// # Usage
//
//	// Create a role with its configuration
//	pmRole := meetingpm.New(meetingpm.Config{
//	    DefaultConfluenceSpace: "TEAM",
//	})
//
//	// Create the skills the role needs
//	meetingSkill := meeting.NewSkill(...)
//	googleSkill := google.NewSkill(...)
//
//	// Register role with agent
//	agent, err := agent.New(config,
//	    agent.WithRole(pmRole, meetingSkill, googleSkill),
//	)
package roles

import (
	"context"
	"errors"
	"fmt"

	"github.com/plexusone/omniagent/skills/compiled"
	"github.com/plexusone/omniskill/role"
	"github.com/plexusone/omniskill/skill"
)

// Manager manages role lifecycle and skill injection.
//
// The enhanced Manager integrates policy enforcement, behavior matching,
// metrics collection, and delegation capabilities from the role's spec.
type Manager struct {
	role   role.Role
	spec   *role.RoleSpec
	skills map[string]skill.Skill

	// Engines for enhanced role capabilities
	policyEngine     *PolicyEngine
	behaviorEngine   *BehaviorEngine
	metricsStore     MetricsStore
	metricsCollector *MetricsCollector
	delegator        *Delegator
	executor         *DelegationExecutor
}

// NewManager creates a new role manager with the given role and skills.
func NewManager(r role.Role, compiledSkills ...compiled.Skill) (*Manager, error) {
	m := &Manager{
		role:   r,
		skills: make(map[string]skill.Skill),
	}

	// Convert compiled skills to skill.Skill interface
	for _, cs := range compiledSkills {
		// Create a skill adapter
		adapter := &skillAdapter{compiled: cs}
		m.skills[cs.Name()] = adapter
	}

	// Validate required skills are present
	for _, required := range r.RequiredSkills() {
		if _, ok := m.skills[required]; !ok {
			return nil, fmt.Errorf("required skill not provided: %s", required)
		}
	}

	// Cache the role spec
	m.spec = r.Spec()

	// Initialize engines from the spec
	m.initEngines()

	return m, nil
}

// NewManagerWithMetrics creates a Manager with a custom metrics store.
func NewManagerWithMetrics(r role.Role, metricsStore MetricsStore, compiledSkills ...compiled.Skill) (*Manager, error) {
	m, err := NewManager(r, compiledSkills...)
	if err != nil {
		return nil, err
	}
	m.metricsStore = metricsStore
	if m.spec != nil && len(m.spec.Metrics) > 0 {
		m.metricsCollector = NewMetricsCollector(metricsStore, m.spec.ID, m.spec.Metrics)
	}
	return m, nil
}

// initEngines initializes the engines from the role spec.
func (m *Manager) initEngines() {
	if m.spec == nil {
		return
	}

	// Initialize policy engine
	if len(m.spec.Policies) > 0 {
		m.policyEngine = NewPolicyEngine(m.spec.Policies)
	}

	// Initialize behavior engine
	if len(m.spec.Behaviors) > 0 {
		m.behaviorEngine = NewBehaviorEngine(m.spec.Behaviors)
	}

	// Initialize delegator
	if m.spec.Delegation != nil {
		m.delegator = NewDelegator(m.spec.Delegation)
	}

	// Initialize metrics (with default in-memory store)
	if len(m.spec.Metrics) > 0 {
		if m.metricsStore == nil {
			m.metricsStore = NewInMemoryMetricsStore()
		}
		m.metricsCollector = NewMetricsCollector(m.metricsStore, m.spec.ID, m.spec.Metrics)
	}
}

// Init initializes the role with its skills.
func (m *Manager) Init(ctx context.Context) error {
	return m.role.Init(ctx, m.skills)
}

// Close closes the role.
func (m *Manager) Close() error {
	return m.role.Close()
}

// Role returns the managed role.
func (m *Manager) Role() role.Role {
	return m.role
}

// SystemPrompt returns the role's system prompt.
func (m *Manager) SystemPrompt(ctx context.Context) (string, error) {
	return m.role.SystemPrompt(ctx)
}

// Workflows returns the role's workflows.
func (m *Manager) Workflows() []role.Workflow {
	return m.role.Workflows()
}

// Spec returns the role's specification.
func (m *Manager) Spec() *role.RoleSpec {
	return m.spec
}

// CheckToolAccess validates that a tool can be used according to policies.
// Returns nil if allowed, an error if denied.
func (m *Manager) CheckToolAccess(ctx context.Context, toolName string) error {
	if m.policyEngine == nil {
		return nil
	}
	return m.policyEngine.CheckToolAccess(ctx, toolName)
}

// CheckDataAccess validates that a data type can be accessed according to policies.
// Returns nil if allowed, an error if denied.
func (m *Manager) CheckDataAccess(ctx context.Context, dataType string) error {
	if m.policyEngine == nil {
		return nil
	}
	return m.policyEngine.CheckDataAccess(ctx, dataType)
}

// RequiresConfirmation checks if an operation requires user confirmation.
func (m *Manager) RequiresConfirmation(ctx context.Context, operation string) bool {
	if m.policyEngine == nil {
		return false
	}
	return m.policyEngine.RequiresConfirmation(ctx, operation)
}

// SetBehaviorContext updates the current behavior context.
func (m *Manager) SetBehaviorContext(ctx role.BehaviorContext) {
	if m.behaviorEngine != nil {
		m.behaviorEngine.SetContext(ctx)
	}
}

// GetActiveBehaviors returns behaviors active in the current context.
func (m *Manager) GetActiveBehaviors(ctx context.Context) []role.Behavior {
	if m.behaviorEngine == nil {
		return nil
	}
	return m.behaviorEngine.GetActive(ctx)
}

// MatchBehaviorTrigger returns behaviors matching a specific event.
func (m *Manager) MatchBehaviorTrigger(ctx context.Context, event string) []role.Behavior {
	if m.behaviorEngine == nil {
		return nil
	}
	return m.behaviorEngine.MatchTrigger(ctx, event)
}

// RecordMetric records a metric value.
func (m *Manager) RecordMetric(ctx context.Context, metricID string, value float64) error {
	if m.metricsCollector == nil {
		return nil
	}
	return m.metricsCollector.Record(ctx, metricID, value)
}

// IncrementMetric increments a counter metric by 1.
func (m *Manager) IncrementMetric(ctx context.Context, metricID string) error {
	if m.metricsCollector == nil {
		return nil
	}
	return m.metricsCollector.Increment(ctx, metricID)
}

// CanDelegate checks if a task type can be delegated.
func (m *Manager) CanDelegate(ctx context.Context, taskType string) (bool, []string) {
	if m.delegator == nil {
		return false, nil
	}
	return m.delegator.CanDelegate(ctx, taskType)
}

// Delegate creates a delegation request for a task.
func (m *Manager) Delegate(ctx context.Context, taskType, taskID string, input map[string]any) (*DelegationRequest, error) {
	if m.delegator == nil {
		return nil, ErrDelegationDisabled
	}
	return m.delegator.Delegate(ctx, taskType, taskID, input)
}

// IsAutonomousDelegation checks if a task type can be delegated autonomously.
func (m *Manager) IsAutonomousDelegation(taskType string) bool {
	if m.delegator == nil {
		return false
	}
	return m.delegator.IsAutonomous(taskType)
}

// SetExecutor sets the delegation executor for running sub-agents.
func (m *Manager) SetExecutor(executor *DelegationExecutor) {
	m.executor = executor
}

// Executor returns the delegation executor, or nil if not configured.
func (m *Manager) Executor() *DelegationExecutor {
	return m.executor
}

// ExecuteDelegation executes a delegation request using the configured executor.
// Returns an error if no executor is configured.
func (m *Manager) ExecuteDelegation(ctx context.Context, req *DelegationRequest) error {
	if m.executor == nil {
		return errors.New("delegation executor not configured")
	}
	return m.executor.Execute(ctx, req)
}

// ExecuteDelegationSync executes a delegation request synchronously.
// Blocks until the task completes or times out.
func (m *Manager) ExecuteDelegationSync(ctx context.Context, req *DelegationRequest) (*role.DelegationResult, error) {
	if m.executor == nil {
		return nil, errors.New("delegation executor not configured")
	}
	return m.executor.ExecuteSync(ctx, req)
}

// DelegateAndExecute is a convenience method that creates and executes a delegation.
// Combines Delegate() and ExecuteDelegation() into a single call.
func (m *Manager) DelegateAndExecute(ctx context.Context, taskType, taskID string, input map[string]any) (*DelegationRequest, error) {
	req, err := m.Delegate(ctx, taskType, taskID, input)
	if err != nil {
		return nil, err
	}

	if err := m.ExecuteDelegation(ctx, req); err != nil {
		return nil, err
	}

	return req, nil
}

// DelegateAndWait is a convenience method that delegates, executes, and waits for completion.
func (m *Manager) DelegateAndWait(ctx context.Context, taskType, taskID string, input map[string]any) (*role.DelegationResult, error) {
	req, err := m.Delegate(ctx, taskType, taskID, input)
	if err != nil {
		return nil, err
	}

	return m.ExecuteDelegationSync(ctx, req)
}

// OptionalSkills returns the optional skills for the role, if defined.
func (m *Manager) OptionalSkills() []string {
	if sr, ok := m.role.(role.SkillRequirer); ok {
		return sr.OptionalSkills()
	}
	if m.spec != nil {
		var names []string
		for _, s := range m.spec.Skills.Optional {
			names = append(names, s.Name)
		}
		return names
	}
	return nil
}

// skillAdapter adapts a compiled.Skill to skill.Skill interface.
type skillAdapter struct {
	compiled compiled.Skill
}

func (a *skillAdapter) Name() string {
	return a.compiled.Name()
}

func (a *skillAdapter) Description() string {
	return a.compiled.Description()
}

func (a *skillAdapter) Tools() []skill.Tool {
	return a.compiled.Tools()
}

func (a *skillAdapter) Init(ctx context.Context) error {
	return a.compiled.Init(ctx)
}

func (a *skillAdapter) Close() error {
	return a.compiled.Close()
}

// Ensure skillAdapter implements skill.Skill.
var _ skill.Skill = (*skillAdapter)(nil)
