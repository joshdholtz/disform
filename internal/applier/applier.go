package applier

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/joshholtz/disform/internal/config"
	"github.com/joshholtz/disform/internal/discord"
	"github.com/joshholtz/disform/internal/planner"
	"github.com/joshholtz/disform/internal/state"
)

// Applier executes a plan against the Discord API and updates state.
type Applier struct {
	client discord.Client
	state  *state.State
	config *config.Config
}

// NewApplier creates an Applier with the given Discord client, state, and config.
func NewApplier(client discord.Client, st *state.State, cfg *config.Config) *Applier {
	return &Applier{
		client: client,
		state:  st,
		config: cfg,
	}
}

// Apply executes all actions in the plan in the correct order:
//   - For creates/updates: roles → categories → channels
//   - For deletes: channels → categories → roles
func (a *Applier) Apply(plan *planner.Plan) error {
	// Separate actions by type for ordered execution.
	var creates, updates, deletes []planner.Action
	for _, action := range plan.Actions {
		switch action.Type {
		case planner.ActionCreate:
			creates = append(creates, action)
		case planner.ActionUpdate:
			updates = append(updates, action)
		case planner.ActionDelete:
			deletes = append(deletes, action)
		}
	}

	// Process creates: roles → categories → channels
	for _, group := range [][]planner.Action{creates, updates} {
		if err := a.applyGroup(group, []planner.ResourceType{
			planner.ResourceRole,
			planner.ResourceCategory,
			planner.ResourceChannel,
		}); err != nil {
			return err
		}
	}

	// Process deletes: channels → categories → roles
	if err := a.applyGroup(deletes, []planner.ResourceType{
		planner.ResourceChannel,
		planner.ResourceCategory,
		planner.ResourceRole,
	}); err != nil {
		return err
	}

	return nil
}

// applyGroup applies a slice of actions in the given resource type order.
func (a *Applier) applyGroup(actions []planner.Action, order []planner.ResourceType) error {
	for _, rt := range order {
		for _, action := range actions {
			if action.ResourceType != rt {
				continue
			}
			if err := a.ApplyAction(action); err != nil {
				return fmt.Errorf("applying %s %s %q: %w", action.Type, action.ResourceType, action.Name, err)
			}
		}
	}
	return nil
}

// ApplyAction executes a single action and updates state accordingly.
func (a *Applier) ApplyAction(action planner.Action) error {
	switch action.Type {
	case planner.ActionCreate:
		switch action.ResourceType {
		case planner.ResourceRole:
			return a.applyCreateRole(action)
		case planner.ResourceCategory:
			return a.applyCreateCategory(action)
		case planner.ResourceChannel:
			return a.applyCreateChannel(action)
		}
	case planner.ActionUpdate:
		switch action.ResourceType {
		case planner.ResourceRole:
			return a.applyUpdateRole(action)
		case planner.ResourceCategory:
			return a.applyUpdateCategory(action)
		case planner.ResourceChannel:
			return a.applyUpdateChannel(action)
		}
	case planner.ActionDelete:
		switch action.ResourceType {
		case planner.ResourceRole:
			return a.applyDeleteRole(action)
		case planner.ResourceCategory:
			return a.applyDeleteCategory(action)
		case planner.ResourceChannel:
			return a.applyDeleteChannel(action)
		}
	}
	return fmt.Errorf("unknown action type %q or resource type %q", action.Type, action.ResourceType)
}

// applyCreateRole creates a role on Discord and records its ID in state.
func (a *Applier) applyCreateRole(action planner.Action) error {
	roleCfg, ok := a.config.Roles[action.Name]
	if !ok {
		return fmt.Errorf("role %q not found in config", action.Name)
	}

	color := 0
	if roleCfg.Color != "" {
		var err error
		color, err = config.ColorToInt(roleCfg.Color)
		if err != nil {
			return fmt.Errorf("invalid color for role %q: %w", action.Name, err)
		}
	}

	perms, err := config.PermissionsToInt(roleCfg.Permissions)
	if err != nil {
		return fmt.Errorf("invalid permissions for role %q: %w", action.Name, err)
	}

	params := discord.CreateRoleParams{
		Name:        action.Name,
		Permissions: strconv.FormatInt(perms, 10),
		Color:       color,
		Hoist:       roleCfg.Hoist,
		Mentionable: roleCfg.Mentionable,
	}

	role, err := a.client.CreateRole(a.config.ServerID, params)
	if err != nil {
		return fmt.Errorf("creating role %q: %w", action.Name, err)
	}

	a.state.SetRole(action.Name, role.ID)
	return nil
}

// applyUpdateRole updates an existing role on Discord.
func (a *Applier) applyUpdateRole(action planner.Action) error {
	params := discord.UpdateRoleParams{}

	for _, change := range action.Changes {
		switch change.Field {
		case "color":
			color, err := config.ColorToInt(change.NewValue)
			if err != nil {
				return fmt.Errorf("invalid color %q: %w", change.NewValue, err)
			}
			params.Color = &color
		case "hoist":
			hoist := change.NewValue == "true"
			params.Hoist = &hoist
		case "mentionable":
			mentionable := change.NewValue == "true"
			params.Mentionable = &mentionable
		case "permissions":
			params.Permissions = &change.NewValue
		}
	}

	_, err := a.client.UpdateRole(a.config.ServerID, action.DiscordID, params)
	if err != nil {
		return fmt.Errorf("updating role %q: %w", action.Name, err)
	}
	return nil
}

// applyDeleteRole deletes a role from Discord and removes it from state.
func (a *Applier) applyDeleteRole(action planner.Action) error {
	if err := a.client.DeleteRole(a.config.ServerID, action.DiscordID); err != nil {
		return fmt.Errorf("deleting role %q: %w", action.Name, err)
	}
	a.state.DeleteRole(action.Name)
	return nil
}

// applyCreateCategory creates a category channel on Discord and records it in state.
func (a *Applier) applyCreateCategory(action planner.Action) error {
	catCfg, ok := a.config.Categories[action.Name]
	if !ok {
		return fmt.Errorf("category %q not found in config", action.Name)
	}

	params := discord.CreateChannelParams{
		Name:     action.Name,
		Type:     discord.ChannelTypeGuildCategory,
		Position: catCfg.Position,
	}

	ch, err := a.client.CreateChannel(a.config.ServerID, params)
	if err != nil {
		return fmt.Errorf("creating category %q: %w", action.Name, err)
	}

	a.state.SetCategory(action.Name, ch.ID)
	return nil
}

// applyUpdateCategory updates an existing category channel on Discord.
func (a *Applier) applyUpdateCategory(action planner.Action) error {
	params := discord.UpdateChannelParams{}

	for _, change := range action.Changes {
		switch change.Field {
		case "position":
			pos, err := strconv.Atoi(change.NewValue)
			if err != nil {
				return fmt.Errorf("invalid position %q: %w", change.NewValue, err)
			}
			params.Position = &pos
		}
	}

	_, err := a.client.UpdateChannel(action.DiscordID, params)
	if err != nil {
		return fmt.Errorf("updating category %q: %w", action.Name, err)
	}
	return nil
}

// applyDeleteCategory deletes a category channel from Discord and removes it from state.
func (a *Applier) applyDeleteCategory(action planner.Action) error {
	if err := a.client.DeleteChannel(action.DiscordID); err != nil {
		return fmt.Errorf("deleting category %q: %w", action.Name, err)
	}
	a.state.DeleteCategory(action.Name)
	return nil
}

// applyCreateChannel creates a channel on Discord with permission overwrites, and records it in state.
func (a *Applier) applyCreateChannel(action planner.Action) error {
	catName, chanName, err := parseChannelName(action.Name)
	if err != nil {
		return err
	}

	cat, ok := a.config.Categories[catName]
	if !ok {
		return fmt.Errorf("category %q not found in config", catName)
	}

	chanCfg, ok := cat.Channels[chanName]
	if !ok {
		return fmt.Errorf("channel %q not found in category %q", chanName, catName)
	}

	parentID, _ := a.state.GetCategoryID(catName)

	chanType := channelTypeToInt(chanCfg.Type)

	overwrites, err := a.resolvePermissionOverwrites(chanCfg.Permissions)
	if err != nil {
		return fmt.Errorf("resolving permission overwrites for %q: %w", action.Name, err)
	}

	params := discord.CreateChannelParams{
		Name:                 chanName,
		Type:                 chanType,
		Topic:                chanCfg.Topic,
		NSFW:                 chanCfg.NSFW,
		RateLimitPerUser:     chanCfg.SlowMode,
		Bitrate:              chanCfg.Bitrate,
		UserLimit:            chanCfg.UserLimit,
		ParentID:             parentID,
		PermissionOverwrites: overwrites,
	}

	ch, err := a.client.CreateChannel(a.config.ServerID, params)
	if err != nil {
		return fmt.Errorf("creating channel %q: %w", action.Name, err)
	}

	a.state.SetChannel(catName, chanName, ch.ID)
	return nil
}

// applyUpdateChannel updates an existing channel on Discord with only changed fields.
func (a *Applier) applyUpdateChannel(action planner.Action) error {
	catName, chanName, err := parseChannelName(action.Name)
	if err != nil {
		return err
	}

	params := discord.UpdateChannelParams{}

	for _, change := range action.Changes {
		switch change.Field {
		case "topic":
			v := change.NewValue
			params.Topic = &v
		case "nsfw":
			v := change.NewValue == "true"
			params.NSFW = &v
		case "slowmode":
			v, err := strconv.Atoi(change.NewValue)
			if err != nil {
				return fmt.Errorf("invalid slowmode %q: %w", change.NewValue, err)
			}
			params.RateLimitPerUser = &v
		case "bitrate":
			v, err := strconv.Atoi(change.NewValue)
			if err != nil {
				return fmt.Errorf("invalid bitrate %q: %w", change.NewValue, err)
			}
			params.Bitrate = &v
		case "user_limit":
			v, err := strconv.Atoi(change.NewValue)
			if err != nil {
				return fmt.Errorf("invalid user_limit %q: %w", change.NewValue, err)
			}
			params.UserLimit = &v
		case "type":
			v := channelTypeToInt(change.NewValue)
			params.Type = &v
		}
	}

	_, err = a.client.UpdateChannel(action.DiscordID, params)
	if err != nil {
		return fmt.Errorf("updating channel %q: %w", action.Name, err)
	}

	// Sync permission overwrites.
	cat, catOk := a.config.Categories[catName]
	if catOk {
		if chanCfg, chanOk := cat.Channels[chanName]; chanOk {
			overwrites, err := a.resolvePermissionOverwrites(chanCfg.Permissions)
			if err != nil {
				return fmt.Errorf("resolving permissions for channel %q: %w", action.Name, err)
			}
			for _, ow := range overwrites {
				permParams := discord.EditChannelPermissionsParams{
					Allow: ow.Allow,
					Deny:  ow.Deny,
					Type:  ow.Type,
				}
				if err := a.client.EditChannelPermissions(action.DiscordID, ow.ID, permParams); err != nil {
					return fmt.Errorf("syncing permissions for channel %q overwrite %q: %w", action.Name, ow.ID, err)
				}
			}
		}
	}

	return nil
}

// applyDeleteChannel deletes a channel from Discord and removes it from state.
func (a *Applier) applyDeleteChannel(action planner.Action) error {
	catName, chanName, err := parseChannelName(action.Name)
	if err != nil {
		return err
	}

	if err := a.client.DeleteChannel(action.DiscordID); err != nil {
		return fmt.Errorf("deleting channel %q: %w", action.Name, err)
	}

	a.state.DeleteChannel(catName, chanName)
	return nil
}

// resolvePermissionOverwrites converts config permission overwrites to Discord API format.
// "@everyone" resolves to the server/guild ID; other names look up in state.Roles.
func (a *Applier) resolvePermissionOverwrites(perms map[string]config.PermissionOverwriteConfig) ([]discord.PermissionOverwrite, error) {
	var overwrites []discord.PermissionOverwrite

	for target, overwrite := range perms {
		allow, err := config.PermissionsToInt(overwrite.Allow)
		if err != nil {
			return nil, fmt.Errorf("resolving allow permissions for %q: %w", target, err)
		}
		deny, err := config.PermissionsToInt(overwrite.Deny)
		if err != nil {
			return nil, fmt.Errorf("resolving deny permissions for %q: %w", target, err)
		}

		var targetID string
		var overwriteType int

		if target == "@everyone" {
			targetID = a.config.ServerID
			overwriteType = discord.OverwriteTypeRole
		} else {
			// Try to look up as a role name in state.
			roleID, ok := a.state.GetRoleID(target)
			if !ok {
				return nil, fmt.Errorf("role %q not found in state for permission overwrite", target)
			}
			targetID = roleID
			overwriteType = discord.OverwriteTypeRole
		}

		overwrites = append(overwrites, discord.PermissionOverwrite{
			ID:    targetID,
			Type:  overwriteType,
			Allow: strconv.FormatInt(allow, 10),
			Deny:  strconv.FormatInt(deny, 10),
		})
	}

	return overwrites, nil
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

// parseChannelName splits "CategoryName/channel-name" into category and channel.
func parseChannelName(name string) (string, string, error) {
	idx := strings.Index(name, "/")
	if idx < 0 {
		return "", "", fmt.Errorf("invalid channel name %q: missing '/'", name)
	}
	return name[:idx], name[idx+1:], nil
}
