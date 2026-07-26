// Copyright 2025 John Wang. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package roles

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/plexusone/omniskill/role"
)

// DelegationExecutor executes delegated tasks by spawning sub-agents.
//
// The executor manages the lifecycle of delegated tasks, including:
//   - Task queuing and scheduling
//   - Sub-agent spawning
//   - Result collection and timeout handling
//   - Retry logic for failed delegations
type DelegationExecutor struct {
	// RoleFactory creates agents for target roles.
	RoleFactory RoleFactory

	// Logger for delegation events.
	Logger *slog.Logger

	// RetryPolicy for failed delegations.
	RetryPolicy *role.DelegationRetryPolicy

	mu       sync.Mutex
	active   map[string]*executingTask
	results  map[string]*role.DelegationResult
	shutdown bool
}

// RoleFactory creates sub-agents for target roles.
//
// Implementations should return an agent capable of processing
// tasks for the specified role. The agent should be fully configured
// with the role's required skills.
type RoleFactory interface {
	// CreateAgent creates a sub-agent for the given role name.
	// Returns an error if the role is not available.
	CreateAgent(ctx context.Context, roleName string) (SubAgent, error)

	// AvailableRoles returns the list of roles that can be created.
	AvailableRoles() []string
}

// SubAgent is the interface for sub-agents that execute delegated tasks.
type SubAgent interface {
	// Process executes a task and returns the result.
	Process(ctx context.Context, taskInput map[string]any) (map[string]any, error)

	// Close releases resources held by the sub-agent.
	Close() error
}

// executingTask tracks an in-progress delegation.
type executingTask struct {
	request   *DelegationRequest
	agent     SubAgent
	cancel    context.CancelFunc
	startTime time.Time
	retries   int
}

// NewDelegationExecutor creates a new executor.
func NewDelegationExecutor(factory RoleFactory) *DelegationExecutor {
	return &DelegationExecutor{
		RoleFactory: factory,
		Logger:      slog.Default(),
		active:      make(map[string]*executingTask),
		results:     make(map[string]*role.DelegationResult),
	}
}

// Execute runs a delegation request asynchronously.
// Returns immediately; use WaitFor to get the result.
func (e *DelegationExecutor) Execute(ctx context.Context, req *DelegationRequest) error {
	if e.shutdown {
		return errors.New("executor is shut down")
	}

	e.mu.Lock()
	if _, exists := e.active[req.TaskID]; exists {
		e.mu.Unlock()
		return fmt.Errorf("task %s is already executing", req.TaskID)
	}
	e.mu.Unlock()

	// Create sub-agent for target role
	agent, err := e.RoleFactory.CreateAgent(ctx, req.TargetRole)
	if err != nil {
		return fmt.Errorf("create agent for role %s: %w", req.TargetRole, err)
	}

	// Create cancellable context with timeout
	execCtx, cancel := context.WithTimeout(ctx, req.Timeout)

	task := &executingTask{
		request:   req,
		agent:     agent,
		cancel:    cancel,
		startTime: time.Now(),
	}

	e.mu.Lock()
	e.active[req.TaskID] = task
	req.Status = role.DelegationStatusRunning
	e.mu.Unlock()

	// Execute in background
	go e.executeTask(execCtx, task)

	e.Logger.Info("delegation started",
		"task_id", req.TaskID,
		"task_type", req.TaskType,
		"target_role", req.TargetRole,
		"timeout", req.Timeout)

	return nil
}

// executeTask runs the task and handles completion/errors.
func (e *DelegationExecutor) executeTask(ctx context.Context, task *executingTask) {
	defer task.cancel()
	defer func() {
		if err := task.agent.Close(); err != nil {
			e.Logger.Warn("failed to close sub-agent", "error", err)
		}
	}()

	req := task.request
	result := &role.DelegationResult{
		RuleID:     "", // Could be set from the rule that triggered this
		TargetRole: req.TargetRole,
		TaskID:     req.TaskID,
	}

	// Execute the task
	output, err := task.agent.Process(ctx, req.Input)

	e.mu.Lock()
	defer e.mu.Unlock()

	// Remove from active
	delete(e.active, req.TaskID)

	// Calculate duration
	result.Duration = time.Since(task.startTime).String()

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			result.Status = role.DelegationStatusTimeout
			result.Error = "task timed out"
		} else if ctx.Err() == context.Canceled {
			result.Status = role.DelegationStatusCancelled
			result.Error = "task cancelled"
		} else {
			result.Status = role.DelegationStatusFailed
			result.Error = err.Error()
		}

		// Check retry policy
		if e.shouldRetry(task, err) {
			e.Logger.Info("retrying delegation",
				"task_id", req.TaskID,
				"retry", task.retries+1)
			task.retries++
			// Re-queue would go here (simplified: just log for now)
		}
	} else {
		result.Status = role.DelegationStatusCompleted
		result.Output = output
	}

	req.Status = result.Status
	req.Result = result
	e.results[req.TaskID] = result

	e.Logger.Info("delegation completed",
		"task_id", req.TaskID,
		"status", result.Status,
		"duration", result.Duration)
}

// shouldRetry determines if a failed task should be retried.
func (e *DelegationExecutor) shouldRetry(task *executingTask, err error) bool {
	if e.RetryPolicy == nil {
		return false
	}
	if task.retries >= e.RetryPolicy.MaxRetries {
		return false
	}
	// Don't retry cancellation or timeout
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	return true
}

// WaitFor waits for a task to complete and returns its result.
func (e *DelegationExecutor) WaitFor(ctx context.Context, taskID string) (*role.DelegationResult, error) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			e.mu.Lock()
			if result, ok := e.results[taskID]; ok {
				e.mu.Unlock()
				return result, nil
			}
			if _, active := e.active[taskID]; !active {
				e.mu.Unlock()
				return nil, fmt.Errorf("task %s not found", taskID)
			}
			e.mu.Unlock()
		}
	}
}

// Cancel cancels an executing task.
func (e *DelegationExecutor) Cancel(taskID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	task, ok := e.active[taskID]
	if !ok {
		return fmt.Errorf("task %s not found or already completed", taskID)
	}

	task.cancel()
	return nil
}

// GetResult returns the result for a completed task.
func (e *DelegationExecutor) GetResult(taskID string) (*role.DelegationResult, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	result, ok := e.results[taskID]
	return result, ok
}

// ActiveTasks returns the number of currently executing tasks.
func (e *DelegationExecutor) ActiveTasks() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.active)
}

// Shutdown gracefully shuts down the executor.
// Waits for active tasks to complete or cancels them after timeout.
func (e *DelegationExecutor) Shutdown(ctx context.Context) error {
	e.mu.Lock()
	e.shutdown = true
	tasks := make([]*executingTask, 0, len(e.active))
	for _, task := range e.active {
		tasks = append(tasks, task)
	}
	e.mu.Unlock()

	// Cancel all active tasks
	for _, task := range tasks {
		task.cancel()
	}

	// Wait for completion
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if e.ActiveTasks() == 0 {
				return nil
			}
		}
	}
}

// ExecuteSync executes a delegation request synchronously.
// Blocks until the task completes or times out.
func (e *DelegationExecutor) ExecuteSync(ctx context.Context, req *DelegationRequest) (*role.DelegationResult, error) {
	if err := e.Execute(ctx, req); err != nil {
		return nil, err
	}

	return e.WaitFor(ctx, req.TaskID)
}

// ExecuteBatch executes multiple delegation requests in parallel.
// Returns results in the same order as requests.
func (e *DelegationExecutor) ExecuteBatch(ctx context.Context, requests []*DelegationRequest) ([]*role.DelegationResult, error) {
	// Start all tasks
	for _, req := range requests {
		if err := e.Execute(ctx, req); err != nil {
			return nil, fmt.Errorf("start task %s: %w", req.TaskID, err)
		}
	}

	// Wait for all results
	results := make([]*role.DelegationResult, len(requests))
	for i, req := range requests {
		result, err := e.WaitFor(ctx, req.TaskID)
		if err != nil {
			return results, fmt.Errorf("wait for task %s: %w", req.TaskID, err)
		}
		results[i] = result
	}

	return results, nil
}

// MapRoleFactory is a simple RoleFactory backed by a map of role creators.
type MapRoleFactory struct {
	creators map[string]func(ctx context.Context) (SubAgent, error)
}

// NewMapRoleFactory creates a factory from a map of role creators.
func NewMapRoleFactory(creators map[string]func(ctx context.Context) (SubAgent, error)) *MapRoleFactory {
	return &MapRoleFactory{creators: creators}
}

// CreateAgent implements RoleFactory.
func (f *MapRoleFactory) CreateAgent(ctx context.Context, roleName string) (SubAgent, error) {
	creator, ok := f.creators[roleName]
	if !ok {
		return nil, fmt.Errorf("role not available: %s", roleName)
	}
	return creator(ctx)
}

// AvailableRoles implements RoleFactory.
func (f *MapRoleFactory) AvailableRoles() []string {
	roles := make([]string, 0, len(f.creators))
	for name := range f.creators {
		roles = append(roles, name)
	}
	return roles
}

// FuncSubAgent wraps a function as a SubAgent.
type FuncSubAgent struct {
	ProcessFunc func(ctx context.Context, input map[string]any) (map[string]any, error)
}

// Process implements SubAgent.
func (f *FuncSubAgent) Process(ctx context.Context, input map[string]any) (map[string]any, error) {
	return f.ProcessFunc(ctx, input)
}

// Close implements SubAgent.
func (f *FuncSubAgent) Close() error {
	return nil
}

// JSONInput is a helper to marshal input to JSON for sub-agent processing.
func JSONInput(input map[string]any) (string, error) {
	data, err := json.Marshal(input)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
