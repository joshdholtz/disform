# disform

[![CI](https://github.com/joshdholtz/disform/actions/workflows/ci.yml/badge.svg)](https://github.com/joshdholtz/disform/actions/workflows/ci.yml)

**Declarative Discord server management. Like Terraform, but for Discord.**

disform reads a YAML config file, compares it against your live server state, and applies only the changes needed — channels, categories, roles, permissions, and guild settings. A state file tracks what disform manages so it never touches resources it didn't create.

```
$ disform plan

  ~ update role admin
      color: "#aaaaaa" → "#E74C3C"

  + create channel General/announcements

  - delete channel General/old-chat

Plan: 1 to add, 1 to change, 1 to destroy.
```

---

## Table of Contents

- [Install](#install)
- [Quick Start](#quick-start)
- [How It Works](#how-it-works)
- [Commands](#commands)
- [Config Reference](#config-reference)
  - [settings](#settings)
  - [roles](#roles)
  - [categories](#categories)
  - [channels](#channels)
  - [Stable keys vs display names](#stable-keys-vs-display-names)
  - [Permission overwrites](#permission-overwrites)
- [Permissions](#permissions)
- [Channel Types](#channel-types)
- [State File](#state-file)
- [Environment & Auth](#environment--auth)
- [CI/CD Integration](#cicd-integration)
- [Bot Setup](#bot-setup)
- [Contributing](#contributing)

---

## Install

**Requires Go 1.21+**

```sh
go install github.com/joshdholtz/disform@latest
```

Or build from source:

```sh
git clone https://github.com/joshdholtz/disform
cd disform
go build -o disform .
```

Verify:

```sh
disform version
```

---

## Quick Start

**1. Get a bot token**

Create a bot at [discord.com/developers](https://discord.com/developers/applications), invite it to your server with `Manage Roles` and `Manage Channels` permissions, and copy the token. See [Bot Setup](#bot-setup) for the full walkthrough.

**2. Create a `.env` file** (or export the variable directly)

```sh
echo "DISCORD_TOKEN=your-token-here" > .env
```

**3. Import your existing server**

```sh
disform import --server-id YOUR_SERVER_ID
```

This creates `disform.yml` (your config) and `disform.state.json` (the state file).

**4. Edit `disform.yml`**

```yaml
server_id: "123456789012345678"

roles:
  admin:
    color: "#E74C3C"
    hoist: true
    mentionable: true
    permissions: [administrator]

  moderator:
    color: "#3498DB"
    hoist: true
    permissions: [kick_members, manage_messages]

categories:
  general:
    name: "General"
    position: 0
    channels:
      welcome:
        type: text
        topic: "Welcome to the server!"
        permissions:
          "@everyone":
            deny: [send_messages]
          moderator:
            allow: [send_messages]
      chat:
        name: "general-chat"
        type: text
```

**5. Preview changes**

```sh
disform plan
```

**6. Apply**

```sh
disform apply
```

---

## How It Works

disform uses a three-way diff at plan time:

```
disform.yml        ──┐
                     ├──► planner ──► []Action ──► applier ──► Discord API
disform.state.json ──┤                  ▲
                     └── live Discord ──┘ (fetched fresh each run)
```

**Desired state** (`disform.yml`) — what you want.  
**Tracked state** (`disform.state.json`) — what disform manages.  
**Live state** (Discord API) — what actually exists right now.

The planner compares all three:

| In config | In state | In Discord | Action |
|-----------|----------|------------|--------|
| ✓ | ✓ | ✓ | Compare fields → PATCH if changed |
| ✓ | ✓ | ✗ | Drift detected → re-create |
| ✓ | ✗ | — | Create |
| ✗ | ✓ | — | Delete |
| ✗ | ✗ | ✓ | Ignore (external resource) |

Resources not in the state file are never touched. disform coexists safely with channels or roles managed by hand or by other bots.

### Apply ordering

Creates and updates run in dependency order:

1. Roles (no dependencies)
2. Categories (no dependencies)
3. Channels (depend on categories for `parent_id`)
4. Guild settings

Deletes run in reverse order so children are removed before parents.

### Partial failure recovery

State is written after every successful action. If apply fails mid-run, re-run `disform apply` — already-applied resources are detected as up-to-date and skipped.

---

## Commands

### Global flags

All commands accept:

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--config` | `-c` | `disform.yml` | Config file path |
| `--state` | `-s` | `disform.state.json` | State file path |
| `--token` | `-t` | `$DISCORD_TOKEN` | Discord bot token |

---

### `disform init`

Create a starter `disform.yml` in the current directory.

```sh
disform init
disform init --server-id 123456789012345678
disform init --force   # overwrite existing file
```

---

### `disform import`

Fetch live server state and generate `disform.yml` + `disform.state.json`. Use this to bring an existing server under disform management.

```sh
disform import --server-id 123456789012345678
disform import --server-id 123456789012345678 --output staging.yml
```

---

### `disform plan`

Show what changes would be made. Reads live Discord state but makes no modifications.

```sh
disform plan
disform plan --json                  # machine-readable JSON for scripting
disform plan --detailed-exitcode     # exit 2 if changes exist (CI gate)
```

---

### `disform apply`

Compute a plan, prompt for confirmation, then execute all changes. Acquires a lock on the state file to prevent concurrent runs.

```sh
disform apply
disform apply --auto-approve

# Apply only specific resources:
disform apply --target role.admin
disform apply --target category.general
disform apply --target channel.general/welcome
```

`--target` can be repeated. Format: `role.<key>`, `category.<key>`, `channel.<categoryKey/channelKey>`.

---

### `disform destroy`

Delete every resource tracked in the state file, then clear it. Prompts for confirmation.

```sh
disform destroy
disform destroy --auto-approve
```

---

### `disform validate`

Check the config file for errors without connecting to Discord.

```sh
disform validate
disform validate --config staging.yml
```

---

### `disform drift`

Compare the state file against live Discord and report resources that were renamed or deleted outside of disform.

```sh
disform drift
```

---

### `disform refresh`

Re-sync the state file to live Discord by name-matching. Useful when a resource was deleted and recreated outside disform (new Discord ID, same name).

```sh
disform refresh
```

---

### `disform show`

Pretty-print the current live server state — settings, roles by hierarchy, categories, and channels.

```sh
disform show
disform show --server-id 123456789012345678
```

---

### `disform fmt`

Normalize `disform.yml` in place: sorts keys alphabetically, uppercases hex colors.

```sh
disform fmt           # write in place
disform fmt --check   # exit 1 if file would change
```

---

### `disform context`

Print LLM-ready documentation about the config format and CLI — paste into Claude, ChatGPT, or any assistant to give it full context about disform.

```sh
disform context
disform context | pbcopy   # copy to clipboard (macOS)
```

---

### `disform version`

Print version, commit hash, and build date.

```sh
disform version
```

---

## Config Reference

```yaml
server_id: "123456789012345678"   # required

settings: ...    # optional — guild-level settings
roles: ...       # optional — role definitions
categories: ...  # optional — categories and their channels
channels: ...    # optional — top-level channels (no parent category)
```

---

### `settings`

Guild-level settings. Omit the section entirely to leave these unmanaged.

```yaml
settings:
  verification_level: none              # none | low | medium | high | very_high
  explicit_content_filter: disabled     # disabled | members_without_roles | all_members
  default_message_notifications: all_messages  # all_messages | only_mentions
  afk_timeout: 300                      # 0 | 60 | 300 | 900 | 1800 | 3600 (seconds)
  afk_channel: voice-afk               # display name of an existing voice channel
  system_channel: arrivals             # display name of an existing text channel
```

---

### `roles`

Map of roles. Keys are stable internal identifiers used in the state file. The optional `name` field is the Discord display name members see.

```yaml
roles:
  "@everyone":                    # base server permissions; cannot be created or deleted
    permissions: [view_channel, send_messages]

  admin:                          # stable key — never change this
    name: "Admin"                 # Discord display name (defaults to key if omitted)
    color: "#E74C3C"             # hex, "#RRGGBB"
    hoist: true                  # show separately in member list
    mentionable: true            # allow @role mentions
    permissions:
      - administrator
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `name` | string | *(key)* | Discord display name |
| `color` | string | none | Hex color `"#RRGGBB"` |
| `hoist` | bool | false | Show separately in member list |
| `mentionable` | bool | false | Allow @role mentions |
| `permissions` | []string | [] | Guild-level permission names |

---

### `categories`

Map of categories. Each category contains its channels.

```yaml
categories:
  general:                        # stable key
    name: "General"              # Discord display name (defaults to key if omitted)
    position: 0                  # sort order in channel list
    channels:
      welcome:                   # stable key
        name: "welcome"
        type: text
        topic: "Welcome!"
```

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Discord display name (defaults to key) |
| `position` | int | Sort order (lower = higher in list) |
| `channels` | map | Channels nested under this category |

---

### `channels`

Top-level channels not inside a category. Same fields as category channels.

```yaml
channels:
  rules:
    type: text
    topic: "Read before posting"
```

**All channel fields:**

```yaml
some-channel:
  name: "display name"      # optional; key used if omitted
  type: text                # see Channel Types
  topic: "description"      # text/announcement/forum channels
  nsfw: false
  slowmode: 30              # seconds between messages (0 = off)
  bitrate: 64000            # voice channels only, bits per second
  user_limit: 10            # voice channels only, 0 = unlimited
  permissions:
    "@everyone":
      allow: [view_channel]
      deny: [send_messages]
    moderator:
      allow: [send_messages, manage_messages]
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `name` | string | *(key)* | Discord display name |
| `type` | string | required | Channel type (see below) |
| `topic` | string | "" | Channel description |
| `nsfw` | bool | false | Age-restricted |
| `slowmode` | int | 0 | Seconds between messages |
| `bitrate` | int | 0 | Voice bitrate in bps |
| `user_limit` | int | 0 | Max voice users (0 = unlimited) |
| `permissions` | map | {} | Per-role permission overwrites |

---

### Stable keys vs display names

The map key for any role, category, or channel is a **stable internal handle**. It is stored in the state file and used to track the resource — including across renames.

The optional `name:` field is the **Discord display name** your members see.

```yaml
categories:
  general:              # ← stable key: do not change this
    name: "General"     # ← display name: change freely, no history lost
    channels:
      welcome:
        name: "welcome"
```

Changing `name:` issues a `PATCH` to Discord with the new display name — no channel history is lost.  
Changing the YAML key would look like a delete + create to disform, destroying channel history.

---

### Permission overwrites

Each key under `permissions` is a role key defined in `roles:` or the special `"@everyone"`.

```yaml
permissions:
  "@everyone":
    deny: [send_messages]
  moderator:
    allow: [send_messages, manage_messages]
  admin:
    allow: [administrator]
```

For new channels, overwrites are sent in the creation request. For existing channels, they are synced via the Discord channel permissions API.

---

## Permissions

Use these names anywhere a permission list is expected.

### General

| Name | Description |
|------|-------------|
| `administrator` | All permissions; bypasses channel overwrites |
| `view_channel` | See channels and read messages |
| `manage_channels` | Create, edit, delete channels |
| `manage_guild` | Edit server settings |
| `manage_roles` | Create and manage roles |
| `manage_webhooks` | Create and manage webhooks |
| `manage_nicknames` | Change other members' nicknames |
| `change_nickname` | Change your own nickname |
| `kick_members` | Remove members from the server |
| `ban_members` | Ban members from the server |
| `moderate_members` | Timeout members |
| `view_audit_log` | View the audit log |
| `view_guild_insights` | View server insights |
| `create_instant_invite` | Create invite links |

### Text channels

| Name | Description |
|------|-------------|
| `send_messages` | Send messages |
| `send_messages_in_threads` | Reply in threads |
| `send_tts_messages` | Send text-to-speech messages |
| `read_message_history` | Read past messages |
| `manage_messages` | Delete and pin messages |
| `manage_threads` | Archive and delete threads |
| `create_public_threads` | Start public threads |
| `create_private_threads` | Start private threads |
| `embed_links` | Links show as rich embeds |
| `attach_files` | Upload files and media |
| `add_reactions` | Add emoji reactions |
| `use_external_emojis` | Use emojis from other servers |
| `use_external_stickers` | Use stickers from other servers |
| `use_slash_commands` | Use application commands |
| `mention_everyone` | Use @everyone and @here |

### Voice channels

| Name | Description |
|------|-------------|
| `connect` | Join voice channels |
| `speak` | Talk in voice channels |
| `stream` | Go live / screenshare |
| `use_vad` | Use voice activity detection |
| `use_embedded_activities` | Use Activities (games) in voice |
| `priority_speaker` | Reduce others' volume when speaking |
| `mute_members` | Server-mute other members |
| `deafen_members` | Server-deafen other members |
| `move_members` | Move members between voice channels |
| `request_to_speak` | Request to speak in Stage channels |

---

## Channel Types

| Config value | Discord type | Description |
|---|---|---|
| `text` | 0 | Standard text channel |
| `voice` | 2 | Voice / video channel |
| `announcement` | 5 | Announcement channel (formerly News) |
| `stage` | 13 | Stage channel for audio broadcasts |
| `forum` | 15 | Forum-style threaded channel |

---

## State File

`disform.state.json` maps config keys to Discord IDs. It is managed automatically — do not edit it by hand. Commit it alongside `disform.yml`.

```json
{
  "version": 1,
  "server_id": "123456789012345678",
  "roles": {
    "admin": { "discord_id": "987654321098765432" }
  },
  "categories": {
    "general": { "discord_id": "111111111111111111" }
  },
  "channels": {
    "general/welcome": { "discord_id": "333333333333333333" },
    "/rules": { "discord_id": "444444444444444444" }
  }
}
```

**Channel key format:**
- `"CategoryKey/channelKey"` — channel inside a category
- `"/channelKey"` — top-level channel (empty category prefix)

The state file enables disform to:
- **Rename without recreating** — same key means same resource, even if the display name changed
- **Detect drift** — resource in state but gone from Discord → re-create on next apply
- **Clean up properly** — resource removed from config → delete from Discord

---

## Environment & Auth

### Token priority (highest to lowest)

1. `--token` CLI flag
2. `DISCORD_TOKEN` environment variable
3. `DISCORD_TOKEN` in `.env` file in the current directory

### `.env` file

disform auto-loads a `.env` file on startup. No third-party library — plain `KEY=VALUE` parsing:

```sh
# .env
DISCORD_TOKEN=your-bot-token-here
```

Supports `export KEY=VALUE`, single- or double-quoted values, and `#` comments. Variables already present in the shell environment take precedence.

### `NO_COLOR`

Set `NO_COLOR` to any value to disable ANSI color output:

```sh
NO_COLOR=1 disform plan
```

---

## CI/CD Integration

### Apply on push to main

```yaml
# .github/workflows/discord.yml
name: Discord sync

on:
  push:
    branches: [main]
    paths: [disform.yml, disform.state.json]

jobs:
  apply:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: "1.21"

      - name: Install disform
        run: go install github.com/joshdholtz/disform@latest

      - name: Apply
        env:
          DISCORD_TOKEN: ${{ secrets.DISCORD_TOKEN }}
        run: disform apply --auto-approve
```

### Plan on pull requests

```yaml
on:
  pull_request:
    paths: [disform.yml]

jobs:
  plan:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: "1.21"

      - name: Install disform
        run: go install github.com/joshdholtz/disform@latest

      - name: Plan
        env:
          DISCORD_TOKEN: ${{ secrets.DISCORD_TOKEN }}
        run: disform plan --detailed-exitcode
        # exit 0 = no changes, exit 2 = changes, exit 1 = error
```

### Validate and format check

```yaml
- name: Validate config
  run: disform validate

- name: Check formatting
  run: disform fmt --check
```

---

## Bot Setup

1. Go to [discord.com/developers/applications](https://discord.com/developers/applications)
2. Create a new application → **Bot** tab → copy the token
3. Under **OAuth2 → URL Generator**, select scope `bot` with permissions:
   - Manage Roles
   - Manage Channels
4. Open the generated URL and invite the bot to your server

> **Important:** The bot's role must be positioned **above** any roles it manages in your server's role hierarchy. Discord enforces this at the API level.

---

## Contributing

```sh
git clone https://github.com/joshdholtz/disform
cd disform
go test ./...
go test -race ./...
go vet ./...
gofmt -w .
go build -o disform .
```

Run the linter (requires [golangci-lint](https://golangci-lint.run/)):

```sh
golangci-lint run
```

Pull requests welcome. Please run `gofmt` and `go vet` before submitting.

---

## License

MIT
