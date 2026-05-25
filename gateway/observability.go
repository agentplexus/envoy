// Copyright 2025 John Wang. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package gateway

import (
	"context"
	"log/slog"
	"time"

	"github.com/plexusone/omniobserve/agentops"
	"github.com/plexusone/omniobserve/observops"
)

// ObservabilityConfig configures gateway observability.
type ObservabilityConfig struct {
	// ServiceName is the name of this gateway service.
	ServiceName string

	// ServiceVersion is the version of this gateway service.
	ServiceVersion string

	// ObservopsProvider is the observops provider for metrics/traces.
	ObservopsProvider observops.Provider

	// AgentopsStore is the agentops store for workflow/task tracking.
	AgentopsStore agentops.Store

	// Logger is the logger for observability events.
	Logger *slog.Logger
}

// Observability provides gateway instrumentation.
type Observability struct {
	config ObservabilityConfig
	logger *slog.Logger

	// Metrics
	connectionsTotal     observops.Counter
	connectionsActive    observops.UpDownCounter
	messagesTotal        observops.Counter
	messageLatency       observops.Histogram
	errorsTotal          observops.Counter
	toolInvocationsTotal observops.Counter
	toolLatency          observops.Histogram
}

// NewObservability creates a new observability instance.
func NewObservability(config ObservabilityConfig) (*Observability, error) {
	if config.ServiceName == "" {
		config.ServiceName = "omniagent-gateway"
	}
	if config.Logger == nil {
		config.Logger = slog.Default()
	}

	o := &Observability{
		config: config,
		logger: config.Logger,
	}

	// Initialize metrics if provider is available
	if config.ObservopsProvider != nil {
		if err := o.initMetrics(); err != nil {
			return nil, err
		}
	}

	return o, nil
}

// initMetrics initializes all metrics instruments.
func (o *Observability) initMetrics() error {
	meter := o.config.ObservopsProvider.Meter()

	var err error

	o.connectionsTotal, err = meter.Counter("gateway.connections.total",
		observops.WithDescription("Total WebSocket connections"),
		observops.WithUnit("1"),
	)
	if err != nil {
		return err
	}

	o.connectionsActive, err = meter.UpDownCounter("gateway.connections.active",
		observops.WithDescription("Currently active WebSocket connections"),
		observops.WithUnit("1"),
	)
	if err != nil {
		return err
	}

	o.messagesTotal, err = meter.Counter("gateway.messages.total",
		observops.WithDescription("Total messages processed"),
		observops.WithUnit("1"),
	)
	if err != nil {
		return err
	}

	o.messageLatency, err = meter.Histogram("gateway.messages.latency",
		observops.WithDescription("Message processing latency"),
		observops.WithUnit("ms"),
	)
	if err != nil {
		return err
	}

	o.errorsTotal, err = meter.Counter("gateway.errors.total",
		observops.WithDescription("Total errors"),
		observops.WithUnit("1"),
	)
	if err != nil {
		return err
	}

	o.toolInvocationsTotal, err = meter.Counter("gateway.tools.invocations.total",
		observops.WithDescription("Total tool invocations"),
		observops.WithUnit("1"),
	)
	if err != nil {
		return err
	}

	o.toolLatency, err = meter.Histogram("gateway.tools.latency",
		observops.WithDescription("Tool invocation latency"),
		observops.WithUnit("ms"),
	)
	if err != nil {
		return err
	}

	return nil
}

// TraceContext holds trace context for a request.
type TraceContext struct {
	ctx       context.Context
	span      observops.Span
	startTime time.Time
}

// StartTrace starts a new trace for a gateway operation.
func (o *Observability) StartTrace(ctx context.Context, operationName string, attrs ...observops.KeyValue) *TraceContext {
	tc := &TraceContext{
		ctx:       ctx,
		startTime: time.Now(),
	}

	// Start observops span if provider available
	if o.config.ObservopsProvider != nil {
		tracer := o.config.ObservopsProvider.Tracer()
		tc.ctx, tc.span = tracer.Start(ctx, operationName,
			observops.WithSpanAttributes(attrs...),
		)
	}

	return tc
}

// EndTrace ends a trace with optional error.
func (o *Observability) EndTrace(tc *TraceContext, err error) {
	if tc == nil {
		return
	}

	duration := time.Since(tc.startTime)

	if tc.span != nil {
		if err != nil {
			tc.span.RecordError(err)
			tc.span.SetStatus(observops.StatusCodeError, err.Error())
		} else {
			tc.span.SetStatus(observops.StatusCodeOK, "")
		}
		tc.span.End()
	}

	// Record latency metric
	if o.messageLatency != nil {
		o.messageLatency.Record(tc.ctx, float64(duration.Milliseconds()))
	}

	// Record error metric
	if err != nil && o.errorsTotal != nil {
		o.errorsTotal.Add(tc.ctx, 1)
	}
}

// RecordClientConnect records a client connection event.
func (o *Observability) RecordClientConnect(ctx context.Context, clientID string) {
	if o.connectionsTotal != nil {
		o.connectionsTotal.Add(ctx, 1,
			observops.WithAttributes(observops.Attribute("client.id", clientID)),
		)
	}
	if o.connectionsActive != nil {
		o.connectionsActive.Add(ctx, 1)
	}

	o.logger.Info("client connected",
		"client_id", clientID,
	)
}

// RecordClientDisconnect records a client disconnection event.
func (o *Observability) RecordClientDisconnect(ctx context.Context, clientID string) {
	if o.connectionsActive != nil {
		o.connectionsActive.Add(ctx, -1)
	}

	o.logger.Info("client disconnected",
		"client_id", clientID,
	)
}

// RecordMessage records a message processing event.
func (o *Observability) RecordMessage(ctx context.Context, clientID string, msgType MessageType, err error) {
	if o.messagesTotal != nil {
		attrs := []observops.KeyValue{
			observops.Attribute("client.id", clientID),
			observops.Attribute("message.type", string(msgType)),
			observops.Attribute("status", statusFromError(err)),
		}
		o.messagesTotal.Add(ctx, 1, observops.WithAttributes(attrs...))
	}

	if err != nil && o.errorsTotal != nil {
		o.errorsTotal.Add(ctx, 1,
			observops.WithAttributes(
				observops.Attribute("error.type", "message_handler"),
				observops.Attribute("message.type", string(msgType)),
			),
		)
	}
}

// RecordToolInvocation records a tool invocation event.
func (o *Observability) RecordToolInvocation(ctx context.Context, toolName string, duration time.Duration, err error) {
	if o.toolInvocationsTotal != nil {
		o.toolInvocationsTotal.Add(ctx, 1,
			observops.WithAttributes(
				observops.Attribute("tool.name", toolName),
				observops.Attribute("status", statusFromError(err)),
			),
		)
	}

	if o.toolLatency != nil {
		o.toolLatency.Record(ctx, float64(duration.Milliseconds()),
			observops.WithAttributes(observops.Attribute("tool.name", toolName)),
		)
	}
}

// StartWorkflow starts a new agentops workflow for tracking.
func (o *Observability) StartWorkflow(ctx context.Context, name string, input map[string]any) (*agentops.Workflow, error) {
	if o.config.AgentopsStore == nil {
		return nil, nil
	}

	return o.config.AgentopsStore.StartWorkflow(ctx, name,
		agentops.WithWorkflowInput(input),
		agentops.WithWorkflowInitiator(o.config.ServiceName),
	)
}

// CompleteWorkflow completes an agentops workflow.
func (o *Observability) CompleteWorkflow(ctx context.Context, workflowID string, output map[string]any) error {
	if o.config.AgentopsStore == nil {
		return nil
	}

	return o.config.AgentopsStore.CompleteWorkflow(ctx, workflowID,
		agentops.WithWorkflowCompleteOutput(output),
	)
}

// FailWorkflow marks an agentops workflow as failed.
func (o *Observability) FailWorkflow(ctx context.Context, workflowID string, err error) error {
	if o.config.AgentopsStore == nil {
		return nil
	}

	return o.config.AgentopsStore.FailWorkflow(ctx, workflowID, err)
}

// RecordEvent records a generic event.
func (o *Observability) RecordEvent(ctx context.Context, eventType, category string, data map[string]any) error {
	if o.config.AgentopsStore == nil {
		return nil
	}

	_, err := o.config.AgentopsStore.EmitEvent(ctx, eventType,
		agentops.WithEventCategory(category),
		agentops.WithEventData(data),
	)
	return err
}

// Shutdown shuts down observability, flushing any buffered data.
func (o *Observability) Shutdown(ctx context.Context) error {
	if o.config.ObservopsProvider != nil {
		return o.config.ObservopsProvider.Shutdown(ctx)
	}
	return nil
}

// ForceFlush forces any buffered telemetry to be exported.
func (o *Observability) ForceFlush(ctx context.Context) error {
	if o.config.ObservopsProvider != nil {
		return o.config.ObservopsProvider.ForceFlush(ctx)
	}
	return nil
}

// Provider returns the observops provider.
func (o *Observability) Provider() observops.Provider {
	return o.config.ObservopsProvider
}

// Store returns the agentops store.
func (o *Observability) Store() agentops.Store {
	return o.config.AgentopsStore
}

func statusFromError(err error) string {
	if err != nil {
		return "error"
	}
	return "ok"
}
