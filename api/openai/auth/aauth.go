package auth

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// AAuthConfig holds AAuth configuration.
type AAuthConfig struct {
	// Enabled controls whether AAuth token validation is enabled.
	Enabled bool

	// IssuerURL is the URL of the AAuth issuer (PeopleServer).
	// Used for token validation and JWKS fetching.
	IssuerURL string

	// Audience is the expected audience claim in AAuth tokens.
	// Typically the URL of this service.
	Audience string

	// JWKSURL is the URL to fetch public keys for token verification.
	// If not set, defaults to {IssuerURL}/.well-known/jwks.json
	JWKSURL string
}

// AAuthVerifier verifies AAuth tokens using JWKS.
type AAuthVerifier struct {
	config     AAuthConfig
	httpClient *http.Client

	mu        sync.RWMutex
	keys      map[string]crypto.PublicKey
	lastFetch time.Time
	cacheTTL  time.Duration
}

// AAuthClaims represents the claims in an AAuth token.
type AAuthClaims struct {
	jwt.RegisteredClaims

	// Scope is the space-separated list of granted scopes.
	Scope string `json:"scope,omitempty"`

	// Act contains the actor (agent) information for delegation tokens.
	Act map[string]any `json:"act,omitempty"`
}

// NewAAuthVerifier creates a new AAuth token verifier.
func NewAAuthVerifier(cfg AAuthConfig) *AAuthVerifier {
	jwksURL := cfg.JWKSURL
	if jwksURL == "" && cfg.IssuerURL != "" {
		jwksURL = strings.TrimRight(cfg.IssuerURL, "/") + "/.well-known/jwks.json"
	}

	return &AAuthVerifier{
		config:     AAuthConfig{Enabled: cfg.Enabled, IssuerURL: cfg.IssuerURL, Audience: cfg.Audience, JWKSURL: jwksURL},
		httpClient: &http.Client{Timeout: 30 * time.Second},
		keys:       make(map[string]crypto.PublicKey),
		cacheTTL:   5 * time.Minute,
	}
}

// Verify validates an AAuth JWT token and returns the claims.
func (v *AAuthVerifier) Verify(ctx context.Context, tokenString string) (*AAuthClaims, error) {
	if !v.config.Enabled {
		return nil, fmt.Errorf("aauth not enabled")
	}

	// Parse token without verification to check if it looks like an AAuth token
	parser := jwt.NewParser()
	token, _, err := parser.ParseUnverified(tokenString, &AAuthClaims{})
	if err != nil {
		return nil, fmt.Errorf("parse token: %w", err)
	}

	// Get key ID from header
	kid, ok := token.Header["kid"].(string)
	if !ok || kid == "" {
		return nil, fmt.Errorf("missing kid header")
	}

	// Get public key for verification
	key, err := v.getKey(ctx, kid)
	if err != nil {
		return nil, fmt.Errorf("get key: %w", err)
	}

	// Parse and verify the token
	var parserOpts []jwt.ParserOption
	if v.config.IssuerURL != "" {
		parserOpts = append(parserOpts, jwt.WithIssuer(v.config.IssuerURL))
	}
	if v.config.Audience != "" {
		parserOpts = append(parserOpts, jwt.WithAudience(v.config.Audience))
	}

	verifyParser := jwt.NewParser(parserOpts...)
	verifiedToken, err := verifyParser.ParseWithClaims(tokenString, &AAuthClaims{}, func(token *jwt.Token) (any, error) {
		return key, nil
	})
	if err != nil {
		return nil, fmt.Errorf("verify token: %w", err)
	}

	claims, ok := verifiedToken.Claims.(*AAuthClaims)
	if !ok || !verifiedToken.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	return claims, nil
}

// IsAAuthToken checks if a token string appears to be an AAuth JWT.
// It looks for JWT structure and AAuth-specific characteristics.
func (v *AAuthVerifier) IsAAuthToken(tokenString string) bool {
	// Quick check: JWTs have 3 parts separated by dots
	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return false
	}

	// Try to decode the header to check for kid
	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return false
	}

	var header map[string]any
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		return false
	}

	// AAuth tokens should have a kid header
	_, hasKid := header["kid"]
	return hasKid
}

func (v *AAuthVerifier) getKey(ctx context.Context, kid string) (crypto.PublicKey, error) {
	v.mu.RLock()
	key, found := v.keys[kid]
	needsRefresh := time.Since(v.lastFetch) > v.cacheTTL
	v.mu.RUnlock()

	if found && !needsRefresh {
		return key, nil
	}

	// Refresh JWKS
	if err := v.refresh(ctx); err != nil {
		// If we have a cached key, use it even if refresh failed
		if found {
			return key, nil
		}
		return nil, err
	}

	v.mu.RLock()
	key, found = v.keys[kid]
	v.mu.RUnlock()

	if !found {
		return nil, fmt.Errorf("key not found: %s", kid)
	}

	return key, nil
}

func (v *AAuthVerifier) refresh(ctx context.Context) error {
	if v.config.JWKSURL == "" {
		return fmt.Errorf("JWKS URL not configured")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.config.JWKSURL, nil)
	if err != nil {
		return err
	}

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("JWKS fetch failed: status %d", resp.StatusCode)
	}

	var jwks struct {
		Keys []jwk `json:"keys"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return err
	}

	keys := make(map[string]crypto.PublicKey)
	for _, key := range jwks.Keys {
		pubKey, err := key.toPublicKey()
		if err != nil {
			continue // Skip invalid keys
		}
		keys[key.KeyID] = pubKey
	}

	v.mu.Lock()
	v.keys = keys
	v.lastFetch = time.Now()
	v.mu.Unlock()

	return nil
}

// jwk represents a JSON Web Key.
type jwk struct {
	KeyType   string `json:"kty"`
	KeyID     string `json:"kid"`
	Algorithm string `json:"alg,omitempty"`
	Use       string `json:"use,omitempty"`

	// RSA parameters
	N string `json:"n,omitempty"` // Modulus
	E string `json:"e,omitempty"` // Exponent

	// EC parameters
	Curve string `json:"crv,omitempty"` // Curve name
	X     string `json:"x,omitempty"`   // X coordinate
	Y     string `json:"y,omitempty"`   // Y coordinate
}

func (k *jwk) toPublicKey() (crypto.PublicKey, error) {
	switch k.KeyType {
	case "RSA":
		return k.toRSAPublicKey()
	case "EC":
		return k.toECPublicKey()
	default:
		return nil, fmt.Errorf("unsupported key type: %s", k.KeyType)
	}
}

func (k *jwk) toRSAPublicKey() (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(k.N)
	if err != nil {
		return nil, err
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(k.E)
	if err != nil {
		return nil, err
	}

	n := new(big.Int).SetBytes(nBytes)
	e := int(new(big.Int).SetBytes(eBytes).Int64())

	return &rsa.PublicKey{N: n, E: e}, nil
}

func (k *jwk) toECPublicKey() (*ecdsa.PublicKey, error) {
	xBytes, err := base64.RawURLEncoding.DecodeString(k.X)
	if err != nil {
		return nil, err
	}
	yBytes, err := base64.RawURLEncoding.DecodeString(k.Y)
	if err != nil {
		return nil, err
	}

	curve, err := curveFromName(k.Curve)
	if err != nil {
		return nil, err
	}

	return &ecdsa.PublicKey{
		Curve: curve,
		X:     new(big.Int).SetBytes(xBytes),
		Y:     new(big.Int).SetBytes(yBytes),
	}, nil
}

func curveFromName(name string) (elliptic.Curve, error) {
	switch name {
	case "P-256":
		return elliptic.P256(), nil
	case "P-384":
		return elliptic.P384(), nil
	case "P-521":
		return elliptic.P521(), nil
	default:
		return nil, fmt.Errorf("unsupported curve: %s", name)
	}
}
