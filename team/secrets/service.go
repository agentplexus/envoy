package secrets

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/plexusone/omnivault/vault"
)

// Namespace prefixes partition the flat backing vault by tenant. Agent secrets
// live under "agents/<id>/<ENV_VAR>"; per-user (INIT-004) and global scopes
// reserve their own prefixes so the same store can host every tenant kind.
const (
	agentPrefix  = "agents"
	userPrefix   = "users"
	globalPrefix = "global"
)

// AgentNamespace returns the vault namespace for an agent's secrets.
func AgentNamespace(agentID uuid.UUID) string {
	return agentPrefix + "/" + agentID.String()
}

// Service reads secrets from the team's OmniVault-backed store, scoped per
// tenant. For RMI-310 it resolves an agent's secrets into the env map its
// runtime instance injects; per-user resolution (INIT-004) layers on the same
// store via the user namespace.
type Service struct {
	vault  vault.Vault
	logger *slog.Logger
}

// Config configures the secret service.
type Config struct {
	// Vault is the backing store (any omnivault provider). Required. Tenant
	// isolation is applied by the service via ScopedVault, so pass the raw
	// deployment vault, not a pre-scoped one.
	Vault vault.Vault
	// Logger defaults to slog.Default().
	Logger *slog.Logger
}

// NewService creates a secret service over the given vault.
func NewService(cfg Config) (*Service, error) {
	if cfg.Vault == nil {
		return nil, errors.New("secrets: vault is required")
	}
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{vault: cfg.Vault, logger: logger}, nil
}

// AgentVault returns a vault confined to the agent's namespace. Callers can only
// reach that agent's secrets through it.
func (s *Service) AgentVault(agentID uuid.UUID) *ScopedVault {
	return Scoped(s.vault, AgentNamespace(agentID))
}

// ResolveAgentSecrets returns the agent's secrets as an env map (env-var name →
// value), read from the agent's namespace. Because the namespace is applied by
// ScopedVault, this can only ever return the given agent's secrets — the basis
// for RMI-310's disjoint-isolation guarantee. An agent with no secrets yields an
// empty (non-nil) map.
func (s *Service) ResolveAgentSecrets(ctx context.Context, agentID uuid.UUID) (map[string]string, error) {
	av := s.AgentVault(agentID)
	names, err := av.List(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("list agent secrets: %w", err)
	}
	env := make(map[string]string, len(names))
	for _, name := range names {
		sec, err := av.Get(ctx, name)
		if err != nil {
			return nil, fmt.Errorf("get agent secret %q: %w", name, err)
		}
		env[name] = sec.String()
	}
	return env, nil
}

// ListAgentSecretNames returns the env-var names of an agent's set secrets,
// without their values — the write-only listing the management API/UI use to
// show set/unset state (INIT-OMNIAGENT-004). Never returns values.
func (s *Service) ListAgentSecretNames(ctx context.Context, agentID uuid.UUID) ([]string, error) {
	names, err := s.AgentVault(agentID).List(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("list agent secret names: %w", err)
	}
	return names, nil
}

// SetAgentSecret stores a single agent secret by env-var name. It is the write
// path used by tooling and tests; the management API/UI is later INIT-004 work.
func (s *Service) SetAgentSecret(ctx context.Context, agentID uuid.UUID, name, value string) error {
	if name == "" {
		return errors.New("secrets: name is required")
	}
	return s.AgentVault(agentID).Set(ctx, name, &vault.Secret{Value: value})
}

// DeleteAgentSecret removes a single agent secret by env-var name.
func (s *Service) DeleteAgentSecret(ctx context.Context, agentID uuid.UUID, name string) error {
	return s.AgentVault(agentID).Delete(ctx, name)
}
