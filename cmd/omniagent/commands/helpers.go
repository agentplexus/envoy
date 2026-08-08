package commands

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/plexusone/omniagent/agent/registry"
	"github.com/plexusone/omniagent/config"
)

// registerAgentsFromConfig registers all agents from the config to the registry.
// It applies defaults from cfg.Agent for any unspecified fields.
func registerAgentsFromConfig(ctx context.Context, cfg *config.Config, agentRegistry *registry.Registry, logger *slog.Logger) error {
	for _, agentCfg := range cfg.Agents {
		regCfg := &registry.AgentConfig{
			ID:           agentCfg.ID,
			Name:         agentCfg.Name,
			Description:  agentCfg.Description,
			Provider:     agentCfg.Provider,
			Model:        agentCfg.Model,
			APIKey:       agentCfg.APIKey,
			BaseURL:      agentCfg.BaseURL,
			Temperature:  agentCfg.Temperature,
			MaxTokens:    agentCfg.MaxTokens,
			SystemPrompt: agentCfg.SystemPrompt,
			Timezone:     agentCfg.Timezone,
			AllowedTools: agentCfg.AllowedTools,
			DeniedTools:  agentCfg.DeniedTools,
			Enabled:      agentCfg.Enabled,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}

		// Use defaults from single agent config if not specified
		if regCfg.Provider == "" {
			regCfg.Provider = cfg.Agent.Provider
		}
		if regCfg.Model == "" {
			regCfg.Model = cfg.Agent.Model
		}
		if regCfg.APIKey == "" {
			regCfg.APIKey = cfg.Agent.APIKey
		}
		if regCfg.BaseURL == "" {
			regCfg.BaseURL = cfg.Agent.BaseURL
		}

		if err := agentRegistry.Create(ctx, regCfg); err != nil {
			return fmt.Errorf("create agent %s: %w", agentCfg.ID, err)
		}
		logger.Info("agent registered", "id", regCfg.ID, "name", regCfg.Name, "model", regCfg.Model)
	}
	return nil
}
