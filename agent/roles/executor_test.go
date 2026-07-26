// Copyright 2025 John Wang. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package roles

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/plexusone/omniskill/role"
)

func TestDelegationExecutor_Execute(t *testing.T) {
	factory := NewMapRoleFactory(map[string]func(context.Context) (SubAgent, error){
		"test-role": func(context.Context) (SubAgent, error) {
			return &FuncSubAgent{
				ProcessFunc: func(ctx context.Context, input map[string]any) (map[string]any, error) {
					return map[string]any{"result": "success"}, nil
				},
			}, nil
		},
	})

	executor := NewDelegationExecutor(factory)

	req := &DelegationRequest{
		TaskID:     "task-1",
		TaskType:   "test",
		TargetRole: "test-role",
		Input:      map[string]any{"key": "value"},
		Timeout:    5 * time.Second,
		Status:     role.DelegationStatusPending,
	}

	err := executor.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if req.Status != role.DelegationStatusRunning {
		t.Errorf("Status = %v, want %v", req.Status, role.DelegationStatusRunning)
	}

	// Wait for completion
	result, err := executor.WaitFor(context.Background(), "task-1")
	if err != nil {
		t.Fatalf("WaitFor() error = %v", err)
	}

	if result.Status != role.DelegationStatusCompleted {
		t.Errorf("Result.Status = %v, want %v", result.Status, role.DelegationStatusCompleted)
	}

	if result.Output["result"] != "success" {
		t.Errorf("Result.Output = %v, want success", result.Output)
	}
}

func TestDelegationExecutor_ExecuteSync(t *testing.T) {
	factory := NewMapRoleFactory(map[string]func(context.Context) (SubAgent, error){
		"test-role": func(context.Context) (SubAgent, error) {
			return &FuncSubAgent{
				ProcessFunc: func(ctx context.Context, input map[string]any) (map[string]any, error) {
					return map[string]any{"sync": true}, nil
				},
			}, nil
		},
	})

	executor := NewDelegationExecutor(factory)

	req := &DelegationRequest{
		TaskID:     "sync-task",
		TaskType:   "test",
		TargetRole: "test-role",
		Input:      nil,
		Timeout:    5 * time.Second,
	}

	result, err := executor.ExecuteSync(context.Background(), req)
	if err != nil {
		t.Fatalf("ExecuteSync() error = %v", err)
	}

	if result.Status != role.DelegationStatusCompleted {
		t.Errorf("Result.Status = %v, want %v", result.Status, role.DelegationStatusCompleted)
	}
}

func TestDelegationExecutor_Timeout(t *testing.T) {
	factory := NewMapRoleFactory(map[string]func(context.Context) (SubAgent, error){
		"slow-role": func(context.Context) (SubAgent, error) {
			return &FuncSubAgent{
				ProcessFunc: func(ctx context.Context, input map[string]any) (map[string]any, error) {
					select {
					case <-ctx.Done():
						return nil, ctx.Err()
					case <-time.After(10 * time.Second):
						return map[string]any{}, nil
					}
				},
			}, nil
		},
	})

	executor := NewDelegationExecutor(factory)

	req := &DelegationRequest{
		TaskID:     "timeout-task",
		TaskType:   "test",
		TargetRole: "slow-role",
		Input:      nil,
		Timeout:    100 * time.Millisecond,
	}

	result, err := executor.ExecuteSync(context.Background(), req)
	if err != nil {
		t.Fatalf("ExecuteSync() error = %v", err)
	}

	if result.Status != role.DelegationStatusTimeout {
		t.Errorf("Result.Status = %v, want %v", result.Status, role.DelegationStatusTimeout)
	}
}

func TestDelegationExecutor_Error(t *testing.T) {
	factory := NewMapRoleFactory(map[string]func(context.Context) (SubAgent, error){
		"error-role": func(context.Context) (SubAgent, error) {
			return &FuncSubAgent{
				ProcessFunc: func(ctx context.Context, input map[string]any) (map[string]any, error) {
					return nil, errors.New("task failed")
				},
			}, nil
		},
	})

	executor := NewDelegationExecutor(factory)

	req := &DelegationRequest{
		TaskID:     "error-task",
		TaskType:   "test",
		TargetRole: "error-role",
		Input:      nil,
		Timeout:    5 * time.Second,
	}

	result, err := executor.ExecuteSync(context.Background(), req)
	if err != nil {
		t.Fatalf("ExecuteSync() error = %v", err)
	}

	if result.Status != role.DelegationStatusFailed {
		t.Errorf("Result.Status = %v, want %v", result.Status, role.DelegationStatusFailed)
	}

	if result.Error != "task failed" {
		t.Errorf("Result.Error = %v, want 'task failed'", result.Error)
	}
}

func TestDelegationExecutor_Cancel(t *testing.T) {
	started := make(chan struct{})
	factory := NewMapRoleFactory(map[string]func(context.Context) (SubAgent, error){
		"cancel-role": func(context.Context) (SubAgent, error) {
			return &FuncSubAgent{
				ProcessFunc: func(ctx context.Context, input map[string]any) (map[string]any, error) {
					close(started)
					<-ctx.Done()
					return nil, ctx.Err()
				},
			}, nil
		},
	})

	executor := NewDelegationExecutor(factory)

	req := &DelegationRequest{
		TaskID:     "cancel-task",
		TaskType:   "test",
		TargetRole: "cancel-role",
		Input:      nil,
		Timeout:    10 * time.Second,
	}

	err := executor.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	// Wait for task to start
	<-started

	// Cancel the task
	err = executor.Cancel("cancel-task")
	if err != nil {
		t.Fatalf("Cancel() error = %v", err)
	}

	// Wait for result
	result, err := executor.WaitFor(context.Background(), "cancel-task")
	if err != nil {
		t.Fatalf("WaitFor() error = %v", err)
	}

	if result.Status != role.DelegationStatusCancelled {
		t.Errorf("Result.Status = %v, want %v", result.Status, role.DelegationStatusCancelled)
	}
}

func TestDelegationExecutor_RoleNotFound(t *testing.T) {
	factory := NewMapRoleFactory(map[string]func(context.Context) (SubAgent, error){})

	executor := NewDelegationExecutor(factory)

	req := &DelegationRequest{
		TaskID:     "unknown-task",
		TaskType:   "test",
		TargetRole: "unknown-role",
		Input:      nil,
		Timeout:    5 * time.Second,
	}

	err := executor.Execute(context.Background(), req)
	if err == nil {
		t.Fatal("Execute() expected error for unknown role")
	}
}

func TestDelegationExecutor_ExecuteBatch(t *testing.T) {
	factory := NewMapRoleFactory(map[string]func(context.Context) (SubAgent, error){
		"batch-role": func(context.Context) (SubAgent, error) {
			return &FuncSubAgent{
				ProcessFunc: func(ctx context.Context, input map[string]any) (map[string]any, error) {
					return input, nil
				},
			}, nil
		},
	})

	executor := NewDelegationExecutor(factory)

	requests := []*DelegationRequest{
		{TaskID: "batch-1", TaskType: "test", TargetRole: "batch-role", Input: map[string]any{"id": 1}, Timeout: 5 * time.Second},
		{TaskID: "batch-2", TaskType: "test", TargetRole: "batch-role", Input: map[string]any{"id": 2}, Timeout: 5 * time.Second},
		{TaskID: "batch-3", TaskType: "test", TargetRole: "batch-role", Input: map[string]any{"id": 3}, Timeout: 5 * time.Second},
	}

	results, err := executor.ExecuteBatch(context.Background(), requests)
	if err != nil {
		t.Fatalf("ExecuteBatch() error = %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("ExecuteBatch() returned %d results, want 3", len(results))
	}

	for i, result := range results {
		if result.Status != role.DelegationStatusCompleted {
			t.Errorf("results[%d].Status = %v, want %v", i, result.Status, role.DelegationStatusCompleted)
		}
	}
}

func TestMapRoleFactory_AvailableRoles(t *testing.T) {
	factory := NewMapRoleFactory(map[string]func(context.Context) (SubAgent, error){
		"role-a": nil,
		"role-b": nil,
		"role-c": nil,
	})

	roles := factory.AvailableRoles()
	if len(roles) != 3 {
		t.Errorf("AvailableRoles() returned %d roles, want 3", len(roles))
	}
}
