package cmd

import (
	"bufio"
	"fmt"
	"io"
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
var applyDryRun bool
var applyTargets []string

var applyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Apply changes to Discord server",
	Long:  "Computes a plan and applies all changes to the Discord server.",
	RunE:  runApply,
}

func init() {
	applyCmd.Flags().BoolVar(&applyAutoApprove, "auto-approve", false, "Skip confirmation prompt")
	applyCmd.Flags().BoolVar(&applyDryRun, "dry-run", false, "Show API requests that would be sent without making any changes")
	applyCmd.Flags().StringArrayVar(&applyTargets, "target", nil, "Apply only specific resources, e.g. --target role.admin --target channel.General/welcome")
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

	// Dry run: use a recording client against live state, skip lock and state save.
	if applyDryRun {
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
		if len(applyTargets) > 0 {
			plan, err = filterPlan(plan, applyTargets)
			if err != nil {
				return err
			}
		}
		printPlan(plan)
		if !plan.HasChanges() {
			fmt.Println("No changes — no API calls would be made.")
			return nil
		}
		rec := newDryRunClient(cfg.ServerID)
		a := applier.NewApplier(rec, st, cfg)
		for _, action := range plan.Actions {
			if err := a.ApplyAction(action); err != nil {
				return fmt.Errorf("dry-run action %s %q: %w", action.Type, action.Name, err)
			}
		}
		fmt.Println("\nAPI requests that would be sent:")
		fmt.Println()
		rec.PrintRecords()
		fmt.Println("Dry run complete. No changes were made.")
		return nil
	}

	lock, err := state.AcquireLock(stateFile)
	if err != nil {
		return err
	}
	defer lock.Release()

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

	if len(applyTargets) > 0 {
		plan, err = filterPlan(plan, applyTargets)
		if err != nil {
			return err
		}
	}

	printPlan(plan)

	if !plan.HasChanges() {
		fmt.Println("No changes to apply.")
		return nil
	}

	if !applyAutoApprove {
		ok, err := confirmApply(os.Stdin, plan)
		if err != nil {
			return err
		}
		if !ok {
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

// filterPlan returns a new plan containing only actions matching the given targets.
// Target format: "resourceType.name" e.g. "role.admin", "channel.General/welcome".
func filterPlan(plan *planner.Plan, targets []string) (*planner.Plan, error) {
	type targetKey struct {
		resourceType planner.ResourceType
		name         string
	}

	keys := make([]targetKey, 0, len(targets))
	for _, t := range targets {
		dot := strings.Index(t, ".")
		if dot < 0 {
			return nil, fmt.Errorf("invalid --target %q: expected format type.name (e.g. role.admin, channel.General/welcome)", t)
		}
		typePart := t[:dot]
		namePart := t[dot+1:]
		var rt planner.ResourceType
		switch typePart {
		case "role":
			rt = planner.ResourceRole
		case "category":
			rt = planner.ResourceCategory
		case "channel":
			rt = planner.ResourceChannel
		default:
			return nil, fmt.Errorf("invalid --target %q: resource type must be role, category, or channel", t)
		}
		keys = append(keys, targetKey{rt, namePart})
	}

	filtered := &planner.Plan{}
	for _, action := range plan.Actions {
		for _, k := range keys {
			if action.ResourceType == k.resourceType && action.Name == k.name {
				filtered.Actions = append(filtered.Actions, action)
				switch action.Type {
				case planner.ActionCreate:
					filtered.ToCreate++
				case planner.ActionUpdate:
					filtered.ToUpdate++
				case planner.ActionDelete:
					filtered.ToDelete++
				}
				break
			}
		}
	}
	return filtered, nil
}

// confirmApply prompts the user to confirm the apply. If the plan contains
// deletes, a second prompt requires typing the exact count of deletions.
// Returns true if the user confirmed, false if they cancelled.
func confirmApply(r io.Reader, plan *planner.Plan) (bool, error) {
	reader := bufio.NewReader(r)

	fmt.Print("\nDo you want to apply these changes? (yes/no): ")
	answer, err := reader.ReadString('\n')
	if err != nil {
		return false, fmt.Errorf("reading input: %w", err)
	}
	if strings.TrimSpace(strings.ToLower(answer)) != "yes" {
		return false, nil
	}

	var deletes []planner.Action
	for _, a := range plan.Actions {
		if a.Type == planner.ActionDelete {
			deletes = append(deletes, a)
		}
	}
	if len(deletes) == 0 {
		return true, nil
	}

	fmt.Println()
	fmt.Println("  The following resources will be permanently deleted:")
	for _, a := range deletes {
		fmt.Printf("    - %s %q\n", a.ResourceType, a.Name)
	}
	fmt.Printf("\n  Type the number of resources to delete (%d) to confirm: ", len(deletes))
	answer, err = reader.ReadString('\n')
	if err != nil {
		return false, fmt.Errorf("reading input: %w", err)
	}
	return strings.TrimSpace(answer) == fmt.Sprintf("%d", len(deletes)), nil
}
