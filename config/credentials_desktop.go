//go:build !windows

package config

// Register desktop vault providers (1Password, Bitwarden, Keeper) on
// non-Windows platforms. These providers use system keychain APIs that
// require platform-specific libraries not available in all environments.
import _ "github.com/plexusone/omnivault-desktop"
