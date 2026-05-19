package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/joshholtz/disform/internal/applier"
	"github.com/joshholtz/disform/internal/config"
	"github.com/joshholtz/disform/internal/discord"
	"github.com/joshholtz/disform/internal/planner"
	"github.com/joshholtz/disform/internal/state"
	"github.com/spf13/cobra"
)

var applyAutoApprove bool

var applyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Apply changes to Discord server",
	Long:  "Computes a plan and applies all changes to the Discord server.",
	RunE:  runApply,
}

func init() {
	applyCmd.Flags().BoolVar(&applyAutoApprove, "auto-approve", false, "Skip confirmation prompt")
}

func runApply(cmd *cobra.Command, args []string) error {
	tok := getToken()
	if tok == "" {
		return fmt.Errorf("Discord bot token is required (set --token or DISCORD_TOKEN env var)")
	}

	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	st, err := state.LoadState(stateFile)
	if err != nil {
		return fmt.Errorf("loading state: %w", err)
	}

	client := discord.NewHTTPClient(tok)

	live, err := fetchLiveState(client, cfg.ServerID)
	if err != nil {
		return fmt.Errorf("fetching live Discord state: %w", err)
	}

	p := planner.NewPlanner(cfg, st, live)
	plan, err := p.Plan()
	if err != nil {
		return fmt.Errorf("computing plan: %w", err)
	}

	printPlan(plan)

	if !plan.HasChanges() {
		fmt.Println("No changes to apply.")
		return nil
	}

	if !applyAutoApprove {
		fmt.Print("\nDo you want to apply these changes? (yes/no): ")
		reader := bufio.NewReader(os.Stdin)
		answer, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("reading input: %w", err)
		}
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer != "yes" {
			fmt.Println("Apply cancelled.")
			return nil
		}
	}

	fmt.Println()
	a := applier.NewApplier(client, st, cfg)

	for _, action := range plan.Actions {
		fmt.Printf("  Applying: %s %s %q... ", action.Type, action.ResourceType, action.Name)
		if err := a.ApplyAction(action); err != nil {
			fmt.Println("error!")
			// Save state even on error (partial progress).
			if saveErr := state.SaveState(st, stateFile); saveErr != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to save state: %v\n", saveErr)
			}
			return fmt.Errorf("%w", err)
		}
		fmt.Println("done")
	}

	if err := state.SaveState(st, stateFile); err != nil {
		return fmt.Errorf("saving state: %w", err)
	}

	fmt.Println("\nApply complete!")
	return nil
}
