package cmd

import (
	"fmt"

	"github.com/joshholtz/disform/internal/discord"
	"github.com/joshholtz/disform/internal/state"
	"github.com/spf13/cobra"
)

var refreshCmd = &cobra.Command{
	Use:   "refresh",
	Short: "Sync state file to match live Discord IDs",
	Long: `Updates the state file so each managed resource's Discord ID matches the
live server. Useful when resources were deleted and recreated outside of
disform — refresh re-links them by name so the next apply works correctly.`,
	RunE: runRefresh,
}

func runRefresh(cmd *cobra.Command, args []string) error {
	tok := getToken()
	if tok == "" {
		return fmt.Errorf("Discord bot token is required (set --token or DISCORD_TOKEN env var)")
	}

	st, err := state.LoadState(stateFile)
	if err != nil {
		return fmt.Errorf("loading state: %w", err)
	}

	if st.ServerID == "" {
		return fmt.Errorf("state file has no server_id; run `disform apply` or `disform import` first")
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

	// Build lookup maps indexed by name.
	liveRoleByName := make(map[string]string) // name → Discord ID
	for _, r := range liveRoles {
		liveRoleByName[r.Name] = r.ID
	}

	liveCatByName := make(map[string]string) // name → Discord ID
	catNameByID := make(map[string]string)   // Discord ID → name
	for _, ch := range liveChannels {
		if ch.Type == discord.ChannelTypeGuildCategory {
			liveCatByName[ch.Name] = ch.ID
			catNameByID[ch.ID] = ch.Name
		}
	}

	// key matches state.ChannelKey format: "CatName/chanName" or "/chanName"
	liveChanByKey := make(map[string]*discord.Channel)
	for _, ch := range liveChannels {
		if ch.Type == discord.ChannelTypeGuildCategory {
			continue
		}
		catName := ""
		if ch.ParentID != nil {
			catName = catNameByID[*ch.ParentID]
		}
		liveChanByKey[state.ChannelKey(catName, ch.Name)] = ch
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
	green := func(s string) string {
		if useColor {
			return "\033[32m" + s + "\033[0m"
		}
		return s
	}

	updated, removed := 0, 0

	for _, name := range sortedStateKeys(st.Roles) {
		rs := st.Roles[name]
		if liveID, ok := liveRoleByName[name]; ok {
			if rs.DiscordID != liveID {
				fmt.Printf("  %s role %q — ID updated (%s → %s)\n", yellow("~"), name, rs.DiscordID, liveID)
				st.SetRole(name, liveID)
				updated++
			}
		} else {
			fmt.Printf("  %s role %q (id: %s) — not found in Discord, removed from state\n", red("-"), name, rs.DiscordID)
			st.DeleteRole(name)
			removed++
		}
	}

	for _, name := range sortedStateKeys(st.Categories) {
		rs := st.Categories[name]
		if liveID, ok := liveCatByName[name]; ok {
			if rs.DiscordID != liveID {
				fmt.Printf("  %s category %q — ID updated (%s → %s)\n", yellow("~"), name, rs.DiscordID, liveID)
				st.SetCategory(name, liveID)
				updated++
			}
		} else {
			fmt.Printf("  %s category %q (id: %s) — not found in Discord, removed from state\n", red("-"), name, rs.DiscordID)
			st.DeleteCategory(name)
			removed++
		}
	}

	for _, key := range sortedStateKeys(st.Channels) {
		rs := st.Channels[key]
		catName, chanName, _ := splitChannelKey(key)
		if liveCh, ok := liveChanByKey[key]; ok {
			if rs.DiscordID != liveCh.ID {
				fmt.Printf("  %s channel %q — ID updated (%s → %s)\n", yellow("~"), key, rs.DiscordID, liveCh.ID)
				st.Channels[key] = state.ResourceState{DiscordID: liveCh.ID}
				updated++
			}
		} else {
			fmt.Printf("  %s channel %q (id: %s) — not found in Discord, removed from state\n", red("-"), key, rs.DiscordID)
			st.DeleteChannel(catName, chanName)
			removed++
		}
	}

	if updated == 0 && removed == 0 {
		fmt.Println("State is already up to date.")
		return nil
	}

	if err := state.SaveState(st, stateFile); err != nil {
		return fmt.Errorf("saving state: %w", err)
	}

	fmt.Printf("\n%s State saved — %d updated, %d removed.\n", green("✓"), updated, removed)
	return nil
}
