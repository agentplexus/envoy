# Vault-Backed Credentials Roadmap

This document tracks the implementation of vault-backed credential management in omniagent.

## Overview

Omniagent now supports vault-backed credentials via omnivault and omnitoken, enabling secure credential storage in 1Password, Bitwarden, and Keeper.

## Tasks

### 1. Add TokenConfig to Config Struct

**Status:** Completed

Add `TokenConfig` to the main `Config` struct so OAuth token configuration can be loaded from config files.

**Files:**

- `config/config.go` - Add TokenConfig field to Config struct

### 2. Add Tests for Credential Resolution

**Status:** Completed

Add unit tests for the credential resolution logic in `credentials.go`.

**Files:**

- `config/credentials_test.go` - New test file

**Test cases:**

- `TestIsVaultURI` - Test vault URI detection
- `TestGetScheme` - Test scheme extraction
- `TestGetPath` - Test path extraction
- `TestResolveCredentials` - Test full resolution (with mock vault)

### 3. Add Tests for Token Management

**Status:** Completed

Add unit tests for the token management wrapper in `tokens.go`.

**Files:**

- `config/tokens_test.go` - New test file

**Test cases:**

- `TestNewTokenManager` - Test creation with valid/invalid config
- `TestHTTPClient` - Test client caching and retrieval

### 4. Update README with Vault Credentials

**Status:** Completed

Document the vault-backed credentials feature in the README.

**Files:**

- `README.md` - Add vault credentials section

**Content:**

- Supported vault providers (1Password, Bitwarden, Keeper)
- Configuration examples for static credentials
- Configuration examples for OAuth tokens
- Environment variables reference

### 5. Verify Full Build and Tests

**Status:** Completed

Ensure the entire project builds and all tests pass.

**Commands:**

```bash
go build ./...
go test ./...
```

## Implementation Order

1. Add TokenConfig to Config struct (required for other features)
2. Add credential resolution tests
3. Add token management tests
4. Update README
5. Verify full build and tests

## Completed

- [x] `config/credentials.go` - Static credential resolution
- [x] `config/tokens.go` - OAuth token management wrapper
- [x] `config/loader.go` - LoadWithContext with credential resolution
- [x] `go.mod` - Added omnitoken, omnivault, omnivault-desktop dependencies
- [x] `config/config.go` - Added TokenConfig to Config struct
- [x] `config/credentials_test.go` - Unit tests for credential resolution
- [x] `config/tokens_test.go` - Unit tests for token management
- [x] `README.md` - Vault-backed credentials documentation
