package commands

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/plexusone/omniagent/config"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Interactive setup wizard",
	Long: `Run the interactive setup wizard to configure OmniAgent.

This wizard helps you:
  - Set up your API key
  - Configure communication channels
  - Choose storage options
  - Create the configuration file`,
	RunE: runSetup,
}

var (
	setupOutput string
	setupForce  bool
)

func init() {
	setupCmd.Flags().StringVarP(&setupOutput, "output", "o", "omniagent.yaml", "output configuration file path")
	setupCmd.Flags().BoolVar(&setupForce, "force", false, "overwrite existing configuration file")
	rootCmd.AddCommand(setupCmd)
}

func runSetup(cmd *cobra.Command, args []string) error {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("OmniAgent Setup Wizard")
	fmt.Println("======================")
	fmt.Println()
	fmt.Println("This wizard will help you create a configuration file.")
	fmt.Println()

	// Check if config file exists
	if !setupForce {
		if _, err := os.Stat(setupOutput); err == nil {
			fmt.Printf("Configuration file %s already exists.\n", setupOutput)
			fmt.Print("Overwrite? [y/N] ")
			response, _ := reader.ReadString('\n')
			response = strings.TrimSpace(response)
			if response != "y" && response != "Y" {
				fmt.Println("Cancelled.")
				return nil
			}
		}
	}

	cfg := config.Default()

	// API Provider
	fmt.Println("Step 1: LLM Provider")
	fmt.Println("--------------------")
	fmt.Println("Which LLM provider would you like to use?")
	fmt.Println("  1. Anthropic (Claude)")
	fmt.Println("  2. OpenAI (GPT)")
	fmt.Print("Choice [1]: ")

	providerChoice, _ := reader.ReadString('\n')
	providerChoice = strings.TrimSpace(providerChoice)

	switch providerChoice {
	case "", "1":
		cfg.Agent.Provider = "anthropic"
		cfg.Agent.Model = "claude-sonnet-4-20250514"
		fmt.Println()
		fmt.Println("Enter your Anthropic API key.")
		fmt.Println("Get one at: https://console.anthropic.com/settings/keys")
		fmt.Print("API Key: ")
		apiKey, _ := reader.ReadString('\n')
		cfg.Agent.APIKey = strings.TrimSpace(apiKey)
	case "2":
		cfg.Agent.Provider = "openai"
		cfg.Agent.Model = "gpt-4o"
		fmt.Println()
		fmt.Println("Enter your OpenAI API key.")
		fmt.Println("Get one at: https://platform.openai.com/api-keys")
		fmt.Print("API Key: ")
		apiKey, _ := reader.ReadString('\n')
		cfg.Agent.APIKey = strings.TrimSpace(apiKey)
	default:
		fmt.Println("Invalid choice, using Anthropic.")
		cfg.Agent.Provider = "anthropic"
	}

	fmt.Println()

	// System Prompt
	fmt.Println("Step 2: System Prompt")
	fmt.Println("---------------------")
	fmt.Println("Enter a system prompt for the agent (or press Enter for default):")
	fmt.Print("> ")
	systemPrompt, _ := reader.ReadString('\n')
	systemPrompt = strings.TrimSpace(systemPrompt)
	if systemPrompt != "" {
		cfg.Agent.SystemPrompt = systemPrompt
	}

	fmt.Println()

	// Channels
	fmt.Println("Step 3: Communication Channels")
	fmt.Println("------------------------------")
	fmt.Println("Which channels would you like to enable?")

	// Telegram
	fmt.Print("Enable Telegram? [y/N] ")
	telegramChoice, _ := reader.ReadString('\n')
	telegramChoice = strings.TrimSpace(telegramChoice)
	if telegramChoice == "y" || telegramChoice == "Y" {
		cfg.Channels.Telegram.Enabled = true
		fmt.Println("Enter your Telegram bot token.")
		fmt.Println("Get one from @BotFather: https://t.me/botfather")
		fmt.Print("Token: ")
		token, _ := reader.ReadString('\n')
		cfg.Channels.Telegram.Token = strings.TrimSpace(token)
	}
	fmt.Println()

	// Discord
	fmt.Print("Enable Discord? [y/N] ")
	discordChoice, _ := reader.ReadString('\n')
	discordChoice = strings.TrimSpace(discordChoice)
	if discordChoice == "y" || discordChoice == "Y" {
		cfg.Channels.Discord.Enabled = true
		fmt.Println("Enter your Discord bot token.")
		fmt.Println("Get one from: https://discord.com/developers/applications")
		fmt.Print("Token: ")
		token, _ := reader.ReadString('\n')
		cfg.Channels.Discord.Token = strings.TrimSpace(token)

		fmt.Println("Enter Guild ID (optional):")
		fmt.Print("Guild ID: ")
		guildID, _ := reader.ReadString('\n')
		cfg.Channels.Discord.GuildID = strings.TrimSpace(guildID)
	}
	fmt.Println()

	// Observability
	fmt.Println("Step 4: Observability (Optional)")
	fmt.Println("---------------------------------")
	fmt.Print("Enable observability/tracing? [y/N] ")
	obsChoice, _ := reader.ReadString('\n')
	obsChoice = strings.TrimSpace(obsChoice)
	if obsChoice == "y" || obsChoice == "Y" {
		cfg.Observability.Enabled = true
		fmt.Println("Enter OpenTelemetry exporter endpoint:")
		fmt.Print("Endpoint: ")
		endpoint, _ := reader.ReadString('\n')
		cfg.Observability.Endpoint = strings.TrimSpace(endpoint)
	}
	fmt.Println()

	// Write configuration
	fmt.Println("Writing Configuration")
	fmt.Println("---------------------")

	// Create directory if needed
	dir := filepath.Dir(setupOutput)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("create directory: %w", err)
		}
	}

	// Marshal to YAML
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	// Write file
	if err := os.WriteFile(setupOutput, data, 0600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	fmt.Printf("Configuration written to %s\n", setupOutput)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Review and edit the configuration file")
	fmt.Println("  2. Run 'omniagent doctor' to verify your setup")
	fmt.Println("  3. Run 'omniagent gateway run' to start the agent")
	fmt.Println()
	fmt.Println("For more information, visit: https://plexusone.github.io/omniagent")

	return nil
}
