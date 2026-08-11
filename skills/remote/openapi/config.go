// Copyright 2025 John Wang. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package openapi

import "net/http"

// AuthType specifies the authentication method.
type AuthType string

const (
	// AuthNone indicates no authentication.
	AuthNone AuthType = ""
	// AuthAPIKey indicates API key authentication.
	AuthAPIKey AuthType = "apiKey"
	// AuthBearer indicates Bearer token authentication.
	AuthBearer AuthType = "bearer"
	// AuthBasic indicates HTTP Basic authentication.
	AuthBasic AuthType = "basic"
)

// Config configures an OpenAPI skill.
type Config struct {
	// Name is the skill identifier (e.g., "petstore").
	// This must be unique among all skills registered with an agent.
	Name string

	// Description is a human-readable description of the skill.
	// If empty, uses the OpenAPI spec's info.description or a default.
	Description string

	// SpecURL is the URL to fetch the OpenAPI spec from.
	// Either SpecURL or SpecFile must be provided.
	SpecURL string

	// SpecFile is the path to a local OpenAPI spec file.
	// Either SpecURL or SpecFile must be provided.
	SpecFile string

	// BaseURL overrides the server URL from the OpenAPI spec.
	// If empty, uses the first server URL from the spec.
	BaseURL string

	// Auth configures authentication for API calls.
	Auth AuthConfig

	// IncludeOperations filters which operations to expose as tools.
	// If empty, all operations are included.
	// Use operation IDs (e.g., ["getPet", "listPets"]).
	IncludeOperations []string

	// ExcludeOperations filters which operations to exclude.
	// Applied after IncludeOperations.
	ExcludeOperations []string

	// IncludeTags filters operations by tags.
	// If set, only operations with at least one matching tag are included.
	IncludeTags []string

	// HTTPClient is the HTTP client to use for API calls.
	// If nil, http.DefaultClient is used.
	HTTPClient *http.Client

	// RequestTimeout is the timeout for individual API requests.
	// If zero, no timeout is applied beyond any client timeout.
	RequestTimeout int // seconds

	// LazyLoad defers spec loading until first tool call (default: false).
	LazyLoad bool
}

// AuthConfig configures authentication for API calls.
type AuthConfig struct {
	// Type is the authentication type.
	Type AuthType

	// APIKey is the API key value (for AuthAPIKey).
	APIKey string

	// APIKeyName is the header or query parameter name (for AuthAPIKey).
	// Defaults to "X-API-Key" for header, "api_key" for query.
	APIKeyName string

	// APIKeyIn specifies where to send the API key: "header" or "query".
	// Defaults to "header".
	APIKeyIn string

	// Token is the bearer token (for AuthBearer).
	Token string

	// Username is the username (for AuthBasic).
	Username string

	// Password is the password (for AuthBasic).
	Password string

	// APIKeyEnv, TokenEnv, and PasswordEnv name the injected-secret env var
	// (INIT-OMNIAGENT-004) whose value populates the corresponding credential
	// at SetSecrets time — e.g. TokenEnv: "GITHUB_TOKEN" makes an agent's
	// GITHUB_TOKEN secret the bearer token. When empty, no injection occurs
	// for that field (the static value above is used, unchanged behavior).
	APIKeyEnv   string
	TokenEnv    string
	PasswordEnv string //nolint:gosec // G117: names an env var, not a credential
}

// setDefaults applies default values to the config.
func (c *Config) setDefaults() {
	if c.Description == "" {
		c.Description = "OpenAPI skill: " + c.Name
	}
	if c.Auth.APIKeyIn == "" {
		c.Auth.APIKeyIn = "header"
	}
	if c.Auth.APIKeyName == "" {
		if c.Auth.APIKeyIn == "header" {
			c.Auth.APIKeyName = "X-API-Key"
		} else {
			c.Auth.APIKeyName = "api_key"
		}
	}
}
