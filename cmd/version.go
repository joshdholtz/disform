package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version vars are injected by goreleaser via ldflags.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print disform version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("disform %s (commit: %s, built: %s)\n", Version, Commit, Date)
	},
}
