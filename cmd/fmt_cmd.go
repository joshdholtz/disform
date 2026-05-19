package cmd

import (
	"bytes"
	"fmt"
	"os"

	"github.com/joshholtz/disform/internal/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var fmtCheck bool

var fmtCmd = &cobra.Command{
	Use:   "fmt",
	Short: "Normalize and format disform.yml",
	Long:  "Rewrites disform.yml with sorted keys and normalized values (e.g. uppercase hex colors). Use --check to verify without writing.",
	RunE:  runFmt,
}

func init() {
	fmtCmd.Flags().BoolVar(&fmtCheck, "check", false, "Check formatting without writing; exits 1 if the file would change")
}

func runFmt(cmd *cobra.Command, args []string) error {
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		return err
	}

	config.NormalizeConfig(cfg)

	formatted, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("formatting config: %w", err)
	}

	original, err := os.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("reading %s: %w", configFile, err)
	}

	if bytes.Equal(original, formatted) {
		if fmtCheck {
			fmt.Printf("%s is already formatted.\n", configFile)
		} else {
			fmt.Printf("%s is already formatted. No changes.\n", configFile)
		}
		return nil
	}

	if fmtCheck {
		fmt.Printf("%s is not formatted. Run `disform fmt` to fix.\n", configFile)
		os.Exit(1)
	}

	if err := os.WriteFile(configFile, formatted, 0644); err != nil {
		return fmt.Errorf("writing %s: %w", configFile, err)
	}

	fmt.Printf("Formatted %s\n", configFile)
	return nil
}
