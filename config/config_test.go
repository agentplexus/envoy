package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefault(t *testing.T) {
	cfg := Default()

	// Gateway defaults
	if cfg.Gateway.Address != "127.0.0.1:18789" {
		t.Errorf("Gateway.Address = %s, want 127.0.0.1:18789", cfg.Gateway.Address)
	}
	if cfg.Gateway.ReadTimeout != 30*time.Second {
		t.Errorf("Gateway.ReadTimeout = %v, want 30s", cfg.Gateway.ReadTimeout)
	}

	// Agent defaults
	if cfg.Agent.Provider != "anthropic" {
		t.Errorf("Agent.Provider = %s, want anthropic", cfg.Agent.Provider)
	}
	if cfg.Agent.Temperature != 0.7 {
		t.Errorf("Agent.Temperature = %f, want 0.7", cfg.Agent.Temperature)
	}

	// Channels disabled by default
	if cfg.Channels.Telegram.Enabled {
		t.Error("Telegram should be disabled by default")
	}
	if cfg.Channels.Discord.Enabled {
		t.Error("Discord should be disabled by default")
	}

	// Tools
	if !cfg.Tools.Browser.Enabled {
		t.Error("Browser tool should be enabled by default")
	}
	if cfg.Tools.Shell.Enabled {
		t.Error("Shell tool should be disabled by default")
	}
}

func TestLoadYAML(t *testing.T) {
	// Create temp config file
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	content := `
gateway:
  address: "0.0.0.0:9000"
agent:
  provider: openai
  model: gpt-4
channels:
  telegram:
    enabled: true
`
	if err := os.WriteFile(cfgPath, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Gateway.Address != "0.0.0.0:9000" {
		t.Errorf("Gateway.Address = %s, want 0.0.0.0:9000", cfg.Gateway.Address)
	}
	if cfg.Agent.Provider != "openai" {
		t.Errorf("Agent.Provider = %s, want openai", cfg.Agent.Provider)
	}
	if cfg.Agent.Model != "gpt-4" {
		t.Errorf("Agent.Model = %s, want gpt-4", cfg.Agent.Model)
	}
	if !cfg.Channels.Telegram.Enabled {
		t.Error("Telegram should be enabled")
	}
}

func TestLoadJSON(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")

	content := `{
  "gateway": {"address": "localhost:8080"},
  "agent": {"provider": "gemini"}
}`
	if err := os.WriteFile(cfgPath, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Gateway.Address != "localhost:8080" {
		t.Errorf("Gateway.Address = %s, want localhost:8080", cfg.Gateway.Address)
	}
	if cfg.Agent.Provider != "gemini" {
		t.Errorf("Agent.Provider = %s, want gemini", cfg.Agent.Provider)
	}
}

func TestLoadEnv(t *testing.T) {
	// Set env vars
	os.Setenv("ENVOY_GATEWAY_ADDRESS", "192.168.1.1:5000")
	os.Setenv("ENVOY_AGENT_PROVIDER", "xai")
	os.Setenv("TELEGRAM_BOT_TOKEN", "test-token")
	defer func() {
		os.Unsetenv("ENVOY_GATEWAY_ADDRESS")
		os.Unsetenv("ENVOY_AGENT_PROVIDER")
		os.Unsetenv("TELEGRAM_BOT_TOKEN")
	}()

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Gateway.Address != "192.168.1.1:5000" {
		t.Errorf("Gateway.Address = %s, want 192.168.1.1:5000", cfg.Gateway.Address)
	}
	if cfg.Agent.Provider != "xai" {
		t.Errorf("Agent.Provider = %s, want xai", cfg.Agent.Provider)
	}
	if cfg.Channels.Telegram.Token != "test-token" {
		t.Errorf("Telegram.Token = %s, want test-token", cfg.Channels.Telegram.Token)
	}
	if !cfg.Channels.Telegram.Enabled {
		t.Error("Telegram should be auto-enabled when token is set")
	}
}

func TestLoadNonexistent(t *testing.T) {
	_, err := Load("/nonexistent/path/config.yaml")
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
}
