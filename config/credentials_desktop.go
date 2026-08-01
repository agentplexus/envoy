//go:build !windows && cgo

package config

// Register desktop vault providers (1Password, Bitwarden, Keeper) on
// non-Windows platforms with CGO. These providers use native libraries
// (Bitwarden SDK, system keychain) not available in static/container builds.
import _ "github.com/plexusone/omnivault-desktop"
