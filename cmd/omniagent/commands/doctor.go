package commands

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/spf13/cobra"

	"github.com/plexusone/omniagent/config"
	"github.com/plexusone/omniagent/internal/version"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Diagnose configuration and environment",
	Long: `Run diagnostic checks on your OmniAgent installation.

This command checks:
  - Configuration file validity
  - Required environment variables
  - API key presence
  - Network connectivity
  - Optional dependencies`,
	RunE: runDoctor,
}

var (
	doctorVerbose bool
)

func init() {
	doctorCmd.Flags().BoolVarP(&doctorVerbose, "verbose", "v", false, "show detailed output")
	rootCmd.AddCommand(doctorCmd)
}

type checkResult struct {
	name    string
	status  string // "ok", "warn", "error"
	message string
}

func runDoctor(cmd *cobra.Command, args []string) error {
	fmt.Println("OmniAgent Doctor")
	fmt.Println("================")
	fmt.Println()

	// Version info
	info := version.Get()
	fmt.Printf("Version: %s\n", info.Version)
	fmt.Printf("Go:      %s\n", runtime.Version())
	fmt.Printf("OS/Arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Println()

	var results []checkResult

	// Check configuration
	results = append(results, checkConfig()...)

	// Check environment
	results = append(results, checkEnvironment()...)

	// Check network
	results = append(results, checkNetwork()...)

	// Check optional dependencies
	results = append(results, checkDependencies()...)

	// Print results
	fmt.Println("Checks")
	fmt.Println("------")

	okCount := 0
	warnCount := 0
	errorCount := 0

	for _, r := range results {
		icon := ""
		switch r.status {
		case "ok":
			icon = "✓"
			okCount++
		case "warn":
			icon = "!"
			warnCount++
		case "error":
			icon = "✗"
			errorCount++
		}

		fmt.Printf("[%s] %s: %s\n", icon, r.name, r.message)
	}

	fmt.Println()
	fmt.Printf("Summary: %d passed, %d warnings, %d errors\n", okCount, warnCount, errorCount)

	if errorCount > 0 {
		return fmt.Errorf("%d check(s) failed", errorCount)
	}
	return nil
}

func checkConfig() []checkResult {
	var results []checkResult

	// Check for config file
	configPaths := []string{
		"omniagent.yaml",
		"omniagent.yml",
		filepath.Join(os.Getenv("HOME"), ".config", "omniagent", "config.yaml"),
	}

	if cfgFile != "" {
		configPaths = []string{cfgFile}
	}

	var foundPath string
	for _, p := range configPaths {
		if _, err := os.Stat(p); err == nil { //nolint:gosec // paths are hardcoded or from safe source
			foundPath = p
			break
		}
	}

	if foundPath == "" {
		results = append(results, checkResult{
			name:    "Config file",
			status:  "warn",
			message: "No config file found (using defaults)",
		})
	} else {
		// Try to load it
		_, err := config.Load(foundPath)
		if err != nil {
			results = append(results, checkResult{
				name:    "Config file",
				status:  "error",
				message: fmt.Sprintf("Invalid config at %s: %v", foundPath, err),
			})
		} else {
			results = append(results, checkResult{
				name:    "Config file",
				status:  "ok",
				message: fmt.Sprintf("Found at %s", foundPath),
			})
		}
	}

	// Check API key
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}
	if cfg != nil && cfg.Agent.APIKey != "" {
		apiKey = cfg.Agent.APIKey
	}

	if apiKey == "" {
		results = append(results, checkResult{
			name:    "API key",
			status:  "error",
			message: "No API key configured (set ANTHROPIC_API_KEY or OPENAI_API_KEY)",
		})
	} else {
		results = append(results, checkResult{
			name:    "API key",
			status:  "ok",
			message: "API key configured",
		})
	}

	return results
}

func checkEnvironment() []checkResult {
	var results []checkResult

	// Check home directory
	home := os.Getenv("HOME")
	if home == "" {
		results = append(results, checkResult{
			name:    "HOME",
			status:  "warn",
			message: "HOME environment variable not set",
		})
	} else {
		results = append(results, checkResult{
			name:    "HOME",
			status:  "ok",
			message: home,
		})
	}

	// Check data directory
	dataDir := filepath.Join(home, ".local", "share", "omniagent")
	if _, err := os.Stat(dataDir); os.IsNotExist(err) { //nolint:gosec // path is constructed from HOME
		results = append(results, checkResult{
			name:    "Data directory",
			status:  "warn",
			message: fmt.Sprintf("%s does not exist (will be created on first run)", dataDir),
		})
	} else {
		results = append(results, checkResult{
			name:    "Data directory",
			status:  "ok",
			message: dataDir,
		})
	}

	// Check for channel-specific env vars
	if os.Getenv("TELEGRAM_BOT_TOKEN") != "" {
		results = append(results, checkResult{
			name:    "Telegram",
			status:  "ok",
			message: "TELEGRAM_BOT_TOKEN set",
		})
	}

	if os.Getenv("DISCORD_BOT_TOKEN") != "" {
		results = append(results, checkResult{
			name:    "Discord",
			status:  "ok",
			message: "DISCORD_BOT_TOKEN set",
		})
	}

	if os.Getenv("TWILIO_ACCOUNT_SID") != "" && os.Getenv("TWILIO_AUTH_TOKEN") != "" {
		results = append(results, checkResult{
			name:    "Twilio",
			status:  "ok",
			message: "TWILIO_ACCOUNT_SID and TWILIO_AUTH_TOKEN set",
		})
	}

	return results
}

func checkNetwork() []checkResult {
	var results []checkResult

	endpoints := []struct {
		name string
		url  string
	}{
		{"Anthropic API", "https://api.anthropic.com"},
		{"OpenAI API", "https://api.openai.com"},
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	for _, ep := range endpoints {
		resp, err := client.Head(ep.url)
		if err != nil {
			results = append(results, checkResult{
				name:    ep.name,
				status:  "error",
				message: fmt.Sprintf("Cannot connect: %v", err),
			})
		} else {
			resp.Body.Close()
			results = append(results, checkResult{
				name:    ep.name,
				status:  "ok",
				message: "Reachable",
			})
		}
	}

	return results
}

func checkDependencies() []checkResult {
	var results []checkResult

	// Optional dependencies
	optionalDeps := []struct {
		name    string
		binary  string
		purpose string
	}{
		{"Git", "git", "skill installation from git repositories"},
		{"Docker", "docker", "sandbox execution"},
		{"Chrome/Chromium", "chromium", "browser automation"},
		{"FFmpeg", "ffmpeg", "audio/video processing"},
	}

	for _, dep := range optionalDeps {
		_, err := exec.LookPath(dep.binary)
		if err != nil {
			// Try alternate names for Chrome
			if dep.binary == "chromium" {
				for _, alt := range []string{"google-chrome", "chrome", "chromium-browser"} {
					if _, err := exec.LookPath(alt); err == nil {
						results = append(results, checkResult{
							name:    dep.name,
							status:  "ok",
							message: fmt.Sprintf("Found as %s (%s)", alt, dep.purpose),
						})
						goto nextDep
					}
				}
			}

			results = append(results, checkResult{
				name:    dep.name,
				status:  "warn",
				message: fmt.Sprintf("Not found (%s)", dep.purpose),
			})
		} else {
			results = append(results, checkResult{
				name:    dep.name,
				status:  "ok",
				message: fmt.Sprintf("Found (%s)", dep.purpose),
			})
		}
	nextDep:
	}

	return results
}
