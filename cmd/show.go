package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/joshholtz/disform/internal/config"
	"github.com/joshholtz/disform/internal/discord"
	"github.com/spf13/cobra"
)

var showServerID string

var showCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current live Discord server state",
	Long:  "Fetches and pretty-prints the live state of a Discord server: roles, categories, channels, and settings.",
	RunE:  runShow,
}

func init() {
	showCmd.Flags().StringVar(&showServerID, "server-id", "", "Discord server ID (defaults to server_id in config file)")
}

func runShow(cmd *cobra.Command, args []string) error {
	tok := getToken()
	if tok == "" {
		return fmt.Errorf("Discord bot token is required (set --token or DISCORD_TOKEN env var)")
	}

	serverID := showServerID
	if serverID == "" {
		cfg, err := config.LoadConfig(configFile)
		if err != nil {
			return fmt.Errorf("--server-id is required when no config file is present")
		}
		serverID = cfg.ServerID
	}

	client := discord.NewHTTPClient(tok)

	guild, err := client.GetGuild(serverID)
	if err != nil {
		return fmt.Errorf("fetching guild: %w", err)
	}

	channels, err := client.GetChannels(serverID)
	if err != nil {
		return fmt.Errorf("fetching channels: %w", err)
	}

	roles, err := client.GetRoles(serverID)
	if err != nil {
		return fmt.Errorf("fetching roles: %w", err)
	}

	fmt.Printf("Server: %s\n", guild.Name)
	fmt.Printf("ID:     %s\n", serverID)
	fmt.Println()

	printShowSettings(guild)
	printShowRoles(roles)
	printShowChannels(channels)

	return nil
}

func printShowSettings(guild *discord.Guild) {
	verificationNames := map[int]string{0: "none", 1: "low", 2: "medium", 3: "high", 4: "very_high"}
	contentFilterNames := map[int]string{0: "disabled", 1: "members_without_roles", 2: "all_members"}
	notificationNames := map[int]string{0: "all_messages", 1: "only_mentions"}

	vl := verificationNames[guild.VerificationLevel]
	cf := contentFilterNames[guild.ExplicitContentFilter]
	dn := notificationNames[guild.DefaultMessageNotifications]

	if vl == "none" && cf == "disabled" && dn == "all_messages" && guild.AFKTimeout == 0 {
		return
	}

	fmt.Println("Settings:")
	if vl != "none" {
		fmt.Printf("  Verification Level:            %s\n", vl)
	}
	if cf != "disabled" {
		fmt.Printf("  Explicit Content Filter:       %s\n", cf)
	}
	if dn != "all_messages" {
		fmt.Printf("  Default Message Notifications: %s\n", dn)
	}
	if guild.AFKTimeout != 0 {
		fmt.Printf("  AFK Timeout:                   %ds\n", guild.AFKTimeout)
	}
	fmt.Println()
}

func printShowRoles(roles []*discord.Role) {
	sorted := make([]*discord.Role, len(roles))
	copy(sorted, roles)
	// Highest position = highest in the role hierarchy.
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Position > sorted[j].Position
	})

	fmt.Printf("Roles (%d):\n", len(sorted))
	for _, r := range sorted {
		var attrs []string
		if r.Color != 0 {
			attrs = append(attrs, fmt.Sprintf("#%06X", r.Color))
		}
		if r.Hoist {
			attrs = append(attrs, "hoist")
		}
		if r.Mentionable {
			attrs = append(attrs, "mentionable")
		}
		if len(attrs) > 0 {
			fmt.Printf("  %s  (%s)\n", r.Name, strings.Join(attrs, ", "))
		} else {
			fmt.Printf("  %s\n", r.Name)
		}
	}
	fmt.Println()
}

func printShowChannels(channels []*discord.Channel) {
	// Separate categories, parented channels, and top-level channels.
	var categories []*discord.Channel
	byParent := make(map[string][]*discord.Channel)
	var topLevel []*discord.Channel

	for _, ch := range channels {
		switch {
		case ch.Type == discord.ChannelTypeGuildCategory:
			categories = append(categories, ch)
		case ch.ParentID != nil:
			byParent[*ch.ParentID] = append(byParent[*ch.ParentID], ch)
		default:
			topLevel = append(topLevel, ch)
		}
	}

	sort.Slice(categories, func(i, j int) bool {
		return categories[i].Position < categories[j].Position
	})
	sort.Slice(topLevel, func(i, j int) bool {
		return topLevel[i].Position < topLevel[j].Position
	})
	for _, children := range byParent {
		sort.Slice(children, func(i, j int) bool {
			return children[i].Position < children[j].Position
		})
	}

	totalChans := len(topLevel)
	for _, cat := range categories {
		totalChans += len(byParent[cat.ID])
	}
	fmt.Printf("Channels (%d categories, %d channels):\n", len(categories), totalChans)

	for _, cat := range categories {
		fmt.Printf("  [category] %s\n", cat.Name)
		for _, ch := range byParent[cat.ID] {
			printShowChannel(ch, "    ")
		}
	}

	if len(topLevel) > 0 {
		if len(categories) > 0 {
			fmt.Println("  (top-level)")
		}
		for _, ch := range topLevel {
			printShowChannel(ch, "  ")
		}
	}

	fmt.Println()
}

func printShowChannel(ch *discord.Channel, indent string) {
	typeStr := channelIntToString(ch.Type)
	var extras []string
	if ch.NSFW {
		extras = append(extras, "nsfw")
	}
	if ch.Topic != nil && *ch.Topic != "" {
		topic := *ch.Topic
		if len(topic) > 50 {
			topic = topic[:47] + "..."
		}
		extras = append(extras, fmt.Sprintf("topic: %q", topic))
	}
	if ch.Bitrate > 0 {
		extras = append(extras, fmt.Sprintf("%d kbps", ch.Bitrate/1000))
	}
	if ch.UserLimit > 0 {
		extras = append(extras, fmt.Sprintf("limit: %d", ch.UserLimit))
	}
	if len(extras) > 0 {
		fmt.Printf("%s[%s] %s  — %s\n", indent, typeStr, ch.Name, strings.Join(extras, ", "))
	} else {
		fmt.Printf("%s[%s] %s\n", indent, typeStr, ch.Name)
	}
}
