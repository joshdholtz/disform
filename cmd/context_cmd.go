package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var contextCmd = &cobra.Command{
	Use:   "context",
	Short: "Print LLM-ready context about disform's config format and commands",
	Long:  "Prints a concise description of disform's YAML config format, CLI commands, and conventions — useful for pasting into an LLM chat session.",
	RunE:  runContext,
}

func runContext(cmd *cobra.Command, args []string) error {
	fmt.Print(`# disform

disform is a Terraform-like CLI for declarative Discord server management.
It reads a YAML config file (disform.yml), compares it against live Discord
state, and applies changes via the Discord API.

## CLI commands

` + "```" + `
disform init --server-id <id>   Create a starter disform.yml
disform import --server-id <id> Import live server → disform.yml + state file
disform plan                    Show what changes would be applied (no changes made)
disform plan --json             Machine-readable JSON plan output
disform plan --detailed-exitcode  Exit 2 when changes exist (CI gate)
disform apply                   Apply config changes to Discord
disform apply --dry-run         Preview raw Discord API requests without making changes
disform apply --auto-approve    Skip confirmation prompts (for CI)
disform apply --target role.NAME  Apply only a specific resource
disform destroy                 Delete all managed resources from Discord
disform validate                Validate config file without hitting Discord
disform drift                   Show resources changed outside of disform
disform refresh                 Re-sync state file Discord IDs by name-matching
disform show [--server-id <id>] Pretty-print live server state
disform fmt                     Normalize config file formatting
disform fmt --check             Exit 1 if config would be changed by fmt
disform version                 Print version info
` + "```" + `

Global flags: --config/-c (default: disform.yml), --state/-s (default: disform.state.json), --token/-t (or DISCORD_TOKEN env var, or .env file)

## Config file format (disform.yml)

` + "```yaml" + `
server_id: "123456789012345678"

settings:                              # optional
  verification_level: none             # none | low | medium | high | very_high
  explicit_content_filter: disabled    # disabled | members_without_roles | all_members
  default_message_notifications: all_messages  # all_messages | only_mentions
  afk_timeout: 300                     # 0 | 60 | 300 | 900 | 1800 | 3600 (seconds)
  afk_channel: voice-afk               # channel name
  system_channel: arrivals             # channel name

roles:
  "@everyone":                         # base server permissions
    permissions: [view_channel, send_messages]
  admin:
    color: "#E74C3C"
    hoist: true                        # show separately in member list
    mentionable: true
    permissions: [administrator]
  moderator:
    color: "#3498DB"
    hoist: true

categories:
  general-cat:                         # stable key (used in state file, never changes)
    name: "General"                    # optional Discord display name; defaults to key
    position: 0
    channels:
      welcome-chan:                     # stable key
        name: "welcome"               # optional Discord display name; defaults to key
        type: text                     # text | voice | announcement | forum | stage
        topic: "Welcome to the server!"
        nsfw: false
        slowmode: 0                    # seconds between messages
        permissions:
          "@everyone":
            deny: [send_messages]
          moderator:
            allow: [send_messages]
      voice-chat:
        type: voice
        bitrate: 64000
        user_limit: 10

channels:                              # top-level channels (no category)
  rules:
    type: text
    topic: "Server rules"
` + "```" + `

## State file (disform.state.json)

Maps logical resource names to Discord IDs. Managed automatically — do not edit by hand.

` + "```json" + `
{
  "version": 1,
  "server_id": "123456789012345678",
  "roles":      { "admin":           { "discord_id": "111..." } },
  "categories": { "General":         { "discord_id": "222..." } },
  "channels":   { "General/welcome": { "discord_id": "333..." } }
}
` + "```" + `

Channel keys use "CategoryName/channel-name" format. Top-level channels use "/channel-name".

## Permissions

Valid permission names: administrator, view_channel, send_messages, manage_messages,
manage_channels, manage_guild, manage_roles, manage_webhooks, kick_members, ban_members,
add_reactions, embed_links, attach_files, read_message_history, mention_everyone,
use_external_emojis, connect, speak, mute_members, deafen_members, move_members,
use_vad, change_nickname, manage_nicknames, create_instant_invite, view_audit_log,
priority_speaker, stream, send_tts_messages, use_slash_commands, request_to_speak,
manage_threads, create_public_threads, create_private_threads, use_external_stickers,
send_messages_in_threads, use_embedded_activities, moderate_members, view_guild_insights.

## Key conventions

- Colors: "#RRGGBB" hex format
- @everyone role: use to set base server permissions; cannot be created or deleted
- Permission overwrites on channels: specify role name or "@everyone" as the key
- Run ` + "`disform plan`" + ` before ` + "`disform apply`" + ` to preview changes
- The .env file in the current directory is auto-loaded (DISCORD_TOKEN=...)
`)
	return nil
}
