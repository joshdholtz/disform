package planner

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/joshholtz/disform/internal/config"
	"github.com/joshholtz/disform/internal/discord"
	"github.com/joshholtz/disform/internal/state"
)

// ActionType represents what operation will be performed.
type ActionType string

const (
	ActionCreate ActionType = "create"
	ActionUpdate ActionType = "update"
	ActionDelete ActionType = "delete"
)

// ResourceType identifies what kind of Discord resource the action applies to.
type ResourceType string

const (
	ResourceRole     ResourceType = "role"
	ResourceCategory ResourceType = "category"
	ResourceChannel  ResourceType = "channel"
	ResourceSettings ResourceType = "settings"
)

const everyoneRoleName = "@everyone"

// FieldChange records a single field change for an update action.
type FieldChange struct {
	Field    string
	OldValue string
	NewValue string
}

// Action represents a single planned change to a Discord resource.
type Action struct {
	Type         ActionType
	ResourceType ResourceType
	Name         string        // logical name (e.g. "admin" or "General/welcome")
	DiscordID    string        // empty for creates
	Changes      []FieldChange // populated for updates
}

// Plan holds all planned actions and summary counts.
type Plan struct {
	Actions  []Action
	ToCreate int
	ToUpdate int
	ToDelete int
}

// HasChanges returns true if there are any actions in the plan.
func (p *Plan) HasChanges() bool {
	return len(p.Actions) > 0
}

// Summary returns a human-readable summary of the plan.
func (p *Plan) Summary() string {
	return fmt.Sprintf("Plan: %d to add, %d to change, %d to destroy.", p.ToCreate, p.ToUpdate, p.ToDelete)
}

// LiveState holds the current state of Discord resources, keyed by Discord ID.
type LiveState struct {
	Guild      *discord.Guild
	Roles      map[string]*discord.Role    // keyed by discord ID
	Categories map[string]*discord.Channel // keyed by discord ID
	Channels   map[string]*discord.Channel // keyed by discord ID
}

// Planner computes a diff between desired config and current state.
type Planner struct {
	config    *config.Config
	state     *state.State
	liveState *LiveState
}

// NewPlanner creates a Planner with the given config, state, and live Discord state.
func NewPlanner(cfg *config.Config, st *state.State, live *LiveState) *Planner {
	return &Planner{
		config:    cfg,
		state:     st,
		liveState: live,
	}
}

// Plan computes the set of actions needed to bring Discord in line with the config.
func (p *Planner) Plan() (*Plan, error) {
	plan := &Plan{}

	if err := p.planRoles(plan); err != nil {
		return nil, fmt.Errorf("planning roles: %w", err)
	}
	if err := p.planCategories(plan); err != nil {
		return nil, fmt.Errorf("planning categories: %w", err)
	}
	if err := p.planChannels(plan); err != nil {
		return nil, fmt.Errorf("planning channels: %w", err)
	}
	if err := p.planTopLevelChannels(plan); err != nil {
		return nil, fmt.Errorf("planning top-level channels: %w", err)
	}
	if err := p.planSettings(plan); err != nil {
		return nil, fmt.Errorf("planning settings: %w", err)
	}

	return plan, nil
}

func (p *Planner) addAction(plan *Plan, action Action) {
	plan.Actions = append(plan.Actions, action)
	switch action.Type {
	case ActionCreate:
		plan.ToCreate++
	case ActionUpdate:
		plan.ToUpdate++
	case ActionDelete:
		plan.ToDelete++
	}
}

// planRoles computes role-level create/update/delete actions.
func (p *Planner) planRoles(plan *Plan) error {
	// Sort role names for deterministic output.
	roleNames := sortedKeys(p.config.Roles)

	for _, name := range roleNames {
		roleCfg := p.config.Roles[name]

		// Handle @everyone specially.
		if name == everyoneRoleName {
			// Find the live @everyone role (ID == guild ID).
			liveRole, inLive := p.liveState.Roles[p.config.ServerID]
			if !inLive {
				continue
			}
			// Compare only permissions for @everyone.
			wantPerms, err := config.PermissionsToInt(roleCfg.Permissions)
			if err != nil {
				return fmt.Errorf("comparing @everyone role permissions: %w", err)
			}
			wantPermsStr := strconv.FormatInt(wantPerms, 10)
			if wantPermsStr != liveRole.Permissions {
				p.addAction(plan, Action{
					Type:         ActionUpdate,
					ResourceType: ResourceRole,
					Name:         everyoneRoleName,
					DiscordID:    p.config.ServerID,
					Changes: []FieldChange{
						{Field: "permissions", OldValue: liveRole.Permissions, NewValue: wantPermsStr},
					},
				})
			}
			continue
		}

		discordID, inState := p.state.GetRoleID(name)

		if !inState {
			// Not in state → create.
			p.addAction(plan, Action{
				Type:         ActionCreate,
				ResourceType: ResourceRole,
				Name:         name,
			})
			continue
		}

		// In state — check if it still exists in live Discord.
		liveRole, inLive := p.liveState.Roles[discordID]
		if !inLive {
			// Externally deleted → re-create.
			p.addAction(plan, Action{
				Type:         ActionCreate,
				ResourceType: ResourceRole,
				Name:         name,
			})
			continue
		}

		// Compare fields.
		changes, err := compareRole(name, roleCfg, liveRole)
		if err != nil {
			return fmt.Errorf("comparing role %q: %w", name, err)
		}
		if len(changes) > 0 {
			p.addAction(plan, Action{
				Type:         ActionUpdate,
				ResourceType: ResourceRole,
				Name:         name,
				DiscordID:    discordID,
				Changes:      changes,
			})
		}
	}

	// Roles in state but not in config → delete.
	stateRoleNames := sortedStringKeys(p.state.Roles)
	for _, name := range stateRoleNames {
		if name == everyoneRoleName {
			continue
		}
		if _, ok := p.config.Roles[name]; !ok {
			discordID, _ := p.state.GetRoleID(name)
			p.addAction(plan, Action{
				Type:         ActionDelete,
				ResourceType: ResourceRole,
				Name:         name,
				DiscordID:    discordID,
			})
		}
	}

	return nil
}

// planCategories computes category-level create/update/delete actions.
func (p *Planner) planCategories(plan *Plan) error {
	catNames := sortedKeys(p.config.Categories)

	for _, name := range catNames {
		catCfg := p.config.Categories[name]
		discordID, inState := p.state.GetCategoryID(name)

		if !inState {
			p.addAction(plan, Action{
				Type:         ActionCreate,
				ResourceType: ResourceCategory,
				Name:         name,
			})
			continue
		}

		liveCategory, inLive := p.liveState.Categories[discordID]
		if !inLive {
			p.addAction(plan, Action{
				Type:         ActionCreate,
				ResourceType: ResourceCategory,
				Name:         name,
			})
			continue
		}

		changes := compareCategory(name, catCfg, liveCategory)
		if len(changes) > 0 {
			p.addAction(plan, Action{
				Type:         ActionUpdate,
				ResourceType: ResourceCategory,
				Name:         name,
				DiscordID:    discordID,
				Changes:      changes,
			})
		}
	}

	// Categories in state but not in config → delete.
	stateCatNames := sortedStringKeys(p.state.Categories)
	for _, name := range stateCatNames {
		if _, ok := p.config.Categories[name]; !ok {
			discordID, _ := p.state.GetCategoryID(name)
			p.addAction(plan, Action{
				Type:         ActionDelete,
				ResourceType: ResourceCategory,
				Name:         name,
				DiscordID:    discordID,
			})
		}
	}

	return nil
}

// planChannels computes channel-level create/update/delete actions.
func (p *Planner) planChannels(plan *Plan) error {
	catNames := sortedKeys(p.config.Categories)

	for _, catName := range catNames {
		cat := p.config.Categories[catName]
		chanNames := sortedKeys(cat.Channels)

		for _, chanName := range chanNames {
			chanCfg := cat.Channels[chanName]
			discordID, inState := p.state.GetChannelID(catName, chanName)

			if !inState {
				p.addAction(plan, Action{
					Type:         ActionCreate,
					ResourceType: ResourceChannel,
					Name:         state.ChannelKey(catName, chanName),
				})
				continue
			}

			liveChannel, inLive := p.liveState.Channels[discordID]
			if !inLive {
				p.addAction(plan, Action{
					Type:         ActionCreate,
					ResourceType: ResourceChannel,
					Name:         state.ChannelKey(catName, chanName),
				})
				continue
			}

			changes, err := compareChannel(chanCfg, liveChannel)
			if err != nil {
				return fmt.Errorf("comparing channel %q/%q: %w", catName, chanName, err)
			}
			if len(changes) > 0 {
				p.addAction(plan, Action{
					Type:         ActionUpdate,
					ResourceType: ResourceChannel,
					Name:         state.ChannelKey(catName, chanName),
					DiscordID:    discordID,
					Changes:      changes,
				})
			}
		}
	}

	// Channels in state but not in config → delete.
	chanKeys := sortedStringKeys(p.state.Channels)
	for _, key := range chanKeys {
		// Skip top-level channels (key starts with "/") — handled by planTopLevelChannels.
		if strings.HasPrefix(key, "/") {
			continue
		}

		catName, chanName, err := parseChannelKey(key)
		if err != nil {
			continue
		}

		cat, catInConfig := p.config.Categories[catName]
		if !catInConfig {
			discordID := p.state.Channels[key].DiscordID
			p.addAction(plan, Action{
				Type:         ActionDelete,
				ResourceType: ResourceChannel,
				Name:         key,
				DiscordID:    discordID,
			})
			continue
		}

		if _, chanInConfig := cat.Channels[chanName]; !chanInConfig {
			discordID := p.state.Channels[key].DiscordID
			p.addAction(plan, Action{
				Type:         ActionDelete,
				ResourceType: ResourceChannel,
				Name:         key,
				DiscordID:    discordID,
			})
		}
	}

	return nil
}

// planTopLevelChannels computes create/update/delete actions for top-level channels (no category).
func (p *Planner) planTopLevelChannels(plan *Plan) error {
	chanNames := sortedKeys(p.config.Channels)

	for _, chanName := range chanNames {
		chanCfg := p.config.Channels[chanName]
		discordID, inState := p.state.GetChannelID("", chanName)

		if !inState {
			p.addAction(plan, Action{
				Type:         ActionCreate,
				ResourceType: ResourceChannel,
				Name:         state.ChannelKey("", chanName),
			})
			continue
		}

		liveChannel, inLive := p.liveState.Channels[discordID]
		if !inLive {
			p.addAction(plan, Action{
				Type:         ActionCreate,
				ResourceType: ResourceChannel,
				Name:         state.ChannelKey("", chanName),
			})
			continue
		}

		changes, err := compareChannel(chanCfg, liveChannel)
		if err != nil {
			return fmt.Errorf("comparing top-level channel %q: %w", chanName, err)
		}
		if len(changes) > 0 {
			p.addAction(plan, Action{
				Type:         ActionUpdate,
				ResourceType: ResourceChannel,
				Name:         state.ChannelKey("", chanName),
				DiscordID:    discordID,
				Changes:      changes,
			})
		}
	}

	// Top-level channels in state (key starts with "/") but not in config → delete.
	chanKeys := sortedStringKeys(p.state.Channels)
	for _, key := range chanKeys {
		if !strings.HasPrefix(key, "/") {
			continue
		}
		chanName := key[1:] // strip leading "/"
		if _, inConfig := p.config.Channels[chanName]; !inConfig {
			discordID := p.state.Channels[key].DiscordID
			p.addAction(plan, Action{
				Type:         ActionDelete,
				ResourceType: ResourceChannel,
				Name:         key,
				DiscordID:    discordID,
			})
		}
	}

	return nil
}

// planSettings computes guild settings update action.
func (p *Planner) planSettings(plan *Plan) error {
	if p.config.Settings == nil {
		return nil
	}
	if p.liveState.Guild == nil {
		return nil
	}

	s := p.config.Settings
	live := p.liveState.Guild
	var changes []FieldChange

	if s.VerificationLevel != "" {
		wantVal, err := config.VerificationLevelToInt(s.VerificationLevel)
		if err != nil {
			return err
		}
		if wantVal != live.VerificationLevel {
			changes = append(changes, FieldChange{
				Field:    "verification_level",
				OldValue: strconv.Itoa(live.VerificationLevel),
				NewValue: s.VerificationLevel,
			})
		}
	}

	if s.ExplicitContentFilter != "" {
		wantVal, err := config.ContentFilterToInt(s.ExplicitContentFilter)
		if err != nil {
			return err
		}
		if wantVal != live.ExplicitContentFilter {
			changes = append(changes, FieldChange{
				Field:    "explicit_content_filter",
				OldValue: strconv.Itoa(live.ExplicitContentFilter),
				NewValue: s.ExplicitContentFilter,
			})
		}
	}

	if s.DefaultMessageNotifications != "" {
		wantVal, err := config.DefaultNotificationsToInt(s.DefaultMessageNotifications)
		if err != nil {
			return err
		}
		if wantVal != live.DefaultMessageNotifications {
			changes = append(changes, FieldChange{
				Field:    "default_message_notifications",
				OldValue: strconv.Itoa(live.DefaultMessageNotifications),
				NewValue: s.DefaultMessageNotifications,
			})
		}
	}

	if s.AFKTimeout != live.AFKTimeout {
		changes = append(changes, FieldChange{
			Field:    "afk_timeout",
			OldValue: strconv.Itoa(live.AFKTimeout),
			NewValue: strconv.Itoa(s.AFKTimeout),
		})
	}

	if s.AFKChannel != "" {
		wantID := p.resolveChannelNameToID(s.AFKChannel)
		liveID := ""
		if live.AFKChannelID != nil {
			liveID = *live.AFKChannelID
		}
		if wantID != liveID {
			changes = append(changes, FieldChange{
				Field:    "afk_channel",
				OldValue: liveID,
				NewValue: s.AFKChannel,
			})
		}
	}

	if s.SystemChannel != "" {
		wantID := p.resolveChannelNameToID(s.SystemChannel)
		liveID := ""
		if live.SystemChannelID != nil {
			liveID = *live.SystemChannelID
		}
		if wantID != liveID {
			changes = append(changes, FieldChange{
				Field:    "system_channel",
				OldValue: liveID,
				NewValue: s.SystemChannel,
			})
		}
	}

	if len(changes) > 0 {
		p.addAction(plan, Action{
			Type:         ActionUpdate,
			ResourceType: ResourceSettings,
			Name:         "server_settings",
			DiscordID:    p.config.ServerID,
			Changes:      changes,
		})
	}

	return nil
}

// resolveChannelNameToID looks up a channel name in state (top-level first, then categorized).
func (p *Planner) resolveChannelNameToID(chanName string) string {
	// Check top-level channels first.
	if id, ok := p.state.GetChannelID("", chanName); ok {
		return id
	}
	// Check categorized channels.
	for catName := range p.config.Categories {
		if id, ok := p.state.GetChannelID(catName, chanName); ok {
			return id
		}
	}
	return ""
}

// compareRole returns a list of field changes between the config and live role.
func compareRole(name string, cfg config.RoleConfig, live *discord.Role) ([]FieldChange, error) {
	var changes []FieldChange

	// Color
	if cfg.Color != "" {
		wantColor, err := config.ColorToInt(cfg.Color)
		if err != nil {
			return nil, fmt.Errorf("invalid color: %w", err)
		}
		if wantColor != live.Color {
			changes = append(changes, FieldChange{
				Field:    "color",
				OldValue: fmt.Sprintf("%d", live.Color),
				NewValue: cfg.Color,
			})
		}
	}

	// Hoist
	if cfg.Hoist != live.Hoist {
		changes = append(changes, FieldChange{
			Field:    "hoist",
			OldValue: strconv.FormatBool(live.Hoist),
			NewValue: strconv.FormatBool(cfg.Hoist),
		})
	}

	// Mentionable
	if cfg.Mentionable != live.Mentionable {
		changes = append(changes, FieldChange{
			Field:    "mentionable",
			OldValue: strconv.FormatBool(live.Mentionable),
			NewValue: strconv.FormatBool(cfg.Mentionable),
		})
	}

	// Permissions
	wantPerms, err := config.PermissionsToInt(cfg.Permissions)
	if err != nil {
		return nil, fmt.Errorf("converting permissions: %w", err)
	}
	wantPermsStr := strconv.FormatInt(wantPerms, 10)
	if wantPermsStr != live.Permissions {
		changes = append(changes, FieldChange{
			Field:    "permissions",
			OldValue: live.Permissions,
			NewValue: wantPermsStr,
		})
	}

	return changes, nil
}

// compareCategory returns field changes between the config and live category.
func compareCategory(name string, cfg config.CategoryConfig, live *discord.Channel) []FieldChange {
	var changes []FieldChange

	if cfg.Position != live.Position {
		changes = append(changes, FieldChange{
			Field:    "position",
			OldValue: strconv.Itoa(live.Position),
			NewValue: strconv.Itoa(cfg.Position),
		})
	}

	return changes
}

// compareChannel returns field changes between the config and live channel.
func compareChannel(cfg config.ChannelConfig, live *discord.Channel) ([]FieldChange, error) {
	var changes []FieldChange

	// Type
	if cfg.Type != "" {
		wantType := channelTypeToInt(cfg.Type)
		if wantType != live.Type {
			changes = append(changes, FieldChange{
				Field:    "type",
				OldValue: strconv.Itoa(live.Type),
				NewValue: cfg.Type,
			})
		}
	}

	// Topic
	liveTopic := ""
	if live.Topic != nil {
		liveTopic = *live.Topic
	}
	if cfg.Topic != liveTopic {
		changes = append(changes, FieldChange{
			Field:    "topic",
			OldValue: liveTopic,
			NewValue: cfg.Topic,
		})
	}

	// NSFW
	if cfg.NSFW != live.NSFW {
		changes = append(changes, FieldChange{
			Field:    "nsfw",
			OldValue: strconv.FormatBool(live.NSFW),
			NewValue: strconv.FormatBool(cfg.NSFW),
		})
	}

	// Slowmode
	if cfg.SlowMode != live.RateLimitPerUser {
		changes = append(changes, FieldChange{
			Field:    "slowmode",
			OldValue: strconv.Itoa(live.RateLimitPerUser),
			NewValue: strconv.Itoa(cfg.SlowMode),
		})
	}

	// Bitrate (only relevant for voice channels)
	if cfg.Type == "voice" || cfg.Type == "stage" {
		if cfg.Bitrate != 0 && cfg.Bitrate != live.Bitrate {
			changes = append(changes, FieldChange{
				Field:    "bitrate",
				OldValue: strconv.Itoa(live.Bitrate),
				NewValue: strconv.Itoa(cfg.Bitrate),
			})
		}
	}

	// UserLimit (only relevant for voice channels)
	if cfg.Type == "voice" || cfg.Type == "stage" {
		if cfg.UserLimit != live.UserLimit {
			changes = append(changes, FieldChange{
				Field:    "user_limit",
				OldValue: strconv.Itoa(live.UserLimit),
				NewValue: strconv.Itoa(cfg.UserLimit),
			})
		}
	}

	return changes, nil
}

// channelTypeToInt converts a config channel type string to Discord's integer type.
func channelTypeToInt(t string) int {
	switch t {
	case "text":
		return discord.ChannelTypeGuildText
	case "voice":
		return discord.ChannelTypeGuildVoice
	case "announcement":
		return discord.ChannelTypeGuildAnnouncement
	case "stage":
		return discord.ChannelTypeGuildStage
	case "forum":
		return discord.ChannelTypeGuildForum
	default:
		return discord.ChannelTypeGuildText
	}
}

// parseChannelKey splits a "CategoryName/channel-name" key into its two parts.
// For top-level channels the key is "/channel-name" which returns ("", "channel-name", nil).
func parseChannelKey(key string) (string, string, error) {
	idx := strings.Index(key, "/")
	if idx < 0 {
		return "", "", fmt.Errorf("invalid channel key %q: missing '/'", key)
	}
	return key[:idx], key[idx+1:], nil
}

// sortedKeys returns the sorted keys of any map with string keys.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// sortedStringKeys returns sorted keys of a map[string]T.
func sortedStringKeys[V any](m map[string]V) []string {
	return sortedKeys(m)
}
