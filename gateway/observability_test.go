// Copyright 2025 John Wang. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package gateway

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestNewObservability(t *testing.T) {
	obs, err := NewObservability(ObservabilityConfig{})
	if err != nil {
		t.Fatalf("NewObservability failed: %v", err)
	}

	if obs.config.ServiceName != "omniagent-gateway" {
		t.Errorf("Expected default service name, got %q", obs.config.ServiceName)
	}
}

func TestObservability_StartEndTrace(t *testing.T) {
	obs, err := NewObservability(ObservabilityConfig{})
	if err != nil {
		t.Fatalf("NewObservability failed: %v", err)
	}

	ctx := context.Background()

	// Start trace
	tc := obs.StartTrace(ctx, "test-operation")
	if tc == nil {
		t.Fatal("StartTrace returned nil")
	}
	if tc.startTime.IsZero() {
		t.Error("startTime not set")
	}

	// End trace without error
	obs.EndTrace(tc, nil)

	// Start another trace
	tc2 := obs.StartTrace(ctx, "test-operation-2")

	// End with error
	obs.EndTrace(tc2, errors.New("test error"))
}

func TestObservability_RecordClientConnect(t *testing.T) {
	obs, err := NewObservability(ObservabilityConfig{})
	if err != nil {
		t.Fatalf("NewObservability failed: %v", err)
	}

	ctx := context.Background()

	// Should not panic without provider
	obs.RecordClientConnect(ctx, "client-123")
	obs.RecordClientDisconnect(ctx, "client-123")
}

func TestObservability_RecordMessage(t *testing.T) {
	obs, err := NewObservability(ObservabilityConfig{})
	if err != nil {
		t.Fatalf("NewObservability failed: %v", err)
	}

	ctx := context.Background()

	// Should not panic without provider
	obs.RecordMessage(ctx, "client-123", MessageTypeChat, nil)
	obs.RecordMessage(ctx, "client-123", MessageTypeChat, errors.New("test error"))
}

func TestObservability_RecordToolInvocation(t *testing.T) {
	obs, err := NewObservability(ObservabilityConfig{})
	if err != nil {
		t.Fatalf("NewObservability failed: %v", err)
	}

	ctx := context.Background()

	// Should not panic without provider
	obs.RecordToolInvocation(ctx, "test-tool", 100*time.Millisecond, nil)
	obs.RecordToolInvocation(ctx, "test-tool", 100*time.Millisecond, errors.New("failed"))
}

func TestObservability_WorkflowOperations(t *testing.T) {
	obs, err := NewObservability(ObservabilityConfig{})
	if err != nil {
		t.Fatalf("NewObservability failed: %v", err)
	}

	ctx := context.Background()

	// Should return nil without store
	workflow, err := obs.StartWorkflow(ctx, "test-workflow", nil)
	if err != nil {
		t.Errorf("StartWorkflow should not error without store: %v", err)
	}
	if workflow != nil {
		t.Error("StartWorkflow should return nil without store")
	}

	// Should not error without store
	err = obs.CompleteWorkflow(ctx, "workflow-id", nil)
	if err != nil {
		t.Errorf("CompleteWorkflow should not error without store: %v", err)
	}

	err = obs.FailWorkflow(ctx, "workflow-id", errors.New("failed"))
	if err != nil {
		t.Errorf("FailWorkflow should not error without store: %v", err)
	}
}

func TestObservability_RecordEvent(t *testing.T) {
	obs, err := NewObservability(ObservabilityConfig{})
	if err != nil {
		t.Fatalf("NewObservability failed: %v", err)
	}

	ctx := context.Background()

	// Should not error without store
	err = obs.RecordEvent(ctx, "test_event", "test", map[string]any{"key": "value"})
	if err != nil {
		t.Errorf("RecordEvent should not error without store: %v", err)
	}
}

func TestObservability_ShutdownFlush(t *testing.T) {
	obs, err := NewObservability(ObservabilityConfig{})
	if err != nil {
		t.Fatalf("NewObservability failed: %v", err)
	}

	ctx := context.Background()

	// Should not error without provider
	if err := obs.Shutdown(ctx); err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}

	if err := obs.ForceFlush(ctx); err != nil {
		t.Errorf("ForceFlush failed: %v", err)
	}
}

func TestObservability_Accessors(t *testing.T) {
	obs, err := NewObservability(ObservabilityConfig{})
	if err != nil {
		t.Fatalf("NewObservability failed: %v", err)
	}

	if obs.Provider() != nil {
		t.Error("Provider should be nil when not configured")
	}

	if obs.Store() != nil {
		t.Error("Store should be nil when not configured")
	}
}

func TestStatusFromError(t *testing.T) {
	if statusFromError(nil) != "ok" {
		t.Error("nil error should return 'ok'")
	}

	if statusFromError(errors.New("test")) != "error" {
		t.Error("non-nil error should return 'error'")
	}
}
