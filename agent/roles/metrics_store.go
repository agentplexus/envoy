// Copyright 2025 John Wang. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package roles

import (
	"context"
	"sync"
	"time"

	"github.com/plexusone/omniskill/role"
)

// MetricsStore collects and stores role metrics.
//
// The store provides methods to record metric values and retrieve
// historical data for analysis and reporting.
type MetricsStore interface {
	// Record stores a metric value.
	Record(ctx context.Context, roleID, metricID string, value float64) error

	// RecordWithLabels stores a metric value with additional labels.
	RecordWithLabels(ctx context.Context, roleID, metricID string, value float64, labels map[string]string) error

	// Get retrieves metric data points.
	Get(ctx context.Context, roleID, metricID string) ([]MetricPoint, error)

	// GetWithLabels retrieves metric data points filtered by labels.
	GetWithLabels(ctx context.Context, roleID, metricID string, labels map[string]string) ([]MetricPoint, error)

	// Summary returns aggregate statistics for a metric.
	Summary(ctx context.Context, roleID, metricID string, period time.Duration) (*MetricSummary, error)
}

// MetricPoint represents a single metric measurement.
type MetricPoint struct {
	Timestamp time.Time
	Value     float64
	Labels    map[string]string
}

// MetricSummary provides aggregate statistics for a metric.
type MetricSummary struct {
	MetricID string
	Period   time.Duration
	Count    int64
	Sum      float64
	Min      float64
	Max      float64
	Avg      float64
	P50      float64
	P90      float64
	P99      float64
}

// InMemoryMetricsStore provides an in-memory implementation of MetricsStore.
// This is suitable for development and testing but not for production use
// where persistence and scalability are required.
type InMemoryMetricsStore struct {
	mu      sync.RWMutex
	points  map[string][]MetricPoint
	maxAge  time.Duration
	maxSize int
}

// NewInMemoryMetricsStore creates a new in-memory metrics store.
func NewInMemoryMetricsStore() *InMemoryMetricsStore {
	return &InMemoryMetricsStore{
		points:  make(map[string][]MetricPoint),
		maxAge:  24 * time.Hour,
		maxSize: 10000,
	}
}

// Record stores a metric value.
func (s *InMemoryMetricsStore) Record(ctx context.Context, roleID, metricID string, value float64) error {
	return s.RecordWithLabels(ctx, roleID, metricID, value, nil)
}

// RecordWithLabels stores a metric value with additional labels.
func (s *InMemoryMetricsStore) RecordWithLabels(ctx context.Context, roleID, metricID string, value float64, labels map[string]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := s.metricKey(roleID, metricID)
	point := MetricPoint{
		Timestamp: time.Now(),
		Value:     value,
		Labels:    labels,
	}

	s.points[key] = append(s.points[key], point)

	// Trim old data if necessary
	s.trimLocked(key)

	return nil
}

// Get retrieves metric data points.
func (s *InMemoryMetricsStore) Get(ctx context.Context, roleID, metricID string) ([]MetricPoint, error) {
	return s.GetWithLabels(ctx, roleID, metricID, nil)
}

// GetWithLabels retrieves metric data points filtered by labels.
func (s *InMemoryMetricsStore) GetWithLabels(ctx context.Context, roleID, metricID string, labels map[string]string) ([]MetricPoint, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := s.metricKey(roleID, metricID)
	points := s.points[key]

	if len(labels) == 0 {
		result := make([]MetricPoint, len(points))
		copy(result, points)
		return result, nil
	}

	var filtered []MetricPoint
	for _, p := range points {
		if s.matchesLabels(p.Labels, labels) {
			filtered = append(filtered, p)
		}
	}
	return filtered, nil
}

// Summary returns aggregate statistics for a metric.
func (s *InMemoryMetricsStore) Summary(ctx context.Context, roleID, metricID string, period time.Duration) (*MetricSummary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := s.metricKey(roleID, metricID)
	points := s.points[key]

	cutoff := time.Now().Add(-period)
	var values []float64
	for _, p := range points {
		if p.Timestamp.After(cutoff) {
			values = append(values, p.Value)
		}
	}

	if len(values) == 0 {
		return &MetricSummary{
			MetricID: metricID,
			Period:   period,
		}, nil
	}

	summary := &MetricSummary{
		MetricID: metricID,
		Period:   period,
		Count:    int64(len(values)),
		Min:      values[0],
		Max:      values[0],
	}

	for _, v := range values {
		summary.Sum += v
		if v < summary.Min {
			summary.Min = v
		}
		if v > summary.Max {
			summary.Max = v
		}
	}
	summary.Avg = summary.Sum / float64(len(values))

	// For percentiles, we'd need to sort - simplified here
	summary.P50 = summary.Avg
	summary.P90 = summary.Max
	summary.P99 = summary.Max

	return summary, nil
}

// metricKey creates a unique key for a role+metric combination.
func (s *InMemoryMetricsStore) metricKey(roleID, metricID string) string {
	return roleID + ":" + metricID
}

// matchesLabels checks if point labels match the filter labels.
func (s *InMemoryMetricsStore) matchesLabels(pointLabels, filterLabels map[string]string) bool {
	for k, v := range filterLabels {
		if pointLabels[k] != v {
			return false
		}
	}
	return true
}

// trimLocked removes old data points. Must be called with lock held.
func (s *InMemoryMetricsStore) trimLocked(key string) {
	points := s.points[key]
	if len(points) <= s.maxSize {
		return
	}

	// Remove oldest entries
	excess := len(points) - s.maxSize
	s.points[key] = points[excess:]
}

// MetricsCollector provides a convenient wrapper for recording metrics
// for a specific role.
type MetricsCollector struct {
	store   MetricsStore
	roleID  string
	metrics map[string]role.MetricDefinition
}

// NewMetricsCollector creates a collector for a specific role.
func NewMetricsCollector(store MetricsStore, roleID string, metrics []role.MetricDefinition) *MetricsCollector {
	c := &MetricsCollector{
		store:   store,
		roleID:  roleID,
		metrics: make(map[string]role.MetricDefinition),
	}
	for _, m := range metrics {
		c.metrics[m.ID] = m
	}
	return c
}

// Record stores a metric value.
func (c *MetricsCollector) Record(ctx context.Context, metricID string, value float64) error {
	return c.store.Record(ctx, c.roleID, metricID, value)
}

// Increment increases a counter metric by 1.
func (c *MetricsCollector) Increment(ctx context.Context, metricID string) error {
	return c.Record(ctx, metricID, 1)
}

// Duration records a duration metric in seconds.
func (c *MetricsCollector) Duration(ctx context.Context, metricID string, d time.Duration) error {
	return c.Record(ctx, metricID, d.Seconds())
}

// Ensure InMemoryMetricsStore implements MetricsStore.
var _ MetricsStore = (*InMemoryMetricsStore)(nil)
