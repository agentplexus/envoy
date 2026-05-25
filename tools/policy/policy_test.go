// Copyright 2025 John Wang. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package policy

import (
	"context"
	"testing"
	"time"
)

func TestManager_CheckAccess_NoPolicy(t *testing.T) {
	m := NewManager()

	err := m.CheckAccess(context.Background(), "user1", "shell")
	if err != nil {
		t.Errorf("CheckAccess with no policy should allow: %v", err)
	}
}

func TestManager_CheckAccess_DeniedTools(t *testing.T) {
	m := NewManager()
	m.SetPolicy("user1", &Policy{
		DeniedTools: []string{"shell", "browser"},
	})

	// Denied tool
	err := m.CheckAccess(context.Background(), "user1", "shell")
	if err == nil {
		t.Error("Expected access denied for shell")
	}
	if _, ok := err.(*AccessDeniedError); !ok {
		t.Errorf("Expected AccessDeniedError, got %T", err)
	}

	// Allowed tool
	err = m.CheckAccess(context.Background(), "user1", "search")
	if err != nil {
		t.Errorf("Expected search to be allowed: %v", err)
	}
}

func TestManager_CheckAccess_AllowedTools(t *testing.T) {
	m := NewManager()
	m.SetPolicy("user1", &Policy{
		AllowedTools: []string{"search", "read"},
	})

	// Allowed tool
	err := m.CheckAccess(context.Background(), "user1", "search")
	if err != nil {
		t.Errorf("Expected search to be allowed: %v", err)
	}

	// Not allowed tool
	err = m.CheckAccess(context.Background(), "user1", "shell")
	if err == nil {
		t.Error("Expected access denied for shell")
	}
}

func TestManager_CheckAccess_DeniedOverridesAllowed(t *testing.T) {
	m := NewManager()
	m.SetPolicy("user1", &Policy{
		AllowedTools: []string{"*"},
		DeniedTools:  []string{"shell"},
	})

	// Shell is denied even though * is allowed
	err := m.CheckAccess(context.Background(), "user1", "shell")
	if err == nil {
		t.Error("Expected access denied for shell")
	}

	// Other tools should be allowed
	err = m.CheckAccess(context.Background(), "user1", "search")
	if err != nil {
		t.Errorf("Expected search to be allowed: %v", err)
	}
}

func TestManager_DefaultPolicy(t *testing.T) {
	m := NewManager()
	m.SetDefaultPolicy(&Policy{
		AllowedTools: []string{"search"},
	})

	// User without specific policy uses default
	err := m.CheckAccess(context.Background(), "user1", "search")
	if err != nil {
		t.Errorf("Expected search to be allowed: %v", err)
	}

	err = m.CheckAccess(context.Background(), "user1", "shell")
	if err == nil {
		t.Error("Expected access denied for shell")
	}

	// User with specific policy overrides default
	m.SetPolicy("admin", &Policy{
		AllowedTools: []string{"*"},
	})

	err = m.CheckAccess(context.Background(), "admin", "shell")
	if err != nil {
		t.Errorf("Expected shell to be allowed for admin: %v", err)
	}
}

func TestManager_RateLimits(t *testing.T) {
	m := NewManager()
	m.SetPolicy("user1", &Policy{
		RateLimits: map[string]int{
			"search": 2, // 2 calls per minute
		},
	})

	// First two calls should succeed
	m.RecordUsage("user1", "search")
	m.RecordUsage("user1", "search")

	// Third call should fail
	err := m.CheckAccess(context.Background(), "user1", "search")
	if err == nil {
		t.Error("Expected rate limit error")
	}
	if _, ok := err.(*RateLimitError); !ok {
		t.Errorf("Expected RateLimitError, got %T", err)
	}

	// Other tools should still work
	err = m.CheckAccess(context.Background(), "user1", "read")
	if err != nil {
		t.Errorf("Expected read to be allowed: %v", err)
	}
}

func TestManager_MaxConcurrent(t *testing.T) {
	m := NewManager()
	m.SetPolicy("user1", &Policy{
		MaxConcurrent: 2,
	})

	// Start two executions
	m.StartExecution("user1")
	m.StartExecution("user1")

	// Third should fail
	err := m.CheckAccess(context.Background(), "user1", "search")
	if err == nil {
		t.Error("Expected concurrency limit error")
	}
	if _, ok := err.(*ConcurrencyError); !ok {
		t.Errorf("Expected ConcurrencyError, got %T", err)
	}

	// End one execution
	m.EndExecution("user1")

	// Now should succeed
	err = m.CheckAccess(context.Background(), "user1", "search")
	if err != nil {
		t.Errorf("Expected access after ending execution: %v", err)
	}
}

func TestManager_RemovePolicy(t *testing.T) {
	m := NewManager()
	m.SetPolicy("user1", &Policy{
		DeniedTools: []string{"shell"},
	})

	// Should be denied
	err := m.CheckAccess(context.Background(), "user1", "shell")
	if err == nil {
		t.Error("Expected access denied")
	}

	// Remove policy
	m.RemovePolicy("user1")

	// Should be allowed now
	err = m.CheckAccess(context.Background(), "user1", "shell")
	if err != nil {
		t.Errorf("Expected access after removing policy: %v", err)
	}
}

func TestManager_CleanupUsage(t *testing.T) {
	m := NewManager()

	// Record some usage
	m.RecordUsage("user1", "search")
	m.RecordUsage("user2", "read")

	// Should have usage data
	m.mu.RLock()
	count := len(m.usage)
	m.mu.RUnlock()
	if count != 2 {
		t.Errorf("Expected 2 usage entries, got %d", count)
	}

	// Cleanup shouldn't remove recent data
	m.CleanupUsage()
	m.mu.RLock()
	count = len(m.usage)
	m.mu.RUnlock()
	if count != 2 {
		t.Errorf("Expected 2 usage entries after cleanup, got %d", count)
	}
}

func TestWithSenderID(t *testing.T) {
	ctx := context.Background()

	// No sender ID
	if GetSenderID(ctx) != "" {
		t.Error("Expected empty sender ID")
	}

	// With sender ID
	ctx = WithSenderID(ctx, "user1")
	if GetSenderID(ctx) != "user1" {
		t.Errorf("Expected user1, got %s", GetSenderID(ctx))
	}
}

func TestAccessDeniedError(t *testing.T) {
	err := &AccessDeniedError{
		SenderID: "user1",
		ToolName: "shell",
		Reason:   "tool is denied",
	}

	msg := err.Error()
	if msg == "" {
		t.Error("Expected non-empty error message")
	}
}

func TestRateLimitError(t *testing.T) {
	err := &RateLimitError{
		SenderID: "user1",
		ToolName: "search",
		Limit:    10,
		Window:   time.Minute,
	}

	msg := err.Error()
	if msg == "" {
		t.Error("Expected non-empty error message")
	}
}

func TestConcurrencyError(t *testing.T) {
	err := &ConcurrencyError{
		SenderID: "user1",
		Limit:    5,
		Active:   5,
	}

	msg := err.Error()
	if msg == "" {
		t.Error("Expected non-empty error message")
	}
}
