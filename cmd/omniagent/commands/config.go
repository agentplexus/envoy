package commands

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/plexusone/omniagent/internal/redact"
)

var (
	configFormat string
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Configuration commands",
	Long:  "Commands for viewing and managing configuration.",
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	Long:  "Display the current configuration (with sensitive values redacted).",
	RunE:  showConfig,
}

func init() {
	configShowCmd.Flags().StringVar(&configFormat, "format", "yaml", "output format (yaml, json)")

	configCmd.AddCommand(configShowCmd)
}

func showConfig(cmd *cobra.Command, args []string) error {
	cfg := getConfig()

	var output []byte
	var err error

	switch configFormat {
	case "json":
		output, err = json.MarshalIndent(cfg, "", "  ")
	case "yaml":
		output, err = yaml.Marshal(cfg)
	default:
		return fmt.Errorf("unknown format: %s (use yaml or json)", configFormat)
	}

	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	// Every resolved credential/secret value was registered with the
	// redactor during config loading (RMI-OMNIAGENT-204); scrubbing the
	// rendered output this way covers all of them uniformly instead of a
	// hand-maintained per-field list that drifts as fields are added.
	fmt.Println(redact.String(string(output)))
	return nil
}
