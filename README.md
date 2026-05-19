# disform

[![CI](https://github.com/joshdholtz/disform/actions/workflows/ci.yml/badge.svg)](https://github.com/joshdholtz/disform/actions/workflows/ci.yml)

Declarative Discord server management. Define your server's channels, categories, roles, and permissions in a YAML file — then `plan` to preview changes and `apply` to make them real.

Think Terraform, but for Discord.

```
  + role "moderator" will be created
  ~ channel "General/announcements" will be updated
    topic: "Old topic" -> "Server announcements"
  - channel "Voice/afk" will be deleted

Plan: 1 to add, 1 to change, 1 to destroy.
```

---

## Install

**Requires Go 1.21+**

```sh
go install github.com/joshdholtz/disform@latest
```

Or download a binary from [Releases](https://github.com/joshdholtz/disform/releases).

---

## Quick start

**1. Create a Discord bot and get a token**

Go to the [Discord Developer Portal](https://discord.com/developers/applications), create an application, add a bot, and copy the token. The bot needs **Manage Channels** and **Manage Roles** permissions on your server.

**2. Set your token**

```sh
export DISCORD_TOKEN=your_bot_token_here
```

**3. Import an existing server** (or write a config from scratch)

```sh
disform import --server-id YOUR_SERVER_ID
# Writes disform.yml and disform.state.json
```

**4. Edit `disform.yml`** to declare the desired state of your server.

**5. Preview changes**

```sh
disform plan
```

**6. Apply**

```sh
disform apply
```

---

## Configuration

All server structure lives in `disform.yml`.

```yaml
server_id: "123456789012345678"

roles:
  admin:
    color: "#E74C3C"
    hoist: true
    mentionable: true
    permissions:
      - administrator

  moderator:
    color: "#E67E22"
    hoist: true
    permissions:
      - kick_members
      - manage_messages
      - manage_channels

  member:
    color: "#2ECC71"

categories:
  General:
    position: 0
    channels:
      welcome:
        type: text
        topic: "Read the rules before chatting."
        permissions:
          "@everyone":
            allow:
              - view_channel
              - read_message_history
            deny:
              - send_messages

      general-chat:
        type: text
        topic: "Talk about anything."

      announcements:
        type: announcement
        permissions:
          "@everyone":
            deny:
              - send_messages

  Voice:
    position: 1
    channels:
      general-voice:
        type: voice
        bitrate: 64000
        user_limit: 0

      music:
        type: voice
        bitrate: 128000
        user_limit: 10
```

### Roles

| Field | Type | Description |
|---|---|---|
| `color` | string | Hex color, e.g. `"#FF0000"` |
| `hoist` | bool | Show members separately in the sidebar |
| `mentionable` | bool | Allow anyone to @mention this role |
| `permissions` | list | Guild-level permissions (see [Permission names](#permission-names)) |

### Categories

| Field | Type | Description |
|---|---|---|
| `position` | int | Sort order in the channel list |
| `channels` | map | Channels nested under this category |

### Channels

| Field | Type | Description |
|---|---|---|
| `type` | string | `text`, `voice`, `announcement`, `forum`, `stage` |
| `topic` | string | Channel topic / description |
| `nsfw` | bool | Mark as age-restricted |
| `slowmode` | int | Seconds between messages (0 = off) |
| `bitrate` | int | Voice bitrate in bps (voice channels only) |
| `user_limit` | int | Max users in voice channel (0 = unlimited) |
| `permissions` | map | Per-role permission overwrites |

### Permission overwrites

Each key under `permissions` is either a role name defined in `roles:` or `"@everyone"`. Each entry has an `allow` list and a `deny` list.

```yaml
permissions:
  "@everyone":
    deny:
      - send_messages
  moderator:
    allow:
      - send_messages
      - manage_messages
```

### Permission names

<details>
<summary>Full list of supported permission names</summary>

| Name | Description |
|---|---|
| `administrator` | All permissions, bypasses overwrites |
| `manage_channels` | Create, edit, delete channels |
| `manage_guild` | Edit server settings |
| `manage_roles` | Create and manage roles |
| `manage_messages` | Delete and pin messages |
| `manage_webhooks` | Create and manage webhooks |
| `manage_emojis` | Add and remove emojis |
| `manage_threads` | Archive and delete threads |
| `manage_nicknames` | Change other members' nicknames |
| `kick_members` | Remove members from server |
| `ban_members` | Ban members from server |
| `moderate_members` | Timeout members |
| `view_channel` | See channels |
| `send_messages` | Send messages in text channels |
| `send_messages_in_threads` | Reply in threads |
| `send_tts_messages` | Send text-to-speech messages |
| `create_public_threads` | Create public threads |
| `create_private_threads` | Create private threads |
| `embed_links` | Links show as embeds |
| `attach_files` | Upload files |
| `add_reactions` | Add emoji reactions |
| `use_external_emojis` | Use emojis from other servers |
| `use_external_stickers` | Use stickers from other servers |
| `use_slash_commands` | Use application commands |
| `use_embedded_activities` | Use Activities in voice channels |
| `use_vad` | Use voice activity detection |
| `mention_everyone` | @everyone and @here |
| `read_message_history` | Read past messages |
| `view_audit_log` | View the audit log |
| `view_guild_insights` | View server insights |
| `priority_speaker` | Reduce others' volume when speaking |
| `stream` | Go live in voice channels |
| `connect` | Join voice channels |
| `speak` | Talk in voice channels |
| `mute_members` | Mute others in voice |
| `deafen_members` | Deafen others in voice |
| `move_members` | Move members between voice channels |
| `request_to_speak` | Request to speak in Stage channels |
| `change_nickname` | Change your own nickname |
| `create_instant_invite` | Create invite links |

</details>

---

## Commands

### `disform init`

Writes a starter `disform.yml` template.

```sh
disform init
disform init --server-id 123456789012345678
disform init --force   # overwrite existing file
```

### `disform plan`

Fetches the live Discord state and shows what would change. Does not modify anything.

```sh
disform plan
disform plan --json                  # machine-readable output for scripting
disform plan --detailed-exitcode     # exit 2 when changes exist (CI gate)
```

### `disform apply`

Runs `plan`, prompts for confirmation, then executes all changes. Acquires a lock on the state file to prevent concurrent runs. State is saved after each successful action so a partial failure is recoverable.

```sh
disform apply
disform apply --auto-approve                    # skip confirmation
disform apply --target role.admin               # apply only one resource
disform apply --target channel.General/welcome  # channel targets use Category/name
```

### `disform destroy`

Deletes every resource tracked in the state file. Prompts for confirmation. Clears the state file on success. Channels are always deleted before their parent categories.

```sh
disform destroy
disform destroy --auto-approve
```

### `disform import`

Reads the current Discord server state and generates both a `disform.yml` config and a `disform.state.json` state file. Use this to bring an existing server under disform management.

```sh
disform import --server-id 123456789012345678
disform import --server-id 123456789012345678 --output my-server.yml
```

### `disform validate`

Checks the config file for errors without connecting to Discord.

```sh
disform validate
disform validate --config staging.yml
```

### `disform drift`

Compares the state file against live Discord and reports resources that were renamed or deleted outside of disform.

```sh
disform drift
```

### `disform fmt`

Normalizes `disform.yml` in place — sorts keys alphabetically, uppercases hex colors.

```sh
disform fmt
disform fmt --check   # exit 1 if file would change (useful in CI)
```

### `disform version`

```sh
disform version
```

---

## Global flags

All commands accept these flags:

| Flag | Default | Description |
|---|---|---|
| `--config`, `-c` | `disform.yml` | Path to the config file |
| `--state`, `-s` | `disform.state.json` | Path to the state file |
| `--token`, `-t` | `$DISCORD_TOKEN` | Discord bot token |

---

## State file

disform tracks what it manages in `disform.state.json`. This file maps logical names in your config to Discord's internal IDs. Commit it alongside `disform.yml`.

```json
{
  "version": 1,
  "server_id": "123456789012345678",
  "roles": {
    "admin": { "discord_id": "987654321098765432" }
  },
  "categories": {
    "General": { "discord_id": "111111111111111111" }
  },
  "channels": {
    "General/welcome": { "discord_id": "222222222222222222" }
  }
}
```

The state file is how disform:
- **Renames** resources correctly (rename in config → update in Discord, not delete+recreate)
- **Detects drift** — if a resource is deleted outside of disform, the next `apply` re-creates it
- **Deletes cleanly** — resources removed from config are deleted from Discord

---

## Bot setup

1. Go to [discord.com/developers/applications](https://discord.com/developers/applications)
2. Create a new application → add a bot
3. Under **Bot**, copy the token
4. Under **OAuth2 → URL Generator**, select scopes `bot` and permissions:
   - Manage Roles
   - Manage Channels
5. Use the generated URL to invite the bot to your server

> **Note:** The bot's role must be positioned above any roles it manages in the server's role hierarchy.

---

## Contributing

```sh
git clone https://github.com/joshdholtz/disform
cd disform
go test ./...
go build -o disform .
```

Pull requests are welcome. Please run `gofmt -w .` and `go vet ./...` before submitting.

---

## License

MIT
