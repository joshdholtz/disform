# disform

Terraform-like CLI for declarative Discord server management. Written in Go.

## Build & test

```sh
go build -o disform .
go test ./...
go test -race ./...
gofmt -w .
go vet ./...
```

Lint (requires golangci-lint):
```sh
golangci-lint run
```

## Architecture

```
cmd/            Cobra CLI commands — thin layer that wires packages together
  plan.go       Fetch live state → planner → print diff
  apply.go      plan → confirm → applier → save state
  destroy.go    Read state → delete all → clear state
  import_cmd.go Fetch live → write disform.yml + disform.state.json

internal/config/
  config.go     LoadConfig, Validate, ColorToInt, PermissionsToInt
                Config/RoleConfig/CategoryConfig/ChannelConfig types

internal/discord/
  types.go      Discord API types (Channel, Role, PermissionOverwrite, param structs)
  client.go     Client interface + HTTPClient (rate-limit retry on 429)

internal/state/
  state.go      Load/save disform.state.json; CRUD for roles/categories/channels
                Channel keys are "CategoryName/channel-name"

internal/planner/
  planner.go    Diff engine: config + state vs live Discord → []Action
                Handles create / update (with FieldChanges) / delete
                Detects external drift (in state, not in live → re-create)

internal/applier/
  applier.go    Executes Actions via discord.Client, mutates state in-place
                Apply order: roles → categories → channels
                Delete order: channels → categories → roles
```

## Key conventions

- **Discord Client is an interface** (`discord.Client`) — always inject it so tests can mock it.
- **State is mutated in-place** by the applier and saved at the end of `apply`. On partial failure the state is saved mid-run to preserve progress.
- **Map iteration is sorted** before any output or comparison so behavior is deterministic.
- **Permission bits are int64** — Discord permission values exceed int32 (bit 40 = moderate_members).
- **Channel key format**: `"CategoryName/channel-name"` (single slash, first slash is the split point).
- **`@everyone` role ID** equals the guild/server ID — hardcoded in the applier when resolving permission overwrites.
- **Color format**: config uses `"#RRGGBB"` strings; Discord API uses integers. `config.ColorToInt` converts.
- **No third-party libraries** except `cobra` and `yaml.v3`. HTTP, JSON, and testing all use stdlib.

## Testing patterns

- Discord client tests use `httptest.NewServer` — set `baseURL` via `NewHTTPClientWithBase`.
- Applier tests use a hand-rolled `mockClient` struct that implements `discord.Client`.
- State/config tests use `t.TempDir()` for file I/O.
- Table-driven tests with `t.Run` for variations on the same scenario.

## Config file (`disform.yml`)

```yaml
server_id: "123456789012345678"
roles:
  admin:
    color: "#FF0000"
    hoist: true
    permissions: [administrator]
categories:
  General:
    position: 0
    channels:
      welcome:
        type: text           # text | voice | announcement | forum | stage
        topic: "Welcome!"
        permissions:
          "@everyone":
            deny: [send_messages]
```

## State file (`disform.state.json`)

```json
{
  "version": 1,
  "server_id": "...",
  "roles":      { "admin":           { "discord_id": "..." } },
  "categories": { "General":         { "discord_id": "..." } },
  "channels":   { "General/welcome": { "discord_id": "..." } }
}
```

## Adding a new resource type

1. Add types to `internal/discord/types.go`
2. Add client methods to `discord.Client` interface and `HTTPClient`
3. Add config struct fields to `internal/config/config.go`
4. Add state tracking to `internal/state/state.go`
5. Add plan logic to `internal/planner/planner.go`
6. Add apply logic to `internal/applier/applier.go`
7. Add tests to each `_test.go` file
