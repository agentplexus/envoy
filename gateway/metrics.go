// Copyright 2025 John Wang. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package gateway

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds Prometheus metrics for the gateway.
type Metrics struct {
	// Connection metrics
	ActiveConnections prometheus.Gauge
	TotalConnections  prometheus.Counter

	// Message metrics
	MessagesReceived  *prometheus.CounterVec
	MessagesSent      *prometheus.CounterVec
	MessageDurationMs *prometheus.HistogramVec
	RateLimitedCount  prometheus.Counter

	// Agent metrics
	AgentRequests   prometheus.Counter
	AgentErrors     prometheus.Counter
	AgentDurationMs prometheus.Histogram

	// Tool metrics
	ToolInvocations *prometheus.CounterVec
	ToolDurationMs  *prometheus.HistogramVec
}

// NewMetrics creates and registers Prometheus metrics.
func NewMetrics(namespace string) *Metrics {
	if namespace == "" {
		namespace = "omniagent"
	}

	return &Metrics{
		ActiveConnections: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "gateway",
			Name:      "active_connections",
			Help:      "Number of active WebSocket connections",
		}),
		TotalConnections: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "gateway",
			Name:      "connections_total",
			Help:      "Total number of WebSocket connections established",
		}),
		MessagesReceived: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "gateway",
			Name:      "messages_received_total",
			Help:      "Total number of messages received",
		}, []string{"type"}),
		MessagesSent: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "gateway",
			Name:      "messages_sent_total",
			Help:      "Total number of messages sent",
		}, []string{"type"}),
		MessageDurationMs: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: "gateway",
			Name:      "message_duration_milliseconds",
			Help:      "Message processing duration in milliseconds",
			Buckets:   prometheus.ExponentialBuckets(1, 2, 15), // 1ms to ~16s
		}, []string{"type"}),
		RateLimitedCount: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "gateway",
			Name:      "rate_limited_total",
			Help:      "Total number of rate-limited messages",
		}),
		AgentRequests: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "agent",
			Name:      "requests_total",
			Help:      "Total number of agent requests",
		}),
		AgentErrors: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "agent",
			Name:      "errors_total",
			Help:      "Total number of agent errors",
		}),
		AgentDurationMs: promauto.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: "agent",
			Name:      "duration_milliseconds",
			Help:      "Agent processing duration in milliseconds",
			Buckets:   prometheus.ExponentialBuckets(10, 2, 15), // 10ms to ~160s
		}),
		ToolInvocations: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "tools",
			Name:      "invocations_total",
			Help:      "Total number of tool invocations",
		}, []string{"tool", "status"}),
		ToolDurationMs: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: "tools",
			Name:      "duration_milliseconds",
			Help:      "Tool invocation duration in milliseconds",
			Buckets:   prometheus.ExponentialBuckets(1, 2, 15), // 1ms to ~16s
		}, []string{"tool"}),
	}
}

// Handler returns an HTTP handler for the /metrics endpoint.
func (m *Metrics) Handler() http.Handler {
	return promhttp.Handler()
}

// RecordConnection records a new connection.
func (m *Metrics) RecordConnection() {
	m.ActiveConnections.Inc()
	m.TotalConnections.Inc()
}

// RecordDisconnection records a disconnection.
func (m *Metrics) RecordDisconnection() {
	m.ActiveConnections.Dec()
}

// RecordMessage records a message received or sent.
func (m *Metrics) RecordMessage(msgType string, direction string, duration time.Duration) {
	if direction == "received" {
		m.MessagesReceived.WithLabelValues(msgType).Inc()
	} else {
		m.MessagesSent.WithLabelValues(msgType).Inc()
	}
	m.MessageDurationMs.WithLabelValues(msgType).Observe(float64(duration.Milliseconds()))
}

// RecordRateLimited records a rate-limited message.
func (m *Metrics) RecordRateLimited() {
	m.RateLimitedCount.Inc()
}

// RecordAgentRequest records an agent request.
func (m *Metrics) RecordAgentRequest(duration time.Duration, err error) {
	m.AgentRequests.Inc()
	m.AgentDurationMs.Observe(float64(duration.Milliseconds()))
	if err != nil {
		m.AgentErrors.Inc()
	}
}

// RecordToolInvocation records a tool invocation.
func (m *Metrics) RecordToolInvocation(toolName string, duration time.Duration, err error) {
	status := "success"
	if err != nil {
		status = "error"
	}
	m.ToolInvocations.WithLabelValues(toolName, status).Inc()
	m.ToolDurationMs.WithLabelValues(toolName).Observe(float64(duration.Milliseconds()))
}
