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

// importServerSettings is used for YAML marshalling during import.
type importServerSettings struct {
	VerificationLevel           string `yaml:"verification_level,omitempty"`
	ExplicitContentFilter       string `yaml:"explicit_content_filter,omitempty"`
	DefaultMessageNotifications string `yaml:"default_message_notifications,omitempty"`
	AFKTimeout                  int    `yaml:"afk_timeout,omitempty"`
	AFKChannel                  string `yaml:"afk_channel,omitempty"`
	SystemChannel               string `yaml:"system_channel,omitempty"`
}

// importConfig is a temporary struct used for YAML marshalling during import.
type importConfig struct {
	ServerID   string                          `yaml:"server_id"`
	Settings   *importServerSettings           `yaml:"settings,omitempty"`
	Roles      map[string]importRoleConfig     `yaml:"roles,omitempty"`
	Channels   map[string]importChannelConfig  `yaml:"channels,omitempty"`
	Categories map[string]importCategoryConfig `yaml:"categories,omitempty"`
}

type importRoleConfig struct {
	Color       string   `yaml:"color,omitempty"`
	Hoist       bool     `yaml:"hoist,omitempty"`
	Mentionable bool     `yaml:"mentionable,omitempty"`
	Permissions []string `yaml:"permissions,omitempty"`
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
		Channels:   make(map[string]importChannelConfig),
		Categories: make(map[string]importCategoryConfig),
	}
	st := state.NewState(serverID)

	// Map categories by ID for channel association.
	categoryByID := make(map[string]*discord.Channel)
	// Map all channels by ID for resolving AFK/system channels by ID → name.
	channelByID := make(map[string]string) // id → name

	// Sort channels by position for deterministic output.
	sort.Slice(channels, func(i, j int) bool {
		return channels[i].Position < channels[j].Position
	})

	// First pass: process categories.
	for _, ch := range channels {
		channelByID[ch.ID] = ch.Name
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
		// Channel has no parent category — add as top-level channel.
		cfg.Channels[ch.Name] = chanCfg
		st.SetChannel("", ch.Name, ch.ID)
	}

	// Process roles.
	for _, r := range roles {
		if r.Name == "@everyone" {
			// Import @everyone permissions if non-zero.
			if r.Permissions != "" && r.Permissions != "0" {
				permsInt := int64(0)
				fmt.Sscanf(r.Permissions, "%d", &permsInt)
				permNames := permissionsToNames(permsInt)
				if len(permNames) > 0 {
					cfg.Roles["@everyone"] = importRoleConfig{
						Permissions: permNames,
					}
				}
			}
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

	// Populate server settings from guild.
	settings := buildImportSettings(guild, channelByID)
	if settings != nil {
		cfg.Settings = settings
	}

	return cfg, st
}

// buildImportSettings converts guild-level settings to importServerSettings.
func buildImportSettings(guild *discord.Guild, channelByID map[string]string) *importServerSettings {
	verificationLevelNames := map[int]string{0: "none", 1: "low", 2: "medium", 3: "high", 4: "very_high"}
	contentFilterNames := map[int]string{0: "disabled", 1: "members_without_roles", 2: "all_members"}
	notificationNames := map[int]string{0: "all_messages", 1: "only_mentions"}

	s := &importServerSettings{
		VerificationLevel:           verificationLevelNames[guild.VerificationLevel],
		ExplicitContentFilter:       contentFilterNames[guild.ExplicitContentFilter],
		DefaultMessageNotifications: notificationNames[guild.DefaultMessageNotifications],
	}

	if guild.AFKTimeout != 0 {
		s.AFKTimeout = guild.AFKTimeout
	}
	if guild.AFKChannelID != nil {
		if name, ok := channelByID[*guild.AFKChannelID]; ok {
			s.AFKChannel = name
		}
	}
	if guild.SystemChannelID != nil {
		if name, ok := channelByID[*guild.SystemChannelID]; ok {
			s.SystemChannel = name
		}
	}

	// Return nil if everything is default (no meaningful settings).
	if s.VerificationLevel == "none" && s.ExplicitContentFilter == "disabled" &&
		s.DefaultMessageNotifications == "all_messages" && s.AFKTimeout == 0 &&
		s.AFKChannel == "" && s.SystemChannel == "" {
		return nil
	}
	return s
}

// permissionsToNames converts a Discord permissions bitfield to a slice of permission name strings.
func permissionsToNames(perms int64) []string {
	permissionBits := map[int64]string{
		1 << 0:  "create_instant_invite",
		1 << 1:  "kick_members",
		1 << 2:  "ban_members",
		1 << 3:  "administrator",
		1 << 4:  "manage_channels",
		1 << 5:  "manage_guild",
		1 << 6:  "add_reactions",
		1 << 7:  "view_audit_log",
		1 << 8:  "priority_speaker",
		1 << 9:  "stream",
		1 << 10: "view_channel",
		1 << 11: "send_messages",
		1 << 12: "send_tts_messages",
		1 << 13: "manage_messages",
		1 << 14: "embed_links",
		1 << 15: "attach_files",
		1 << 16: "read_message_history",
		1 << 17: "mention_everyone",
		1 << 18: "use_external_emojis",
		1 << 19: "view_guild_insights",
		1 << 20: "connect",
		1 << 21: "speak",
		1 << 22: "mute_members",
		1 << 23: "deafen_members",
		1 << 24: "move_members",
		1 << 25: "use_vad",
		1 << 26: "change_nickname",
		1 << 27: "manage_nicknames",
		1 << 28: "manage_roles",
		1 << 29: "manage_webhooks",
		1 << 30: "manage_emojis",
		1 << 31: "use_slash_commands",
		1 << 32: "request_to_speak",
		1 << 34: "manage_threads",
		1 << 35: "create_public_threads",
		1 << 36: "create_private_threads",
		1 << 37: "use_external_stickers",
		1 << 38: "send_messages_in_threads",
		1 << 39: "use_embedded_activities",
		1 << 40: "moderate_members",
	}
	var names []string
	for bit, name := range permissionBits {
		if perms&bit != 0 {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
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
