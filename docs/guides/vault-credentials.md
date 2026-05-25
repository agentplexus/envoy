# Vault Credentials

OmniAgent supports storing credentials in password managers via [omnivault](https://github.com/plexusone/omnivault). This guide covers supported providers, OS compatibility, and setup instructions.

## Overview

Instead of storing API keys and tokens in configuration files, you can reference them using vault URIs:

```yaml
agent:
  api_key: "op://MyVault/anthropic/api-key"  # 1Password

channels:
  telegram:
    token: "bw://org-id/telegram-token"      # Bitwarden
  discord:
    token: "env://DISCORD_BOT_TOKEN"         # Environment variable
```

Credentials are resolved once at startup. Plain string values still work for development.

## Supported Providers

| Provider | URI Scheme | Description |
|----------|------------|-------------|
| 1Password | `op://` | 1Password service accounts |
| Bitwarden | `bw://` | Bitwarden Secrets Manager |
| Keeper | `keeper://` | Keeper Secrets Manager |
| File | `file://` | Read from local file |
| Environment | `env://` | Read from environment variable |
| Memory | `memory://` | In-memory storage (testing) |

## OS Compatibility

!!! warning "Platform Limitations"
    Desktop vault providers (1Password, Bitwarden) require native libraries that may not be available on all platforms.

| Provider | macOS | Linux | Windows | Notes |
|----------|:-----:|:-----:|:-------:|-------|
| 1Password (`op://`) | :material-check: | :material-check: | :material-close: | Requires 1Password SDK |
| Bitwarden (`bw://`) | :material-check: | :material-check: | :material-close: | Requires Bitwarden SDK with CGO |
| Keeper (`keeper://`) | :material-check: | :material-check: | :material-check: | Pure Go implementation |
| File (`file://`) | :material-check: | :material-check: | :material-check: | Universal |
| Environment (`env://`) | :material-check: | :material-check: | :material-check: | Universal |
| Memory (`memory://`) | :material-check: | :material-check: | :material-check: | Universal |

### Windows Limitations

The 1Password and Bitwarden providers use native SDKs with CGO dependencies that require platform-specific DLLs. These DLLs are not available in standard Windows CI environments.

**Workarounds for Windows:**

1. **Use `env://` or `file://`** - These work on all platforms
2. **Use Keeper** - Pure Go implementation works everywhere
3. **Install native SDKs** - For local development, install the 1Password or Bitwarden CLI tools

## Provider Setup

### 1Password (`op://`)

1Password uses service accounts for programmatic access.

**URI Format:**
```
op://vault-name/item-name/field-name
```

**Environment Variables:**

| Variable | Required | Description |
|----------|:--------:|-------------|
| `OP_SERVICE_ACCOUNT_TOKEN` | Yes | Service account token (starts with `ops_`) |

**Setup:**

1. Create a service account in your 1Password account
2. Grant access to the vault containing your secrets
3. Set the token:

```bash
export OP_SERVICE_ACCOUNT_TOKEN="ops_..."
```

**Example:**
```yaml
agent:
  api_key: "op://Development/Anthropic API/credential"
```

### Bitwarden (`bw://`)

Bitwarden uses Secrets Manager for programmatic access.

**URI Format:**
```
bw://organization-id/secret-name
# or
bw://secret-name  # Uses BW_ORGANIZATION_ID
```

**Environment Variables:**

| Variable | Required | Description |
|----------|:--------:|-------------|
| `BW_ACCESS_TOKEN` | Yes | Machine account access token |
| `BW_ORGANIZATION_ID` | No | Default organization ID |
| `BW_API_URL` | No | Custom API URL (self-hosted) |
| `BW_IDENTITY_URL` | No | Custom Identity URL (self-hosted) |

**Setup:**

1. Enable Secrets Manager in your Bitwarden organization
2. Create a machine account with access to your secrets
3. Set the credentials:

```bash
export BW_ACCESS_TOKEN="..."
export BW_ORGANIZATION_ID="..."
```

**Example:**
```yaml
channels:
  telegram:
    token: "bw://telegram-bot-token"
```

### Keeper (`keeper://`)

Keeper uses Secrets Manager for programmatic access.

**URI Format:**
```
keeper://folder/record-title/field-name
# or
keeper://record-uid/field-name
```

**Environment Variables:**

| Variable | Required | Description |
|----------|:--------:|-------------|
| `KSM_TOKEN` | One of these | One-time token (format: `REGION:TOKEN`) |
| `KSM_CONFIG` | One of these | Base64-encoded config JSON |
| `KSM_CONFIG_FILE` | One of these | Path to config file |

**Setup:**

1. Create a Secrets Manager application in Keeper
2. Generate a one-time token or config file
3. Set the credentials:

```bash
export KSM_TOKEN="US:abc123..."
# or
export KSM_CONFIG_FILE="/path/to/config.json"
```

**Example:**
```yaml
voice:
  stt:
    api_key: "keeper://API Keys/Deepgram/api-key"
```

### File (`file://`)

Read secrets from local files.

**URI Format:**
```
file:///absolute/path/to/secret
```

**Example:**
```yaml
agent:
  api_key: "file:///etc/secrets/anthropic-api-key"
```

!!! tip "File Permissions"
    Ensure secret files have restricted permissions (e.g., `chmod 600`).

### Environment (`env://`)

Read secrets from environment variables.

**URI Format:**
```
env://VARIABLE_NAME
```

**Example:**
```yaml
agent:
  api_key: "env://ANTHROPIC_API_KEY"

channels:
  discord:
    token: "env://DISCORD_BOT_TOKEN"
```

!!! note "env:// vs ${VAR}"
    Both `env://VAR` and `${VAR}` read environment variables, but:

    - `${VAR}` - Expanded during config parsing (supports defaults: `${VAR:-default}`)
    - `env://VAR` - Resolved by omnivault during credential resolution

## OAuth Token Management

For services requiring OAuth token refresh (Google, Zoom, RingCentral), use the `tokens` configuration with [omnitoken](https://github.com/plexusone/omnitoken):

```yaml
tokens:
  vault_uri: "op://MyVault"
  services:
    google:
      credentials_name: "google-oauth"
      scopes:
        - "https://www.googleapis.com/auth/calendar"
    zoom:
      credentials_name: "zoom-oauth"
```

The token manager handles:

- In-memory token caching
- Automatic refresh when tokens expire
- Vault coordination for multi-process deployments
- Refresh token persistence

## Troubleshooting

### "unknown vault URI scheme"

The vault provider isn't registered. Ensure you're using a supported scheme.

### "failed to resolve credential"

Check that:

1. The required environment variables are set
2. The vault path is correct
3. Your service account has access to the secret

### Windows DLL errors

If you see `STATUS_DLL_NOT_FOUND` or similar errors on Windows:

1. Use `env://` or `file://` instead of `op://` or `bw://`
2. Or use Keeper (`keeper://`) which has a pure Go implementation

### 1Password "unauthorized" errors

Verify:

1. `OP_SERVICE_ACCOUNT_TOKEN` is set correctly
2. The service account has access to the vault
3. The token hasn't expired

### Bitwarden connection errors

Verify:

1. `BW_ACCESS_TOKEN` is set correctly
2. `BW_ORGANIZATION_ID` is set if not in URI
3. For self-hosted: `BW_API_URL` and `BW_IDENTITY_URL` are correct

## Best Practices

1. **Use vault credentials in production** - Never commit API keys to version control
2. **Use env:// in CI/CD** - CI systems typically inject secrets as environment variables
3. **Restrict service account permissions** - Only grant access to needed secrets
4. **Rotate credentials regularly** - Update service account tokens periodically
5. **Use different vaults per environment** - Separate dev/staging/production secrets
