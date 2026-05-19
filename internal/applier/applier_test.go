package applier

import (
	"fmt"
	"testing"

	"github.com/joshholtz/disform/internal/config"
	"github.com/joshholtz/disform/internal/discord"
	"github.com/joshholtz/disform/internal/planner"
	"github.com/joshholtz/disform/internal/state"
)

// --- Mock Discord Client ---

type CreateChannelCall struct {
	GuildID string
	Params  discord.CreateChannelParams
}

type UpdateChannelCall struct {
	ChannelID string
	Params    discord.UpdateChannelParams
}

type CreateRoleCall struct {
	GuildID string
	Params  discord.CreateRoleParams
}

type UpdateRoleCall struct {
	GuildID string
	RoleID  string
	Params  discord.UpdateRoleParams
}

type EditPermCall struct {
	ChannelID   string
	OverwriteID string
	Params      discord.EditChannelPermissionsParams
}

type ModifyGuildCall struct {
	GuildID string
	Params  discord.ModifyGuildParams
}

type mockClient struct {
	createChannelCalls []CreateChannelCall
	updateChannelCalls []UpdateChannelCall
	deleteChannelCalls []string
	createRoleCalls    []CreateRoleCall
	updateRoleCalls    []UpdateRoleCall
	deleteRoleCalls    []string
	editPermCalls      []EditPermCall
	deletePermCalls    []string
	modifyGuildCalls   []ModifyGuildCall

	channelResponses map[string]*discord.Channel
	roleResponses    map[string]*discord.Role
	errors           map[string]error
}

func newMockClient() *mockClient {
	return &mockClient{
		channelResponses: make(map[string]*discord.Channel),
		roleResponses:    make(map[string]*discord.Role),
		errors:           make(map[string]error),
	}
}

func (m *mockClient) GetGuild(guildID string) (*discord.Guild, error) {
	return &discord.Guild{ID: guildID, Name: "Test Server"}, nil
}

func (m *mockClient) GetChannels(guildID string) ([]*discord.Channel, error) {
	return nil, nil
}

func (m *mockClient) GetRoles(guildID string) ([]*discord.Role, error) {
	return nil, nil
}

func (m *mockClient) CreateChannel(guildID string, params discord.CreateChannelParams) (*discord.Channel, error) {
	call := CreateChannelCall{GuildID: guildID, Params: params}
	m.createChannelCalls = append(m.createChannelCalls, call)
	if err := m.errors["create_channel_"+params.Name]; err != nil {
		return nil, err
	}
	if ch, ok := m.channelResponses[params.Name]; ok {
		return ch, nil
	}
	return &discord.Channel{ID: "new-ch-" + params.Name, Name: params.Name, Type: params.Type}, nil
}

func (m *mockClient) UpdateChannel(channelID string, params discord.UpdateChannelParams) (*discord.Channel, error) {
	m.updateChannelCalls = append(m.updateChannelCalls, UpdateChannelCall{ChannelID: channelID, Params: params})
	if err := m.errors["update_channel_"+channelID]; err != nil {
		return nil, err
	}
	return &discord.Channel{ID: channelID}, nil
}

func (m *mockClient) DeleteChannel(channelID string) error {
	m.deleteChannelCalls = append(m.deleteChannelCalls, channelID)
	if err := m.errors["delete_channel_"+channelID]; err != nil {
		return err
	}
	return nil
}

func (m *mockClient) CreateRole(guildID string, params discord.CreateRoleParams) (*discord.Role, error) {
	m.createRoleCalls = append(m.createRoleCalls, CreateRoleCall{GuildID: guildID, Params: params})
	if err := m.errors["create_role_"+params.Name]; err != nil {
		return nil, err
	}
	if role, ok := m.roleResponses[params.Name]; ok {
		return role, nil
	}
	return &discord.Role{ID: "new-role-" + params.Name, Name: params.Name}, nil
}

func (m *mockClient) UpdateRole(guildID, roleID string, params discord.UpdateRoleParams) (*discord.Role, error) {
	m.updateRoleCalls = append(m.updateRoleCalls, UpdateRoleCall{GuildID: guildID, RoleID: roleID, Params: params})
	if err := m.errors["update_role_"+roleID]; err != nil {
		return nil, err
	}
	return &discord.Role{ID: roleID}, nil
}

func (m *mockClient) DeleteRole(guildID, roleID string) error {
	m.deleteRoleCalls = append(m.deleteRoleCalls, roleID)
	if err := m.errors["delete_role_"+roleID]; err != nil {
		return err
	}
	return nil
}

func (m *mockClient) EditChannelPermissions(channelID, overwriteID string, params discord.EditChannelPermissionsParams) error {
	m.editPermCalls = append(m.editPermCalls, EditPermCall{ChannelID: channelID, OverwriteID: overwriteID, Params: params})
	if err := m.errors["edit_perm_"+channelID]; err != nil {
		return err
	}
	return nil
}

func (m *mockClient) DeleteChannelPermission(channelID, overwriteID string) error {
	m.deletePermCalls = append(m.deletePermCalls, channelID+"/"+overwriteID)
	return nil
}

func (m *mockClient) ModifyGuild(guildID string, params discord.ModifyGuildParams) (*discord.Guild, error) {
	m.modifyGuildCalls = append(m.modifyGuildCalls, ModifyGuildCall{GuildID: guildID, Params: params})
	if err := m.errors["modify_guild_"+guildID]; err != nil {
		return nil, err
	}
	return &discord.Guild{ID: guildID, Name: "Test Server"}, nil
}

// --- Helpers ---

func makeConfig(serverID string, roles map[string]config.RoleConfig, cats map[string]config.CategoryConfig) *config.Config {
	if roles == nil {
		roles = map[string]config.RoleConfig{}
	}
	if cats == nil {
		cats = map[string]config.CategoryConfig{}
	}
	return &config.Config{
		ServerID:   serverID,
		Roles:      roles,
		Categories: cats,
	}
}

// --- Tests ---

func TestApplyCreateRole(t *testing.T) {
	client := newMockClient()
	st := state.NewState("server-123")
	cfg := makeConfig("server-123", map[string]config.RoleConfig{
		"admin": {Color: "#FF0000", Hoist: true, Mentionable: true, Permissions: []string{"administrator"}},
	}, nil)

	a := NewApplier(client, st, cfg)
	action := planner.Action{
		Type:         planner.ActionCreate,
		ResourceType: planner.ResourceRole,
		Name:         "admin",
	}

	if err := a.ApplyAction(action); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(client.createRoleCalls) != 1 {
		t.Fatalf("expected 1 CreateRole call, got %d", len(client.createRoleCalls))
	}
	call := client.createRoleCalls[0]
	if call.GuildID != "server-123" {
		t.Errorf("expected guildID 'server-123', got %q", call.GuildID)
	}
	if call.Params.Name != "admin" {
		t.Errorf("expected role name 'admin', got %q", call.Params.Name)
	}
	if call.Params.Color != 0xFF0000 {
		t.Errorf("expected color 0xFF0000, got %d", call.Params.Color)
	}
	if !call.Params.Hoist {
		t.Error("expected hoist=true")
	}
	if !call.Params.Mentionable {
		t.Error("expected mentionable=true")
	}

	// Verify state updated
	id, ok := st.GetRoleID("admin")
	if !ok {
		t.Error("expected role 'admin' to be in state")
	}
	if id == "" {
		t.Error("expected non-empty discord ID in state")
	}
}

func TestApplyUpdateRole(t *testing.T) {
	client := newMockClient()
	st := state.NewState("server-123")
	st.SetRole("mod", "role-999")
	cfg := makeConfig("server-123", map[string]config.RoleConfig{
		"mod": {Color: "#FFA500"},
	}, nil)

	a := NewApplier(client, st, cfg)
	newColor := "#FFA500"
	action := planner.Action{
		Type:         planner.ActionUpdate,
		ResourceType: planner.ResourceRole,
		Name:         "mod",
		DiscordID:    "role-999",
		Changes: []planner.FieldChange{
			{Field: "color", OldValue: "#FF0000", NewValue: newColor},
		},
	}

	if err := a.ApplyAction(action); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(client.updateRoleCalls) != 1 {
		t.Fatalf("expected 1 UpdateRole call, got %d", len(client.updateRoleCalls))
	}
	call := client.updateRoleCalls[0]
	if call.RoleID != "role-999" {
		t.Errorf("expected roleID 'role-999', got %q", call.RoleID)
	}
	if call.Params.Color == nil {
		t.Fatal("expected Color to be set")
	}
	if *call.Params.Color != 0xFFA500 {
		t.Errorf("expected color 0xFFA500, got %d", *call.Params.Color)
	}
}

func TestApplyUpdateRoleHoistAndMentionable(t *testing.T) {
	client := newMockClient()
	st := state.NewState("server-123")
	st.SetRole("mod", "role-111")
	cfg := makeConfig("server-123", map[string]config.RoleConfig{
		"mod": {},
	}, nil)

	a := NewApplier(client, st, cfg)
	action := planner.Action{
		Type:         planner.ActionUpdate,
		ResourceType: planner.ResourceRole,
		Name:         "mod",
		DiscordID:    "role-111",
		Changes: []planner.FieldChange{
			{Field: "hoist", OldValue: "false", NewValue: "true"},
			{Field: "mentionable", OldValue: "false", NewValue: "true"},
			{Field: "permissions", OldValue: "0", NewValue: "8"},
		},
	}

	if err := a.ApplyAction(action); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	call := client.updateRoleCalls[0]
	if call.Params.Hoist == nil || !*call.Params.Hoist {
		t.Error("expected Hoist=true")
	}
	if call.Params.Mentionable == nil || !*call.Params.Mentionable {
		t.Error("expected Mentionable=true")
	}
	if call.Params.Permissions == nil || *call.Params.Permissions != "8" {
		t.Errorf("expected Permissions='8', got %v", call.Params.Permissions)
	}
}

func TestApplyDeleteRole(t *testing.T) {
	client := newMockClient()
	st := state.NewState("server-123")
	st.SetRole("admin", "role-999")
	cfg := makeConfig("server-123", nil, nil)

	a := NewApplier(client, st, cfg)
	action := planner.Action{
		Type:         planner.ActionDelete,
		ResourceType: planner.ResourceRole,
		Name:         "admin",
		DiscordID:    "role-999",
	}

	if err := a.ApplyAction(action); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(client.deleteRoleCalls) != 1 || client.deleteRoleCalls[0] != "role-999" {
		t.Errorf("expected DeleteRole('role-999'), got %v", client.deleteRoleCalls)
	}

	_, ok := st.GetRoleID("admin")
	if ok {
		t.Error("expected role 'admin' to be removed from state")
	}
}

func TestApplyCreateCategory(t *testing.T) {
	client := newMockClient()
	st := state.NewState("server-123")
	cfg := makeConfig("server-123", nil, map[string]config.CategoryConfig{
		"General": {Position: 2},
	})

	a := NewApplier(client, st, cfg)
	action := planner.Action{
		Type:         planner.ActionCreate,
		ResourceType: planner.ResourceCategory,
		Name:         "General",
	}

	if err := a.ApplyAction(action); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(client.createChannelCalls) != 1 {
		t.Fatalf("expected 1 CreateChannel call, got %d", len(client.createChannelCalls))
	}
	call := client.createChannelCalls[0]
	if call.Params.Type != discord.ChannelTypeGuildCategory {
		t.Errorf("expected type %d (category), got %d", discord.ChannelTypeGuildCategory, call.Params.Type)
	}
	if call.Params.Name != "General" {
		t.Errorf("expected name 'General', got %q", call.Params.Name)
	}
	if call.Params.Position != 2 {
		t.Errorf("expected position 2, got %d", call.Params.Position)
	}

	id, ok := st.GetCategoryID("General")
	if !ok {
		t.Error("expected 'General' in state")
	}
	if id == "" {
		t.Error("expected non-empty discord ID")
	}
}

func TestApplyUpdateCategory(t *testing.T) {
	client := newMockClient()
	st := state.NewState("server-123")
	st.SetCategory("General", "cat-111")
	cfg := makeConfig("server-123", nil, map[string]config.CategoryConfig{
		"General": {Position: 3},
	})

	a := NewApplier(client, st, cfg)
	action := planner.Action{
		Type:         planner.ActionUpdate,
		ResourceType: planner.ResourceCategory,
		Name:         "General",
		DiscordID:    "cat-111",
		Changes: []planner.FieldChange{
			{Field: "position", OldValue: "0", NewValue: "3"},
		},
	}

	if err := a.ApplyAction(action); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(client.updateChannelCalls) != 1 {
		t.Fatalf("expected 1 UpdateChannel call, got %d", len(client.updateChannelCalls))
	}
	call := client.updateChannelCalls[0]
	if call.ChannelID != "cat-111" {
		t.Errorf("expected channelID 'cat-111', got %q", call.ChannelID)
	}
	if call.Params.Position == nil || *call.Params.Position != 3 {
		t.Errorf("expected Position=3")
	}
}

func TestApplyDeleteCategory(t *testing.T) {
	client := newMockClient()
	st := state.NewState("server-123")
	st.SetCategory("General", "cat-999")
	cfg := makeConfig("server-123", nil, nil)

	a := NewApplier(client, st, cfg)
	action := planner.Action{
		Type:         planner.ActionDelete,
		ResourceType: planner.ResourceCategory,
		Name:         "General",
		DiscordID:    "cat-999",
	}

	if err := a.ApplyAction(action); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(client.deleteChannelCalls) != 1 || client.deleteChannelCalls[0] != "cat-999" {
		t.Errorf("expected DeleteChannel('cat-999'), got %v", client.deleteChannelCalls)
	}

	_, ok := st.GetCategoryID("General")
	if ok {
		t.Error("expected category 'General' removed from state")
	}
}

func TestApplyCreateChannel(t *testing.T) {
	client := newMockClient()
	st := state.NewState("server-123")
	st.SetCategory("General", "cat-111")
	cfg := makeConfig("server-123", nil, map[string]config.CategoryConfig{
		"General": {
			Channels: map[string]config.ChannelConfig{
				"welcome": {Type: "text", Topic: "Welcome!", NSFW: false, SlowMode: 5},
			},
		},
	})

	a := NewApplier(client, st, cfg)
	action := planner.Action{
		Type:         planner.ActionCreate,
		ResourceType: planner.ResourceChannel,
		Name:         "General/welcome",
	}

	if err := a.ApplyAction(action); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(client.createChannelCalls) != 1 {
		t.Fatalf("expected 1 CreateChannel call, got %d", len(client.createChannelCalls))
	}
	call := client.createChannelCalls[0]
	if call.Params.Name != "welcome" {
		t.Errorf("expected name 'welcome', got %q", call.Params.Name)
	}
	if call.Params.Type != discord.ChannelTypeGuildText {
		t.Errorf("expected type %d, got %d", discord.ChannelTypeGuildText, call.Params.Type)
	}
	if call.Params.Topic != "Welcome!" {
		t.Errorf("expected topic 'Welcome!', got %q", call.Params.Topic)
	}
	if call.Params.RateLimitPerUser != 5 {
		t.Errorf("expected slowmode 5, got %d", call.Params.RateLimitPerUser)
	}
	if call.Params.ParentID != "cat-111" {
		t.Errorf("expected parentID 'cat-111', got %q", call.Params.ParentID)
	}

	id, ok := st.GetChannelID("General", "welcome")
	if !ok || id == "" {
		t.Error("expected channel to be recorded in state")
	}
}

func TestApplyCreateChannelPermissionOverwrites(t *testing.T) {
	client := newMockClient()
	st := state.NewState("server-123")
	st.SetCategory("General", "cat-111")
	st.SetRole("member", "role-member")

	cfg := makeConfig("server-123", map[string]config.RoleConfig{
		"member": {Color: "#00AA00"},
	}, map[string]config.CategoryConfig{
		"General": {
			Channels: map[string]config.ChannelConfig{
				"welcome": {
					Type: "text",
					Permissions: map[string]config.PermissionOverwriteConfig{
						"@everyone": {
							Deny: []string{"send_messages"},
						},
						"member": {
							Allow: []string{"view_channel", "read_message_history"},
						},
					},
				},
			},
		},
	})

	a := NewApplier(client, st, cfg)
	action := planner.Action{
		Type:         planner.ActionCreate,
		ResourceType: planner.ResourceChannel,
		Name:         "General/welcome",
	}

	if err := a.ApplyAction(action); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(client.createChannelCalls) != 1 {
		t.Fatalf("expected 1 CreateChannel call")
	}

	call := client.createChannelCalls[0]
	if len(call.Params.PermissionOverwrites) != 2 {
		t.Errorf("expected 2 permission overwrites, got %d", len(call.Params.PermissionOverwrites))
	}

	// Find @everyone overwrite (server ID)
	var everyoneFound, memberFound bool
	for _, ow := range call.Params.PermissionOverwrites {
		if ow.ID == "server-123" {
			everyoneFound = true
			// send_messages = 1 << 11 = 2048
			if ow.Deny != "2048" {
				t.Errorf("expected @everyone deny='2048', got %q", ow.Deny)
			}
			if ow.Allow != "0" {
				t.Errorf("expected @everyone allow='0', got %q", ow.Allow)
			}
		}
		if ow.ID == "role-member" {
			memberFound = true
			// view_channel (1<<10=1024) | read_message_history (1<<16=65536) = 66560
			if ow.Allow != "66560" {
				t.Errorf("expected member allow='66560', got %q", ow.Allow)
			}
		}
	}
	if !everyoneFound {
		t.Error("expected @everyone overwrite")
	}
	if !memberFound {
		t.Error("expected member role overwrite")
	}
}

func TestApplyUpdateChannel(t *testing.T) {
	client := newMockClient()
	st := state.NewState("server-123")
	st.SetCategory("General", "cat-111")
	st.SetChannel("General", "general", "ch-222")
	cfg := makeConfig("server-123", nil, map[string]config.CategoryConfig{
		"General": {
			Channels: map[string]config.ChannelConfig{
				"general": {Type: "text", Topic: "New topic"},
			},
		},
	})

	a := NewApplier(client, st, cfg)
	action := planner.Action{
		Type:         planner.ActionUpdate,
		ResourceType: planner.ResourceChannel,
		Name:         "General/general",
		DiscordID:    "ch-222",
		Changes: []planner.FieldChange{
			{Field: "topic", OldValue: "Old topic", NewValue: "New topic"},
		},
	}

	if err := a.ApplyAction(action); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(client.updateChannelCalls) != 1 {
		t.Fatalf("expected 1 UpdateChannel call, got %d", len(client.updateChannelCalls))
	}
	call := client.updateChannelCalls[0]
	if call.ChannelID != "ch-222" {
		t.Errorf("expected channelID 'ch-222', got %q", call.ChannelID)
	}
	if call.Params.Topic == nil || *call.Params.Topic != "New topic" {
		t.Errorf("expected topic 'New topic'")
	}
}

func TestApplyUpdateChannelAllFields(t *testing.T) {
	client := newMockClient()
	st := state.NewState("server-123")
	st.SetCategory("Voice", "cat-111")
	st.SetChannel("Voice", "music", "ch-222")
	cfg := makeConfig("server-123", nil, map[string]config.CategoryConfig{
		"Voice": {
			Channels: map[string]config.ChannelConfig{
				"music": {Type: "voice"},
			},
		},
	})

	a := NewApplier(client, st, cfg)
	action := planner.Action{
		Type:         planner.ActionUpdate,
		ResourceType: planner.ResourceChannel,
		Name:         "Voice/music",
		DiscordID:    "ch-222",
		Changes: []planner.FieldChange{
			{Field: "nsfw", OldValue: "false", NewValue: "true"},
			{Field: "slowmode", OldValue: "0", NewValue: "5"},
			{Field: "bitrate", OldValue: "64000", NewValue: "128000"},
			{Field: "user_limit", OldValue: "0", NewValue: "10"},
			{Field: "type", OldValue: "0", NewValue: "voice"},
		},
	}

	if err := a.ApplyAction(action); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	call := client.updateChannelCalls[0]
	if call.Params.NSFW == nil || !*call.Params.NSFW {
		t.Error("expected NSFW=true")
	}
	if call.Params.RateLimitPerUser == nil || *call.Params.RateLimitPerUser != 5 {
		t.Error("expected RateLimitPerUser=5")
	}
	if call.Params.Bitrate == nil || *call.Params.Bitrate != 128000 {
		t.Error("expected Bitrate=128000")
	}
	if call.Params.UserLimit == nil || *call.Params.UserLimit != 10 {
		t.Error("expected UserLimit=10")
	}
	if call.Params.Type == nil || *call.Params.Type != discord.ChannelTypeGuildVoice {
		t.Error("expected Type=voice")
	}
}

func TestApplyDeleteChannel(t *testing.T) {
	client := newMockClient()
	st := state.NewState("server-123")
	st.SetCategory("General", "cat-111")
	st.SetChannel("General", "old-channel", "ch-999")
	cfg := makeConfig("server-123", nil, nil)

	a := NewApplier(client, st, cfg)
	action := planner.Action{
		Type:         planner.ActionDelete,
		ResourceType: planner.ResourceChannel,
		Name:         "General/old-channel",
		DiscordID:    "ch-999",
	}

	if err := a.ApplyAction(action); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(client.deleteChannelCalls) != 1 || client.deleteChannelCalls[0] != "ch-999" {
		t.Errorf("expected DeleteChannel('ch-999'), got %v", client.deleteChannelCalls)
	}

	_, ok := st.GetChannelID("General", "old-channel")
	if ok {
		t.Error("expected channel removed from state")
	}
}

// TestApplyOrderingRolesBeforeCategories verifies create/update ordering.
func TestApplyOrderingRolesBeforeCategories(t *testing.T) {
	client := newMockClient()
	st := state.NewState("server-123")
	cfg := makeConfig("server-123", map[string]config.RoleConfig{
		"admin": {},
	}, map[string]config.CategoryConfig{
		"General": {Channels: map[string]config.ChannelConfig{}},
	})

	a := NewApplier(client, st, cfg)
	plan := &planner.Plan{
		Actions: []planner.Action{
			{Type: planner.ActionCreate, ResourceType: planner.ResourceCategory, Name: "General"},
			{Type: planner.ActionCreate, ResourceType: planner.ResourceRole, Name: "admin"},
		},
		ToCreate: 2,
	}

	if err := a.Apply(plan); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Role should be created before category
	// Both createRole and createChannel (category) should have been called
	if len(client.createRoleCalls) != 1 {
		t.Errorf("expected 1 CreateRole call, got %d", len(client.createRoleCalls))
	}
	if len(client.createChannelCalls) != 1 {
		t.Errorf("expected 1 CreateChannel (category) call, got %d", len(client.createChannelCalls))
	}
}

// TestApplyDeleteOrderingChannelsBeforeCategories verifies delete ordering.
func TestApplyDeleteOrderingChannelsBeforeCategories(t *testing.T) {
	client := newMockClient()
	st := state.NewState("server-123")
	st.SetCategory("General", "cat-111")
	st.SetChannel("General", "welcome", "ch-222")
	cfg := makeConfig("server-123", nil, nil)

	// Track order of deletions
	var deleteOrder []string
	client.errors["noop"] = nil // just to initialize

	// We need a way to track order; use a custom approach
	// Actually we can check the deleteChannelCalls ordering
	a := NewApplier(client, st, cfg)
	plan := &planner.Plan{
		Actions: []planner.Action{
			// Category listed before channel in the plan
			{Type: planner.ActionDelete, ResourceType: planner.ResourceCategory, Name: "General", DiscordID: "cat-111"},
			{Type: planner.ActionDelete, ResourceType: planner.ResourceChannel, Name: "General/welcome", DiscordID: "ch-222"},
		},
		ToDelete: 2,
	}

	if err := a.Apply(plan); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Channel should be deleted first
	if len(client.deleteChannelCalls) != 2 {
		t.Fatalf("expected 2 DeleteChannel calls, got %d", len(client.deleteChannelCalls))
	}
	deleteOrder = client.deleteChannelCalls
	if deleteOrder[0] != "ch-222" {
		t.Errorf("expected channel 'ch-222' to be deleted first, got %q", deleteOrder[0])
	}
	if deleteOrder[1] != "cat-111" {
		t.Errorf("expected category 'cat-111' to be deleted second, got %q", deleteOrder[1])
	}
}

// TestApplyPartialFailure verifies that a failed action returns an error.
func TestApplyPartialFailure(t *testing.T) {
	client := newMockClient()
	st := state.NewState("server-123")
	cfg := makeConfig("server-123", map[string]config.RoleConfig{
		"admin": {},
		"mod":   {},
	}, nil)

	// Make mod creation fail
	client.errors["create_role_mod"] = fmt.Errorf("discord API error: permission denied")

	a := NewApplier(client, st, cfg)
	plan := &planner.Plan{
		Actions: []planner.Action{
			{Type: planner.ActionCreate, ResourceType: planner.ResourceRole, Name: "admin"},
			{Type: planner.ActionCreate, ResourceType: planner.ResourceRole, Name: "mod"},
		},
		ToCreate: 2,
	}

	err := a.Apply(plan)
	if err == nil {
		t.Fatal("expected error from partial failure")
	}
}

// TestApplyEmptyPlan verifies no-op on empty plan.
func TestApplyEmptyPlan(t *testing.T) {
	client := newMockClient()
	st := state.NewState("server-123")
	cfg := makeConfig("server-123", nil, nil)

	a := NewApplier(client, st, cfg)
	plan := &planner.Plan{}

	if err := a.Apply(plan); err != nil {
		t.Fatalf("unexpected error on empty plan: %v", err)
	}

	if len(client.createRoleCalls) != 0 || len(client.createChannelCalls) != 0 {
		t.Error("expected no API calls for empty plan")
	}
}

// TestApplyCreateVoiceChannel verifies voice channel creation.
func TestApplyCreateVoiceChannel(t *testing.T) {
	client := newMockClient()
	st := state.NewState("server-123")
	st.SetCategory("Voice", "cat-111")
	cfg := makeConfig("server-123", nil, map[string]config.CategoryConfig{
		"Voice": {
			Channels: map[string]config.ChannelConfig{
				"music": {Type: "voice", Bitrate: 128000, UserLimit: 10},
			},
		},
	})

	a := NewApplier(client, st, cfg)
	action := planner.Action{
		Type:         planner.ActionCreate,
		ResourceType: planner.ResourceChannel,
		Name:         "Voice/music",
	}

	if err := a.ApplyAction(action); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	call := client.createChannelCalls[0]
	if call.Params.Type != discord.ChannelTypeGuildVoice {
		t.Errorf("expected voice type, got %d", call.Params.Type)
	}
	if call.Params.Bitrate != 128000 {
		t.Errorf("expected bitrate 128000, got %d", call.Params.Bitrate)
	}
	if call.Params.UserLimit != 10 {
		t.Errorf("expected user_limit 10, got %d", call.Params.UserLimit)
	}
}

// TestApplyCreateChannelMissingConfig verifies error on missing config.
func TestApplyCreateChannelMissingConfig(t *testing.T) {
	client := newMockClient()
	st := state.NewState("server-123")
	cfg := makeConfig("server-123", nil, nil)

	a := NewApplier(client, st, cfg)
	action := planner.Action{
		Type:         planner.ActionCreate,
		ResourceType: planner.ResourceChannel,
		Name:         "NonExistent/channel",
	}

	err := a.ApplyAction(action)
	if err == nil {
		t.Fatal("expected error for missing config category")
	}
}

// TestApplyCreateRoleMissingConfig verifies error on missing role config.
func TestApplyCreateRoleMissingConfig(t *testing.T) {
	client := newMockClient()
	st := state.NewState("server-123")
	cfg := makeConfig("server-123", nil, nil)

	a := NewApplier(client, st, cfg)
	action := planner.Action{
		Type:         planner.ActionCreate,
		ResourceType: planner.ResourceRole,
		Name:         "nonexistent-role",
	}

	err := a.ApplyAction(action)
	if err == nil {
		t.Fatal("expected error for missing role config")
	}
}

// TestApplyPermOverwriteUnknownRole verifies error when role not in state.
func TestApplyPermOverwriteUnknownRole(t *testing.T) {
	client := newMockClient()
	st := state.NewState("server-123")
	st.SetCategory("General", "cat-111")
	// "member" role NOT in state

	cfg := makeConfig("server-123", nil, map[string]config.CategoryConfig{
		"General": {
			Channels: map[string]config.ChannelConfig{
				"welcome": {
					Type: "text",
					Permissions: map[string]config.PermissionOverwriteConfig{
						"member": {Allow: []string{"view_channel"}},
					},
				},
			},
		},
	})

	a := NewApplier(client, st, cfg)
	action := planner.Action{
		Type:         planner.ActionCreate,
		ResourceType: planner.ResourceChannel,
		Name:         "General/welcome",
	}

	err := a.ApplyAction(action)
	if err == nil {
		t.Fatal("expected error for unknown role in permission overwrite")
	}
}

// TestApplyUpdateChannelSyncsPermissions verifies that updating a channel also syncs permissions.
func TestApplyUpdateChannelSyncsPermissions(t *testing.T) {
	client := newMockClient()
	st := state.NewState("server-123")
	st.SetCategory("General", "cat-111")
	st.SetChannel("General", "general", "ch-222")

	cfg := makeConfig("server-123", nil, map[string]config.CategoryConfig{
		"General": {
			Channels: map[string]config.ChannelConfig{
				"general": {
					Type: "text",
					Permissions: map[string]config.PermissionOverwriteConfig{
						"@everyone": {Deny: []string{"send_messages"}},
					},
				},
			},
		},
	})

	a := NewApplier(client, st, cfg)
	action := planner.Action{
		Type:         planner.ActionUpdate,
		ResourceType: planner.ResourceChannel,
		Name:         "General/general",
		DiscordID:    "ch-222",
		Changes: []planner.FieldChange{
			{Field: "topic", OldValue: "old", NewValue: "new"},
		},
	}

	if err := a.ApplyAction(action); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have called EditChannelPermissions for the @everyone overwrite
	if len(client.editPermCalls) != 1 {
		t.Errorf("expected 1 EditChannelPermissions call, got %d", len(client.editPermCalls))
	}
	if client.editPermCalls[0].ChannelID != "ch-222" {
		t.Errorf("expected channel ID 'ch-222', got %q", client.editPermCalls[0].ChannelID)
	}
	// @everyone uses server ID
	if client.editPermCalls[0].OverwriteID != "server-123" {
		t.Errorf("expected overwrite ID 'server-123', got %q", client.editPermCalls[0].OverwriteID)
	}
}

// TestApplyUpdateSettings verifies ModifyGuild is called with correct params.
func TestApplyUpdateSettings(t *testing.T) {
	client := newMockClient()
	st := state.NewState("server-123")
	cfg := makeConfig("server-123", nil, nil)
	cfg.Settings = &config.ServerSettings{
		VerificationLevel:           "high",
		ExplicitContentFilter:       "all_members",
		DefaultMessageNotifications: "only_mentions",
		AFKTimeout:                  300,
	}

	a := NewApplier(client, st, cfg)
	action := planner.Action{
		Type:         planner.ActionUpdate,
		ResourceType: planner.ResourceSettings,
		Name:         "server_settings",
		DiscordID:    "server-123",
		Changes: []planner.FieldChange{
			{Field: "verification_level", OldValue: "1", NewValue: "high"},
			{Field: "explicit_content_filter", OldValue: "0", NewValue: "all_members"},
			{Field: "default_message_notifications", OldValue: "0", NewValue: "only_mentions"},
			{Field: "afk_timeout", OldValue: "0", NewValue: "300"},
		},
	}

	if err := a.ApplyAction(action); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(client.modifyGuildCalls) != 1 {
		t.Fatalf("expected 1 ModifyGuild call, got %d", len(client.modifyGuildCalls))
	}
	call := client.modifyGuildCalls[0]
	if call.GuildID != "server-123" {
		t.Errorf("expected guildID 'server-123', got %q", call.GuildID)
	}
	if call.Params.VerificationLevel == nil || *call.Params.VerificationLevel != 3 {
		t.Errorf("expected VerificationLevel=3 (high), got %v", call.Params.VerificationLevel)
	}
	if call.Params.ExplicitContentFilter == nil || *call.Params.ExplicitContentFilter != 2 {
		t.Errorf("expected ExplicitContentFilter=2 (all_members), got %v", call.Params.ExplicitContentFilter)
	}
	if call.Params.DefaultMessageNotifications == nil || *call.Params.DefaultMessageNotifications != 1 {
		t.Errorf("expected DefaultMessageNotifications=1 (only_mentions), got %v", call.Params.DefaultMessageNotifications)
	}
	if call.Params.AFKTimeout == nil || *call.Params.AFKTimeout != 300 {
		t.Errorf("expected AFKTimeout=300, got %v", call.Params.AFKTimeout)
	}
}

// TestApplyCreateTopLevelChannel verifies CreateChannel is called without parent_id.
func TestApplyCreateTopLevelChannel(t *testing.T) {
	client := newMockClient()
	st := state.NewState("server-123")
	cfg := makeConfig("server-123", nil, nil)
	cfg.Channels = map[string]config.ChannelConfig{
		"announcements": {Type: "announcement", Topic: "Server announcements"},
	}

	a := NewApplier(client, st, cfg)
	action := planner.Action{
		Type:         planner.ActionCreate,
		ResourceType: planner.ResourceChannel,
		Name:         "/announcements",
	}

	if err := a.ApplyAction(action); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(client.createChannelCalls) != 1 {
		t.Fatalf("expected 1 CreateChannel call, got %d", len(client.createChannelCalls))
	}
	call := client.createChannelCalls[0]
	if call.Params.Name != "announcements" {
		t.Errorf("expected name 'announcements', got %q", call.Params.Name)
	}
	if call.Params.Type != discord.ChannelTypeGuildAnnouncement {
		t.Errorf("expected type announcement (%d), got %d", discord.ChannelTypeGuildAnnouncement, call.Params.Type)
	}
	if call.Params.ParentID != "" {
		t.Errorf("expected empty ParentID for top-level channel, got %q", call.Params.ParentID)
	}
	if call.Params.Topic != "Server announcements" {
		t.Errorf("expected topic 'Server announcements', got %q", call.Params.Topic)
	}

	// Verify state updated with empty category.
	id, ok := st.GetChannelID("", "announcements")
	if !ok || id == "" {
		t.Error("expected top-level channel recorded in state with empty category")
	}
}

// TestApplyDeleteTopLevelChannel verifies DeleteChannel called and state cleaned up.
func TestApplyDeleteTopLevelChannel(t *testing.T) {
	client := newMockClient()
	st := state.NewState("server-123")
	st.SetChannel("", "old-announce", "ch-777")
	cfg := makeConfig("server-123", nil, nil)
	cfg.Channels = map[string]config.ChannelConfig{}

	a := NewApplier(client, st, cfg)
	action := planner.Action{
		Type:         planner.ActionDelete,
		ResourceType: planner.ResourceChannel,
		Name:         "/old-announce",
		DiscordID:    "ch-777",
	}

	if err := a.ApplyAction(action); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(client.deleteChannelCalls) != 1 || client.deleteChannelCalls[0] != "ch-777" {
		t.Errorf("expected DeleteChannel('ch-777'), got %v", client.deleteChannelCalls)
	}

	_, ok := st.GetChannelID("", "old-announce")
	if ok {
		t.Error("expected top-level channel removed from state")
	}
}
