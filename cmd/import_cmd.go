package cmd

import (
	"fmt"
	"os"
	"sort"

	"github.com/joshholtz/disform/internal/discord"
	"github.com/joshholtz/disform/internal/state"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var importOutput string
var importServerID string

var importCmd = &cobra.Command{
	Use:   "import",
	Short: "Import existing Discord server into disform",
	Long:  "Fetches the current Discord server state and generates a config file and state file.",
	RunE:  runImport,
}

func init() {
	importCmd.Flags().StringVarP(&importOutput, "output", "o", "disform.yml", "Output config file path")
	importCmd.Flags().StringVar(&importServerID, "server-id", "", "Discord server ID to import (required)")
	_ = importCmd.MarkFlagRequired("server-id")
}

func runImport(cmd *cobra.Command, args []string) error {
	tok := getToken()
	if tok == "" {
		return fmt.Errorf("Discord bot token is required (set --token or DISCORD_TOKEN env var)")
	}

	client := discord.NewHTTPClient(tok)

	fmt.Printf("Importing server %s...\n", importServerID)

	// Fetch guild info.
	guild, err := client.GetGuild(importServerID)
	if err != nil {
		return fmt.Errorf("fetching guild: %w", err)
	}
	fmt.Printf("Server: %s\n", guild.Name)

	// Fetch channels and roles.
	channels, err := client.GetChannels(importServerID)
	if err != nil {
		return fmt.Errorf("fetching channels: %w", err)
	}

	roles, err := client.GetRoles(importServerID)
	if err != nil {
		return fmt.Errorf("fetching roles: %w", err)
	}

	// Build config and state.
	cfg, st := buildImportedConfig(importServerID, guild, channels, roles)

	// Write config file.
	configData, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshalling config: %w", err)
	}

	if err := os.WriteFile(importOutput, configData, 0644); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}
	fmt.Printf("Config written to %s\n", importOutput)

	// Write state file.
	if err := state.SaveState(st, stateFile); err != nil {
		return fmt.Errorf("writing state file: %w", err)
	}
	fmt.Printf("State written to %s\n", stateFile)

	// Print summary.
	fmt.Printf("\nImported:\n")
	fmt.Printf("  %d role(s)\n", len(st.Roles))
	fmt.Printf("  %d category/categories\n", len(st.Categories))
	fmt.Printf("  %d channel(s)\n", len(st.Channels))

	return nil
}

// importConfig is a temporary struct used for YAML marshalling during import.
type importConfig struct {
	ServerID   string                          `yaml:"server_id"`
	Roles      map[string]importRoleConfig     `yaml:"roles,omitempty"`
	Categories map[string]importCategoryConfig `yaml:"categories,omitempty"`
}

type importRoleConfig struct {
	Color       string `yaml:"color,omitempty"`
	Hoist       bool   `yaml:"hoist,omitempty"`
	Mentionable bool   `yaml:"mentionable,omitempty"`
}

type importCategoryConfig struct {
	Position int                            `yaml:"position"`
	Channels map[string]importChannelConfig `yaml:"channels,omitempty"`
}

type importChannelConfig struct {
	Type      string `yaml:"type"`
	Topic     string `yaml:"topic,omitempty"`
	Bitrate   int    `yaml:"bitrate,omitempty"`
	UserLimit int    `yaml:"user_limit,omitempty"`
	NSFW      bool   `yaml:"nsfw,omitempty"`
}

// buildImportedConfig creates a config and state from live Discord data.
func buildImportedConfig(serverID string, guild *discord.Guild, channels []*discord.Channel, roles []*discord.Role) (*importConfig, *state.State) {
	cfg := &importConfig{
		ServerID:   serverID,
		Roles:      make(map[string]importRoleConfig),
		Categories: make(map[string]importCategoryConfig),
	}
	st := state.NewState(serverID)

	// Map categories by ID for channel association.
	categoryByID := make(map[string]*discord.Channel)

	// Sort channels by position for deterministic output.
	sort.Slice(channels, func(i, j int) bool {
		return channels[i].Position < channels[j].Position
	})

	// First pass: process categories.
	for _, ch := range channels {
		if ch.Type == discord.ChannelTypeGuildCategory {
			categoryByID[ch.ID] = ch
			cfg.Categories[ch.Name] = importCategoryConfig{
				Position: ch.Position,
				Channels: make(map[string]importChannelConfig),
			}
			st.SetCategory(ch.Name, ch.ID)
		}
	}

	// Second pass: process channels.
	for _, ch := range channels {
		if ch.Type == discord.ChannelTypeGuildCategory {
			continue
		}

		chanCfg := importChannelConfig{
			Type: channelIntToString(ch.Type),
		}
		if ch.Topic != nil {
			chanCfg.Topic = *ch.Topic
		}
		if ch.Bitrate > 0 {
			chanCfg.Bitrate = ch.Bitrate
		}
		if ch.UserLimit > 0 {
			chanCfg.UserLimit = ch.UserLimit
		}
		chanCfg.NSFW = ch.NSFW

		if ch.ParentID != nil {
			if cat, ok := categoryByID[*ch.ParentID]; ok {
				catCfg := cfg.Categories[cat.Name]
				catCfg.Channels[ch.Name] = chanCfg
				cfg.Categories[cat.Name] = catCfg
				st.SetChannel(cat.Name, ch.Name, ch.ID)
				continue
			}
		}
		// Channel has no parent category — log it so the user knows.
		fmt.Printf("  warning: channel %q (id: %s) has no category and was skipped — add it to a category in disform.yml manually\n", ch.Name, ch.ID)
	}

	// Process roles (skip @everyone which is always position 0).
	for _, r := range roles {
		if r.Name == "@everyone" {
			continue
		}
		roleCfg := importRoleConfig{
			Hoist:       r.Hoist,
			Mentionable: r.Mentionable,
		}
		if r.Color != 0 {
			roleCfg.Color = fmt.Sprintf("#%06X", r.Color)
		}
		cfg.Roles[r.Name] = roleCfg
		st.SetRole(r.Name, r.ID)
	}

	return cfg, st
}

// channelIntToString converts a Discord channel type int to a config string.
func channelIntToString(t int) string {
	switch t {
	case discord.ChannelTypeGuildText:
		return "text"
	case discord.ChannelTypeGuildVoice:
		return "voice"
	case discord.ChannelTypeGuildAnnouncement:
		return "announcement"
	case discord.ChannelTypeGuildStage:
		return "stage"
	case discord.ChannelTypeGuildForum:
		return "forum"
	default:
		return "text"
	}
}
