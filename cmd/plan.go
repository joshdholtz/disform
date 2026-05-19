package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/joshholtz/disform/internal/config"
	"github.com/joshholtz/disform/internal/discord"
	"github.com/joshholtz/disform/internal/planner"
	"github.com/joshholtz/disform/internal/state"
	"github.com/spf13/cobra"
)

var planDetailedExitCode bool
var planJSON bool

var planCmd = &cobra.Command{
	Use:   "plan",
	Short: "Show planned changes to Discord server",
	Long:  "Compares the current config against live Discord state and shows what changes would be applied.",
	RunE:  runPlan,
}

func init() {
	planCmd.Flags().BoolVar(&planDetailedExitCode, "detailed-exitcode", false, "Exit 2 when changes are present (useful for CI)")
	planCmd.Flags().BoolVar(&planJSON, "json", false, "Output plan as JSON")
}

func runPlan(cmd *cobra.Command, args []string) error {
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

	if planJSON {
		printPlanJSON(plan)
	} else {
		printPlan(plan)
	}

	if planDetailedExitCode && plan.HasChanges() {
		os.Exit(2)
	}
	return nil
}

// fetchLiveState fetches current Discord guild, channels, and roles into a LiveState.
func fetchLiveState(client discord.Client, serverID string) (*planner.LiveState, error) {
	guild, err := client.GetGuild(serverID)
	if err != nil {
		return nil, fmt.Errorf("fetching guild: %w", err)
	}

	channels, err := client.GetChannels(serverID)
	if err != nil {
		return nil, fmt.Errorf("fetching channels: %w", err)
	}

	roles, err := client.GetRoles(serverID)
	if err != nil {
		return nil, fmt.Errorf("fetching roles: %w", err)
	}

	live := &planner.LiveState{
		Guild:      guild,
		Roles:      make(map[string]*discord.Role),
		Categories: make(map[string]*discord.Channel),
		Channels:   make(map[string]*discord.Channel),
	}

	for _, r := range roles {
		live.Roles[r.ID] = r
	}

	for _, ch := range channels {
		switch ch.Type {
		case discord.ChannelTypeGuildCategory:
			live.Categories[ch.ID] = ch
		default:
			live.Channels[ch.ID] = ch
		}
	}

	return live, nil
}

// printPlan prints the plan to stdout with ANSI color coding.
func printPlan(plan *planner.Plan) {
	useColor := shouldUseColor()

	green := func(s string) string {
		if useColor {
			return "\033[32m" + s + "\033[0m"
		}
		return s
	}
	yellow := func(s string) string {
		if useColor {
			return "\033[33m" + s + "\033[0m"
		}
		return s
	}
	red := func(s string) string {
		if useColor {
			return "\033[31m" + s + "\033[0m"
		}
		return s
	}

	// Sort actions for deterministic output.
	actions := make([]planner.Action, len(plan.Actions))
	copy(actions, plan.Actions)
	sort.Slice(actions, func(i, j int) bool {
		if actions[i].ResourceType != actions[j].ResourceType {
			return resourceTypeOrder(actions[i].ResourceType) < resourceTypeOrder(actions[j].ResourceType)
		}
		return actions[i].Name < actions[j].Name
	})

	for _, action := range actions {
		switch action.Type {
		case planner.ActionCreate:
			fmt.Printf("  %s %s %q will be created\n",
				green("+"),
				action.ResourceType,
				action.Name,
			)
		case planner.ActionUpdate:
			fmt.Printf("  %s %s %q will be updated\n",
				yellow("~"),
				action.ResourceType,
				action.Name,
			)
			for _, change := range action.Changes {
				fmt.Printf("    %s: %q -> %q\n", change.Field, change.OldValue, change.NewValue)
			}
		case planner.ActionDelete:
			fmt.Printf("  %s %s %q will be deleted\n",
				red("-"),
				action.ResourceType,
				action.Name,
			)
		}
	}

	if plan.HasChanges() {
		fmt.Println()
	}
	fmt.Println(plan.Summary())
}

type planJSONOutput struct {
	ToCreate int              `json:"to_create"`
	ToUpdate int              `json:"to_update"`
	ToDelete int              `json:"to_delete"`
	Summary  string           `json:"summary"`
	Actions  []actionJSONItem `json:"actions"`
}

type actionJSONItem struct {
	Type         string           `json:"type"`
	ResourceType string           `json:"resource_type"`
	Name         string           `json:"name"`
	DiscordID    string           `json:"discord_id,omitempty"`
	Changes      []changeJSONItem `json:"changes,omitempty"`
}

type changeJSONItem struct {
	Field    string `json:"field"`
	OldValue string `json:"old_value"`
	NewValue string `json:"new_value"`
}

func printPlanJSON(plan *planner.Plan) {
	out := planJSONOutput{
		ToCreate: plan.ToCreate,
		ToUpdate: plan.ToUpdate,
		ToDelete: plan.ToDelete,
		Summary:  plan.Summary(),
		Actions:  make([]actionJSONItem, 0, len(plan.Actions)),
	}
	for _, a := range plan.Actions {
		item := actionJSONItem{
			Type:         string(a.Type),
			ResourceType: string(a.ResourceType),
			Name:         a.Name,
			DiscordID:    a.DiscordID,
		}
		for _, c := range a.Changes {
			item.Changes = append(item.Changes, changeJSONItem{
				Field:    c.Field,
				OldValue: c.OldValue,
				NewValue: c.NewValue,
			})
		}
		out.Actions = append(out.Actions, item)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(out)
}

func resourceTypeOrder(rt planner.ResourceType) int {
	switch rt {
	case planner.ResourceRole:
		return 0
	case planner.ResourceCategory:
		return 1
	case planner.ResourceChannel:
		return 2
	case planner.ResourceSettings:
		return 3
	default:
		return 4
	}
}

// shouldUseColor returns true if color output should be used.
func shouldUseColor() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	// Check if stdout is a terminal (file descriptor 1).
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}
