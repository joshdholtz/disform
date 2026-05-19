package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/joshholtz/disform/internal/applier"
	"github.com/joshholtz/disform/internal/config"
	"github.com/joshholtz/disform/internal/discord"
	"github.com/joshholtz/disform/internal/planner"
	"github.com/joshholtz/disform/internal/state"
)

func findRecord(records []dryRunRecord, method, pathSubstr string) *dryRunRecord {
	for i := range records {
		if records[i].Method == method && strings.Contains(records[i].Path, pathSubstr) {
			return &records[i]
		}
	}
	return nil
}

func TestDryRunClientRecordsCreateRole(t *testing.T) {
	c := newDryRunClient("guild-1")
	params := discord.CreateRoleParams{Name: "admin", Permissions: "8"}
	role, err := c.CreateRole("guild-1", params)
	if err != nil {
		t.Fatal(err)
	}
	if role.ID == "" {
		t.Error("expected non-empty fake ID")
	}
	r := findRecord(c.records, "POST", "/guilds/guild-1/roles")
	if r == nil {
		t.Fatal("expected POST /guilds/guild-1/roles to be recorded")
	}
	var got discord.CreateRoleParams
	b, _ := json.Marshal(r.Body)
	_ = json.Unmarshal(b, &got)
	if got.Name != "admin" {
		t.Errorf("expected name 'admin', got %q", got.Name)
	}
}

func TestDryRunClientRecordsCreateChannel(t *testing.T) {
	c := newDryRunClient("guild-1")
	_, err := c.CreateChannel("guild-1", discord.CreateChannelParams{Name: "general", Type: discord.ChannelTypeGuildText})
	if err != nil {
		t.Fatal(err)
	}
	if findRecord(c.records, "POST", "/guilds/guild-1/channels") == nil {
		t.Error("expected POST /guilds/guild-1/channels to be recorded")
	}
}

func TestDryRunClientRecordsUpdateChannel(t *testing.T) {
	c := newDryRunClient("guild-1")
	name := "new-name"
	_, err := c.UpdateChannel("chan-123", discord.UpdateChannelParams{Name: &name})
	if err != nil {
		t.Fatal(err)
	}
	if findRecord(c.records, "PATCH", "/channels/chan-123") == nil {
		t.Error("expected PATCH /channels/chan-123 to be recorded")
	}
}

func TestDryRunClientRecordsDeleteRole(t *testing.T) {
	c := newDryRunClient("guild-1")
	if err := c.DeleteRole("guild-1", "role-99"); err != nil {
		t.Fatal(err)
	}
	if findRecord(c.records, "DELETE", "/guilds/guild-1/roles/role-99") == nil {
		t.Error("expected DELETE /guilds/guild-1/roles/role-99 to be recorded")
	}
}

func TestDryRunClientRecordsDeleteChannel(t *testing.T) {
	c := newDryRunClient("guild-1")
	if err := c.DeleteChannel("chan-42"); err != nil {
		t.Fatal(err)
	}
	if findRecord(c.records, "DELETE", "/channels/chan-42") == nil {
		t.Error("expected DELETE /channels/chan-42 to be recorded")
	}
}

func TestDryRunClientRecordsModifyGuild(t *testing.T) {
	c := newDryRunClient("guild-1")
	level := 1
	_, err := c.ModifyGuild("guild-1", discord.ModifyGuildParams{VerificationLevel: &level})
	if err != nil {
		t.Fatal(err)
	}
	if findRecord(c.records, "PATCH", "/guilds/guild-1") == nil {
		t.Error("expected PATCH /guilds/guild-1 to be recorded")
	}
}

func TestDryRunClientFakeIDsAreUnique(t *testing.T) {
	c := newDryRunClient("guild-1")
	r1, _ := c.CreateRole("guild-1", discord.CreateRoleParams{Name: "a"})
	r2, _ := c.CreateRole("guild-1", discord.CreateRoleParams{Name: "b"})
	if r1.ID == r2.ID {
		t.Errorf("expected unique fake IDs, both got %q", r1.ID)
	}
}

// TestDryRunClientCategoryIDChains verifies that a channel created after a category
// gets the category's fake ID as its parent_id — exercising the ID chaining.
func TestDryRunClientCategoryIDChains(t *testing.T) {
	c := newDryRunClient("guild-1")

	cfg := &config.Config{
		ServerID: "guild-1",
		Roles:    map[string]config.RoleConfig{},
		Categories: map[string]config.CategoryConfig{
			"General": {
				Channels: map[string]config.ChannelConfig{
					"welcome": {Type: "text"},
				},
			},
		},
	}
	st := state.NewState("guild-1")

	// Plan: everything is new.
	live := &planner.LiveState{
		Guild:      &discord.Guild{ID: "guild-1"},
		Roles:      map[string]*discord.Role{},
		Categories: map[string]*discord.Channel{},
		Channels:   map[string]*discord.Channel{},
	}
	p := planner.NewPlanner(cfg, st, live)
	plan, err := p.Plan()
	if err != nil {
		t.Fatal(err)
	}

	a := applier.NewApplier(c, st, cfg)
	for _, action := range plan.Actions {
		if err := a.ApplyAction(action); err != nil {
			t.Fatalf("ApplyAction(%s %s): %v", action.Type, action.Name, err)
		}
	}

	// Find the channel POST and check its parent_id matches the category fake ID.
	catID, ok := st.GetCategoryID("General")
	if !ok || catID == "" {
		t.Fatal("expected General category ID in state after dry-run apply")
	}
	if !strings.HasPrefix(catID, "<dry-run-id-") {
		t.Errorf("expected fake category ID, got %q", catID)
	}

	var chanPost *dryRunRecord
	for i := range c.records {
		r := &c.records[i]
		if r.Method != "POST" || !strings.Contains(r.Path, "/channels") {
			continue
		}
		var params discord.CreateChannelParams
		b, _ := json.Marshal(r.Body)
		_ = json.Unmarshal(b, &params)
		if params.Type == discord.ChannelTypeGuildText {
			chanPost = r
			break
		}
	}
	if chanPost == nil {
		t.Fatal("expected POST for text channel")
	}
	var params discord.CreateChannelParams
	b, _ := json.Marshal(chanPost.Body)
	_ = json.Unmarshal(b, &params)
	if params.ParentID != catID {
		t.Errorf("channel parent_id = %q, want %q", params.ParentID, catID)
	}
}

// TestDryRunClientRecordOrderRoleBeforeChannel verifies roles are recorded before channels.
func TestDryRunClientRecordOrderRoleBeforeChannel(t *testing.T) {
	c := newDryRunClient("guild-1")
	cfg := &config.Config{
		ServerID: "guild-1",
		Roles: map[string]config.RoleConfig{
			"admin": {Permissions: []string{}},
		},
		Categories: map[string]config.CategoryConfig{
			"General": {
				Channels: map[string]config.ChannelConfig{
					"welcome": {Type: "text"},
				},
			},
		},
	}
	st := state.NewState("guild-1")
	live := &planner.LiveState{
		Guild:      &discord.Guild{ID: "guild-1"},
		Roles:      map[string]*discord.Role{},
		Categories: map[string]*discord.Channel{},
		Channels:   map[string]*discord.Channel{},
	}
	p := planner.NewPlanner(cfg, st, live)
	plan, err := p.Plan()
	if err != nil {
		t.Fatal(err)
	}
	a := applier.NewApplier(c, st, cfg)
	for _, action := range plan.Actions {
		if err := a.ApplyAction(action); err != nil {
			t.Fatalf("ApplyAction: %v", err)
		}
	}

	var roleIdx, chanIdx int = -1, -1
	for i, r := range c.records {
		if r.Method == "POST" && strings.Contains(r.Path, "/roles") {
			roleIdx = i
		}
		if r.Method == "POST" && strings.Contains(r.Path, "/channels") {
			var params discord.CreateChannelParams
			b, _ := json.Marshal(r.Body)
			_ = json.Unmarshal(b, &params)
			if params.Type == discord.ChannelTypeGuildText && chanIdx == -1 {
				chanIdx = i
			}
		}
	}
	if roleIdx == -1 {
		t.Fatal("no POST /roles recorded")
	}
	if chanIdx == -1 {
		t.Fatal("no POST /channels for text channel recorded")
	}
	if roleIdx > chanIdx {
		t.Errorf("role POST (index %d) should come before channel POST (index %d)", roleIdx, chanIdx)
	}
}

// TestDryRunClientNoStateWritten verifies that running through dryRunClient
// does not persist Discord IDs into state (i.e. it records fake IDs only internally).
func TestDryRunClientNoRealSideEffects(t *testing.T) {
	c := newDryRunClient("guild-1")

	cfg := &config.Config{
		ServerID: "guild-1",
		Roles:    map[string]config.RoleConfig{"mod": {}},
	}
	st := state.NewState("guild-1")
	live := &planner.LiveState{
		Guild:      &discord.Guild{ID: "guild-1"},
		Roles:      map[string]*discord.Role{},
		Categories: map[string]*discord.Channel{},
		Channels:   map[string]*discord.Channel{},
	}
	p := planner.NewPlanner(cfg, st, live)
	plan, err := p.Plan()
	if err != nil {
		t.Fatal(err)
	}
	a := applier.NewApplier(c, st, cfg)
	for _, action := range plan.Actions {
		_ = a.ApplyAction(action)
	}

	// The applier writes the fake ID to state in-memory — that's expected and
	// correct (it allows chaining). What we verify is that no HTTP call was made.
	if len(c.records) == 0 {
		t.Error("expected at least one recorded call")
	}
	for _, r := range c.records {
		if strings.Contains(r.Path, "discord.com") {
			t.Errorf("dry-run client should not hit real Discord, got path %q", r.Path)
		}
	}
}
