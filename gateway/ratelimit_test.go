// Copyright 2025 John Wang. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package gateway

import (
	"testing"
	"time"
)

func TestNewRateLimiter(t *testing.T) {
	rl := NewRateLimiter(DefaultRateLimitConfig())
	if rl == nil {
		t.Fatal("NewRateLimiter returned nil")
	}

	if rl.config.Rate != 1.0 {
		t.Errorf("expected rate 1.0, got %f", rl.config.Rate)
	}

	if rl.config.Burst != 5 {
		t.Errorf("expected burst 5, got %d", rl.config.Burst)
	}
}

func TestNewRateLimiter_InvalidConfig(t *testing.T) {
	rl := NewRateLimiter(RateLimitConfig{
		Rate:  -1,
		Burst: -1,
	})

	if rl.config.Rate != 1.0 {
		t.Errorf("expected rate to default to 1.0, got %f", rl.config.Rate)
	}

	if rl.config.Burst != 5 {
		t.Errorf("expected burst to default to 5, got %d", rl.config.Burst)
	}
}

func TestRateLimiter_Allow_BurstCapacity(t *testing.T) {
	rl := NewRateLimiter(RateLimitConfig{
		Rate:            1.0,
		Burst:           3,
		CleanupInterval: time.Hour, // Don't cleanup during test
	})

	senderID := "test-sender"

	// Should allow burst of 3 messages
	for i := 0; i < 3; i++ {
		if !rl.Allow(senderID) {
			t.Errorf("message %d should be allowed within burst", i+1)
		}
	}

	// Fourth message should be rate limited
	if rl.Allow(senderID) {
		t.Error("message 4 should be rate limited")
	}
}

func TestRateLimiter_Allow_TokenRefill(t *testing.T) {
	rl := NewRateLimiter(RateLimitConfig{
		Rate:            10.0, // 10 tokens per second
		Burst:           1,
		CleanupInterval: time.Hour,
	})

	senderID := "test-sender"

	// Use up the initial token
	if !rl.Allow(senderID) {
		t.Error("first message should be allowed")
	}

	// Should be rate limited immediately
	if rl.Allow(senderID) {
		t.Error("second message should be rate limited immediately")
	}

	// Wait for token refill (100ms = 1 token at 10/s rate)
	time.Sleep(150 * time.Millisecond)

	// Should be allowed now
	if !rl.Allow(senderID) {
		t.Error("message after refill should be allowed")
	}
}

func TestRateLimiter_Allow_MultipleSenders(t *testing.T) {
	rl := NewRateLimiter(RateLimitConfig{
		Rate:            1.0,
		Burst:           1,
		CleanupInterval: time.Hour,
	})

	// Each sender should have their own bucket
	if !rl.Allow("sender-1") {
		t.Error("sender-1 first message should be allowed")
	}

	if !rl.Allow("sender-2") {
		t.Error("sender-2 first message should be allowed")
	}

	// Both should now be rate limited
	if rl.Allow("sender-1") {
		t.Error("sender-1 second message should be rate limited")
	}

	if rl.Allow("sender-2") {
		t.Error("sender-2 second message should be rate limited")
	}
}

func TestRateLimiter_Reset(t *testing.T) {
	rl := NewRateLimiter(RateLimitConfig{
		Rate:            1.0,
		Burst:           1,
		CleanupInterval: time.Hour,
	})

	senderID := "test-sender"

	// Use up the token
	rl.Allow(senderID)

	// Should be rate limited
	if rl.Allow(senderID) {
		t.Error("should be rate limited before reset")
	}

	// Reset the sender
	rl.Reset(senderID)

	// Should be allowed again with fresh bucket
	if !rl.Allow(senderID) {
		t.Error("should be allowed after reset")
	}
}

func TestRateLimiter_Cleanup(t *testing.T) {
	rl := NewRateLimiter(RateLimitConfig{
		Rate:            1.0,
		Burst:           1,
		CleanupInterval: 10 * time.Millisecond,
	})

	senderID := "test-sender"

	// Create a bucket
	rl.Allow(senderID)

	// Manually trigger cleanup with a very short stale threshold
	rl.mu.Lock()
	if len(rl.buckets) != 1 {
		t.Errorf("expected 1 bucket, got %d", len(rl.buckets))
	}
	rl.mu.Unlock()

	// The cleanup won't remove it immediately since it was just accessed
	rl.cleanup()

	rl.mu.Lock()
	if len(rl.buckets) != 1 {
		t.Error("bucket should not be cleaned up immediately")
	}
	rl.mu.Unlock()
}

func TestDefaultRateLimitConfig(t *testing.T) {
	cfg := DefaultRateLimitConfig()

	if cfg.Rate != 1.0 {
		t.Errorf("expected rate 1.0, got %f", cfg.Rate)
	}

	if cfg.Burst != 5 {
		t.Errorf("expected burst 5, got %d", cfg.Burst)
	}

	if cfg.CleanupInterval != time.Minute {
		t.Errorf("expected cleanup interval 1 minute, got %v", cfg.CleanupInterval)
	}
}
