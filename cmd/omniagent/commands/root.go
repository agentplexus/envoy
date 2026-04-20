// Package commands implements the omniagent CLI commands.
package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/plexusone/omniagent/agent"
	"github.com/plexusone/omniagent/config"
)

var (
	cfgFile      string
	cfg          *config.Config
	agentOptions []agent.Option // Registered agent options (e.g., compiled skills)
)

// rootCmd is the base command for omniagent.
var rootCmd = &cobra.Command{
	Use:   "omniagent",
	Short: "Your AI representative across communication channels",
	Long: `OmniAgent is a personal AI assistant that routes messages across
multiple communication platforms, processes them via an AI agent,
and responds on your behalf.

Start the gateway:
  omniagent gateway run

Check channel status:
  omniagent channels status

Show configuration:
  omniagent config show`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip config loading for version command
		if cmd.Name() == "version" {
			return nil
		}

		var err error
		cfg, err = config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
		return nil
	},
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file (default: omniagent.yaml)")

	// Add subcommands
	rootCmd.AddCommand(gatewayCmd)
	rootCmd.AddCommand(channelsCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(skillsCmd)
	rootCmd.AddCommand(versionCmd)
}

// getConfig returns the loaded configuration.
func getConfig() *config.Config {
	if cfg == nil {
		cfg = &config.Config{}
		*cfg = config.Default()
	}
	return cfg
}

// RegisterAgentOption registers an agent option to be applied when creating the agent.
// This allows external packages (like grokify-omniagent) to inject compiled skills
// and other configuration before calling Execute().
//
// Example:
//
//	commands.RegisterAgentOption(agent.WithCompiledSkill(investSkill))
//	commands.RegisterAgentOption(agent.WithStorage(sqliteStorage))
//	commands.Execute()
func RegisterAgentOption(opt agent.Option) {
	agentOptions = append(agentOptions, opt)
}

// getAgentOptions returns all registered agent options.
func getAgentOptions() []agent.Option {
	return agentOptions
}
