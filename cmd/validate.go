package cmd

import (
	"fmt"

	"github.com/joshholtz/disform/internal/config"
	"github.com/spf13/cobra"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate config file without connecting to Discord",
	Long:  "Loads and validates disform.yml, reporting any errors without making API calls.",
	RunE:  runValidate,
}

func runValidate(cmd *cobra.Command, args []string) error {
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		return err
	}

	totalChannels := 0
	for _, cat := range cfg.Categories {
		totalChannels += len(cat.Channels)
	}

	fmt.Printf("Config valid.\n")
	fmt.Printf("  server_id:  %s\n", cfg.ServerID)
	fmt.Printf("  roles:      %d\n", len(cfg.Roles))
	fmt.Printf("  categories: %d\n", len(cfg.Categories))
	fmt.Printf("  channels:   %d\n", totalChannels)
	return nil
}
