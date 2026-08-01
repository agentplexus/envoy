// Copyright 2025 John Wang. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package gateway

import (
	"sync"
	"time"
)

// RateLimiter implements per-sender rate limiting using a token bucket algorithm.
type RateLimiter struct {
	mu      sync.RWMutex
	buckets map[string]*tokenBucket
	config  RateLimitConfig
}

// RateLimitConfig configures the rate limiter.
type RateLimitConfig struct {
	// Rate is the number of messages allowed per second per sender.
	Rate float64
	// Burst is the maximum number of messages that can be sent in a burst.
	Burst int
	// CleanupInterval is how often to clean up stale buckets.
	CleanupInterval time.Duration
}

// DefaultRateLimitConfig returns sensible defaults for rate limiting.
func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		Rate:            1.0, // 1 message per second
		Burst:           5,   // Allow bursts of up to 5 messages
		CleanupInterval: time.Minute,
	}
}

// tokenBucket implements a token bucket for rate limiting.
type tokenBucket struct {
	tokens     float64
	lastUpdate time.Time
	rate       float64
	burst      int
}

// NewRateLimiter creates a new rate limiter with the given configuration.
func NewRateLimiter(config RateLimitConfig) *RateLimiter {
	if config.Rate <= 0 {
		config.Rate = 1.0
	}
	if config.Burst <= 0 {
		config.Burst = 5
	}
	if config.CleanupInterval <= 0 {
		config.CleanupInterval = time.Minute
	}

	rl := &RateLimiter{
		buckets: make(map[string]*tokenBucket),
		config:  config,
	}

	// Start cleanup goroutine
	go rl.cleanupLoop()

	return rl
}

// Allow checks if a message from the given sender should be allowed.
// Returns true if the message is allowed, false if rate limited.
func (rl *RateLimiter) Allow(senderID string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	bucket, ok := rl.buckets[senderID]
	if !ok {
		bucket = &tokenBucket{
			tokens:     float64(rl.config.Burst),
			lastUpdate: time.Now(),
			rate:       rl.config.Rate,
			burst:      rl.config.Burst,
		}
		rl.buckets[senderID] = bucket
	}

	// Refill tokens based on elapsed time
	now := time.Now()
	elapsed := now.Sub(bucket.lastUpdate).Seconds()
	bucket.tokens += elapsed * bucket.rate
	if bucket.tokens > float64(bucket.burst) {
		bucket.tokens = float64(bucket.burst)
	}
	bucket.lastUpdate = now

	// Check if we can consume a token
	if bucket.tokens >= 1.0 {
		bucket.tokens--
		return true
	}

	return false
}

// Reset removes all rate limit state for a sender.
func (rl *RateLimiter) Reset(senderID string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.buckets, senderID)
}

// cleanupLoop periodically removes stale buckets to prevent memory leaks.
func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(rl.config.CleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		rl.cleanup()
	}
}

// cleanup removes buckets that have been idle for too long.
func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	staleThreshold := time.Now().Add(-5 * rl.config.CleanupInterval)
	for senderID, bucket := range rl.buckets {
		if bucket.lastUpdate.Before(staleThreshold) {
			delete(rl.buckets, senderID)
		}
	}
}
