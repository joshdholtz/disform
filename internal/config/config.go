package config

import (
	"fmt"
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

// ValidChannelTypes lists all supported Discord channel types.
var ValidChannelTypes = map[string]bool{
	"text":         true,
	"voice":        true,
	"announcement": true,
	"forum":        true,
	"stage":        true,
}

// discordPermissions maps permission name to its bit position value.
var discordPermissions = map[string]int64{
	"create_instant_invite":    1 << 0,
	"kick_members":             1 << 1,
	"ban_members":              1 << 2,
	"administrator":            1 << 3,
	"manage_channels":          1 << 4,
	"manage_guild":             1 << 5,
	"add_reactions":            1 << 6,
	"view_audit_log":           1 << 7,
	"priority_speaker":         1 << 8,
	"stream":                   1 << 9,
	"view_channel":             1 << 10,
	"send_messages":            1 << 11,
	"send_tts_messages":        1 << 12,
	"manage_messages":          1 << 13,
	"embed_links":              1 << 14,
	"attach_files":             1 << 15,
	"read_message_history":     1 << 16,
	"mention_everyone":         1 << 17,
	"use_external_emojis":      1 << 18,
	"view_guild_insights":      1 << 19,
	"connect":                  1 << 20,
	"speak":                    1 << 21,
	"mute_members":             1 << 22,
	"deafen_members":           1 << 23,
	"move_members":             1 << 24,
	"use_vad":                  1 << 25,
	"change_nickname":          1 << 26,
	"manage_nicknames":         1 << 27,
	"manage_roles":             1 << 28,
	"manage_webhooks":          1 << 29,
	"manage_emojis":            1 << 30,
	"use_slash_commands":       1 << 31,
	"request_to_speak":         1 << 32,
	"manage_threads":           1 << 34,
	"create_public_threads":    1 << 35,
	"create_private_threads":   1 << 36,
	"use_external_stickers":    1 << 37,
	"send_messages_in_threads": 1 << 38,
	"use_embedded_activities":  1 << 39,
	"moderate_members":         1 << 40,
}

// Config represents the top-level disform configuration.
type Config struct {
	ServerID   string                    `yaml:"server_id"`
	Roles      map[string]RoleConfig     `yaml:"roles"`
	Categories map[string]CategoryConfig `yaml:"categories"`
}

// RoleConfig defines a Discord role.
type RoleConfig struct {
	Color       string   `yaml:"color"`
	Hoist       bool     `yaml:"hoist"`
	Mentionable bool     `yaml:"mentionable"`
	Permissions []string `yaml:"permissions"`
}

// CategoryConfig defines a Discord channel category.
type CategoryConfig struct {
	Position int                      `yaml:"position"`
	Channels map[string]ChannelConfig `yaml:"channels"`
}

// ChannelConfig defines a Discord channel.
type ChannelConfig struct {
	Type        string                               `yaml:"type"`
	Topic       string                               `yaml:"topic"`
	Position    int                                  `yaml:"position"`
	NSFW        bool                                 `yaml:"nsfw"`
	SlowMode    int                                  `yaml:"slowmode"`
	Bitrate     int                                  `yaml:"bitrate"`
	UserLimit   int                                  `yaml:"user_limit"`
	Permissions map[string]PermissionOverwriteConfig `yaml:"permissions"`
}

// PermissionOverwriteConfig defines allow/deny permission overwrites for a role or member.
type PermissionOverwriteConfig struct {
	Allow []string `yaml:"allow"`
	Deny  []string `yaml:"deny"`
}

// LoadConfig reads and validates a YAML config file.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file %q: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file %q: %w", path, err)
	}

	if cfg.Roles == nil {
		cfg.Roles = make(map[string]RoleConfig)
	}
	if cfg.Categories == nil {
		cfg.Categories = make(map[string]CategoryConfig)
	}

	if err := Validate(&cfg); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &cfg, nil
}

// Validate checks that the config is semantically valid.
func Validate(c *Config) error {
	if c.ServerID == "" {
		return fmt.Errorf("server_id is required")
	}

	for roleName, role := range c.Roles {
		if role.Color != "" {
			if _, err := ColorToInt(role.Color); err != nil {
				return fmt.Errorf("role %q: invalid color %q: %w", roleName, role.Color, err)
			}
		}
		for _, perm := range role.Permissions {
			if _, ok := discordPermissions[perm]; !ok {
				return fmt.Errorf("role %q: unknown permission %q", roleName, perm)
			}
		}
	}

	for catName, cat := range c.Categories {
		for chanName, ch := range cat.Channels {
			if ch.Type != "" {
				if !ValidChannelTypes[ch.Type] {
					return fmt.Errorf("category %q, channel %q: invalid type %q", catName, chanName, ch.Type)
				}
			}
			for target, overwrite := range ch.Permissions {
				for _, perm := range overwrite.Allow {
					if _, ok := discordPermissions[perm]; !ok {
						return fmt.Errorf("category %q, channel %q, permission target %q: unknown allow permission %q", catName, chanName, target, perm)
					}
				}
				for _, perm := range overwrite.Deny {
					if _, ok := discordPermissions[perm]; !ok {
						return fmt.Errorf("category %q, channel %q, permission target %q: unknown deny permission %q", catName, chanName, target, perm)
					}
				}
			}
		}
	}

	return nil
}

// NormalizeConfig normalizes config values to canonical form.
// Colors are converted to uppercase hex. Used by `disform fmt`.
func NormalizeConfig(c *Config) {
	for name, role := range c.Roles {
		if role.Color != "" {
			if val, err := ColorToInt(role.Color); err == nil {
				role.Color = fmt.Sprintf("#%06X", val)
				c.Roles[name] = role
			}
		}
	}
}

// ColorToInt converts a "#RRGGBB" hex string to an integer.
// The input must start with '#' followed by exactly 6 hex digits.
func ColorToInt(hex string) (int, error) {
	if len(hex) == 0 || hex[0] != '#' {
		return 0, fmt.Errorf("color must start with '#', got %q", hex)
	}
	h := hex[1:]
	if len(h) != 6 {
		return 0, fmt.Errorf("expected 6 hex digits after '#', got %q", h)
	}
	val, err := strconv.ParseInt(h, 16, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid hex color %q: %w", hex, err)
	}
	return int(val), nil
}

// PermissionsToInt converts a slice of permission name strings to a Discord bitfield int64.
func PermissionsToInt(perms []string) (int64, error) {
	var result int64
	for _, perm := range perms {
		bit, ok := discordPermissions[perm]
		if !ok {
			return 0, fmt.Errorf("unknown permission %q", perm)
		}
		result |= bit
	}
	return result, nil
}
