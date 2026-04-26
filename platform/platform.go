// Package platform provides runtime platform abstractions for omniagent.
//
// Platforms define how the agent is deployed and run. The standalone platform
// runs a local HTTP server with WebSocket and webhook support. Future platforms
// may include AWS Lambda, Bedrock AgentCore, or other serverless environments.
//
// # Example Usage
//
//	// Create standalone platform
//	p, _ := standalone.New(standalone.Config{
//	    Address: ":8080",
//	    Channels: channelConfig,
//	})
//
//	// Run the platform with an agent
//	err := p.Run(ctx, myAgent)
package platform

import (
	"context"

	"github.com/plexusone/omniagent/agent"
)

// Platform defines the runtime environment for an agent.
// Implementations handle server lifecycle, message routing,
// and platform-specific integrations.
type Platform interface {
	// Run starts the platform and blocks until context is cancelled.
	// The agent is used to process incoming messages.
	Run(ctx context.Context, agent *agent.Agent) error

	// Name returns the platform identifier (e.g., "standalone", "lambda").
	Name() string
}

// Lifecycle provides optional lifecycle hooks for platforms.
type Lifecycle interface {
	// OnStart is called when the platform starts.
	OnStart(ctx context.Context) error

	// OnStop is called when the platform stops.
	OnStop(ctx context.Context) error
}

// HealthChecker provides health check capability.
type HealthChecker interface {
	// Health returns the platform health status.
	Health(ctx context.Context) Health
}

// Health represents platform health status.
type Health struct {
	Status  string         `json:"status"` // "healthy", "degraded", "unhealthy"
	Details map[string]any `json:"details,omitempty"`
}

// Healthy returns a healthy status.
func Healthy() Health {
	return Health{Status: "healthy"}
}

// Degraded returns a degraded status with details.
func Degraded(details map[string]any) Health {
	return Health{Status: "degraded", Details: details}
}

// Unhealthy returns an unhealthy status with details.
func Unhealthy(details map[string]any) Health {
	return Health{Status: "unhealthy", Details: details}
}
