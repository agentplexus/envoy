package auth

import (
	"context"
	"fmt"

	"github.com/plexusone/agentauth"
)

// AgentAuthConfig configures agent authentication with ID-JAG and AAuth support.
type AgentAuthConfig struct {
	// Enabled controls whether agent authentication is enabled.
	Enabled bool

	// IDJAGEnabled enables ID-JAG token verification (automatic auth).
	IDJAGEnabled bool

	// IDJAGIssuers maps issuer URLs to JWKS URLs for ID-JAG verification.
	// If JWKS URL is empty, defaults to {issuer}/.well-known/jwks.json
	IDJAGIssuers map[string]string

	// IDJAGAudience is the expected audience for ID-JAG tokens.
	IDJAGAudience string

	// AAuthEnabled enables AAuth token verification (human consent).
	AAuthEnabled bool

	// AAuthIssuers maps issuer URLs to JWKS URLs for AAuth verification.
	// If JWKS URL is empty, defaults to {issuer}/.well-known/jwks.json
	AAuthIssuers map[string]string

	// AAuthAudience is the expected audience for AAuth tokens.
	AAuthAudience string

	// SensitiveActions require AAuth (human consent).
	// Default: write, delete, update, create, send, upload, admin
	SensitiveActions []string

	// ActionPolicy maps specific actions to required protocols.
	// Overrides default behavior for specific actions.
	ActionPolicy map[string]agentauth.Protocol
}

// DefaultAgentAuthConfig returns a default configuration with sensible defaults.
func DefaultAgentAuthConfig() *AgentAuthConfig {
	return &AgentAuthConfig{
		Enabled:      true,
		IDJAGEnabled: true,
		AAuthEnabled: true,
		IDJAGIssuers: make(map[string]string),
		AAuthIssuers: make(map[string]string),
		SensitiveActions: []string{
			"write",
			"delete",
			"update",
			"create",
			"send",
			"upload",
			"admin",
		},
		ActionPolicy: make(map[string]agentauth.Protocol),
	}
}

// AgentAuthVerifier verifies both ID-JAG and AAuth tokens with action-based routing.
type AgentAuthVerifier struct {
	config   *AgentAuthConfig
	verifier *agentauth.TokenVerifier
}

// NewAgentAuthVerifier creates a new agent authentication verifier.
func NewAgentAuthVerifier(cfg *AgentAuthConfig) *AgentAuthVerifier {
	if cfg == nil {
		cfg = DefaultAgentAuthConfig()
	}

	// Convert to agentauth config
	verifierConfig := &agentauth.VerifierConfig{
		IDJAGEnabled:     cfg.IDJAGEnabled,
		IDJAGIssuers:     cfg.IDJAGIssuers,
		IDJAGAudience:    cfg.IDJAGAudience,
		AAuthEnabled:     cfg.AAuthEnabled,
		AAuthIssuers:     cfg.AAuthIssuers,
		AAuthAudience:    cfg.AAuthAudience,
		SensitiveActions: cfg.SensitiveActions,
		ActionPolicy:     cfg.ActionPolicy,
		DefaultProtocol:  agentauth.ProtocolIDJAG,
	}

	verifier := agentauth.NewTokenVerifier(verifierConfig)

	// Register configured issuers
	for issuer, jwksURL := range cfg.IDJAGIssuers {
		verifier.AddIDJAGIssuer(issuer, jwksURL)
	}
	for issuer, jwksURL := range cfg.AAuthIssuers {
		verifier.AddAAuthIssuer(issuer, jwksURL)
	}

	return &AgentAuthVerifier{
		config:   cfg,
		verifier: verifier,
	}
}

// Verify verifies a token without action checking.
func (v *AgentAuthVerifier) Verify(ctx context.Context, token string) (*agentauth.TokenClaims, error) {
	if !v.config.Enabled {
		return nil, fmt.Errorf("agent authentication not enabled")
	}
	return v.verifier.Verify(ctx, token)
}

// VerifyForAction verifies a token and checks if it's valid for the given action.
func (v *AgentAuthVerifier) VerifyForAction(ctx context.Context, token, action string) (*agentauth.TokenClaims, error) {
	if !v.config.Enabled {
		return nil, fmt.Errorf("agent authentication not enabled")
	}
	return v.verifier.VerifyForAction(ctx, token, action)
}

// GetRequiredProtocol returns the required protocol for an action.
func (v *AgentAuthVerifier) GetRequiredProtocol(action string) agentauth.Protocol {
	return v.verifier.GetRequiredProtocol(action)
}

// IsSensitiveAction returns true if the action requires AAuth (human consent).
func (v *AgentAuthVerifier) IsSensitiveAction(action string) bool {
	return v.verifier.IsSensitiveAction(action)
}

// IsAgentToken checks if a token string appears to be an agent token (JWT).
func (v *AgentAuthVerifier) IsAgentToken(token string) bool {
	return agentauth.IsJWT(token)
}

// AddIDJAGIssuer adds a trusted ID-JAG issuer at runtime.
func (v *AgentAuthVerifier) AddIDJAGIssuer(issuerURL, jwksURL string) {
	v.verifier.AddIDJAGIssuer(issuerURL, jwksURL)
}

// AddAAuthIssuer adds a trusted AAuth issuer at runtime.
func (v *AgentAuthVerifier) AddAAuthIssuer(issuerURL, jwksURL string) {
	v.verifier.AddAAuthIssuer(issuerURL, jwksURL)
}

// AgentAuthClaims is an alias for agentauth.TokenClaims for convenience.
type AgentAuthClaims = agentauth.TokenClaims

// AgentAuthProtocol is an alias for agentauth.Protocol for convenience.
type AgentAuthProtocol = agentauth.Protocol

// Protocol constants for convenience.
const (
	ProtocolIDJAG = agentauth.ProtocolIDJAG
	ProtocolAAuth = agentauth.ProtocolAAuth
)
