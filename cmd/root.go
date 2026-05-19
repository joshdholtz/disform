package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	configFile string
	stateFile  string
	token      string
)

var rootCmd = &cobra.Command{
	Use:   "disform",
	Short: "Declarative Discord server management",
	Long: `disform is a Terraform-like tool for managing Discord server structure
(channels, categories, roles, permissions) declaratively via YAML config files.`,
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "disform.yml", "Config file path")
	rootCmd.PersistentFlags().StringVarP(&stateFile, "state", "s", "disform.state.json", "State file path")
	rootCmd.PersistentFlags().StringVarP(&token, "token", "t", "", "Discord bot token (or set DISCORD_TOKEN env var)")

	rootCmd.AddCommand(planCmd)
	rootCmd.AddCommand(applyCmd)
	rootCmd.AddCommand(destroyCmd)
	rootCmd.AddCommand(importCmd)
}

// getToken returns the bot token from flag or environment variable.
func getToken() string {
	if token != "" {
		return token
	}
	return os.Getenv("DISCORD_TOKEN")
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
