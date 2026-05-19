package cmd

import (
	"fmt"
	"sort"

	"github.com/joshholtz/disform/internal/discord"
	"github.com/joshholtz/disform/internal/state"
	"github.com/spf13/cobra"
)

var driftCmd = &cobra.Command{
	Use:   "drift",
	Short: "Show resources changed outside of disform",
	Long:  "Compares the state file against live Discord and reports resources that were modified or deleted without using disform apply.",
	RunE:  runDrift,
}

func runDrift(cmd *cobra.Command, args []string) error {
	tok := getToken()
	if tok == "" {
		return fmt.Errorf("Discord bot token is required (set --token or DISCORD_TOKEN env var)")
	}

	st, err := state.LoadState(stateFile)
	if err != nil {
		return fmt.Errorf("loading state: %w", err)
	}

	if len(st.Roles)+len(st.Categories)+len(st.Channels) == 0 {
		fmt.Println("No managed resources in state. Run `disform apply` first.")
		return nil
	}

	client := discord.NewHTTPClient(tok)

	liveChannels, err := client.GetChannels(st.ServerID)
	if err != nil {
		return fmt.Errorf("fetching channels: %w", err)
	}
	liveRoles, err := client.GetRoles(st.ServerID)
	if err != nil {
		return fmt.Errorf("fetching roles: %w", err)
	}

	liveRoleByID := make(map[string]*discord.Role, len(liveRoles))
	for _, r := range liveRoles {
		liveRoleByID[r.ID] = r
	}
	liveChanByID := make(map[string]*discord.Channel, len(liveChannels))
	for _, ch := range liveChannels {
		liveChanByID[ch.ID] = ch
	}

	useColor := shouldUseColor()
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

	driftCount := 0

	// Check roles.
	roleNames := sortedStateKeys(st.Roles)
	for _, name := range roleNames {
		rs := st.Roles[name]
		live, ok := liveRoleByID[rs.DiscordID]
		if !ok {
			fmt.Printf("  %s role %q (id: %s) was deleted outside of disform\n", red("deleted:"), name, rs.DiscordID)
			driftCount++
			continue
		}
		if live.Name != name {
			fmt.Printf("  %s role %q renamed to %q outside of disform\n", yellow("renamed:"), name, live.Name)
			driftCount++
		}
	}

	// Check categories.
	catNames := sortedStateKeys(st.Categories)
	for _, name := range catNames {
		rs := st.Categories[name]
		live, ok := liveChanByID[rs.DiscordID]
		if !ok {
			fmt.Printf("  %s category %q (id: %s) was deleted outside of disform\n", red("deleted:"), name, rs.DiscordID)
			driftCount++
			continue
		}
		if live.Name != name {
			fmt.Printf("  %s category %q renamed to %q outside of disform\n", yellow("renamed:"), name, live.Name)
			driftCount++
		}
	}

	// Check channels.
	chanKeys := sortedStateKeys(st.Channels)
	for _, key := range chanKeys {
		rs := st.Channels[key]
		live, ok := liveChanByID[rs.DiscordID]
		if !ok {
			fmt.Printf("  %s channel %q (id: %s) was deleted outside of disform\n", red("deleted:"), key, rs.DiscordID)
			driftCount++
			continue
		}
		_, chanName, _ := splitChannelKey(key)
		if live.Name != chanName {
			fmt.Printf("  %s channel %q renamed to %q outside of disform\n", yellow("renamed:"), key, live.Name)
			driftCount++
		}
	}

	if driftCount == 0 {
		fmt.Println("No drift detected. Live Discord matches the last applied state.")
	} else {
		fmt.Printf("\n%d drifted resource(s). Run `disform plan` to see the full remediation plan.\n", driftCount)
	}

	return nil
}

func sortedStateKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func splitChannelKey(key string) (string, string, bool) {
	for i, c := range key {
		if c == '/' {
			return key[:i], key[i+1:], true
		}
	}
	return "", key, false
}
