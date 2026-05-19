package planner

import (
	"strings"
	"testing"

	"github.com/joshholtz/disform/internal/config"
	"github.com/joshholtz/disform/internal/discord"
	"github.com/joshholtz/disform/internal/state"
)

// helpers

func buildLiveState(roles []*discord.Role, categories []*discord.Channel, channels []*discord.Channel) *LiveState {
	ls := &LiveState{
		Roles:      make(map[string]*discord.Role),
		Categories: make(map[string]*discord.Channel),
		Channels:   make(map[string]*discord.Channel),
	}
	for _, r := range roles {
		ls.Roles[r.ID] = r
	}
	for _, c := range categories {
		ls.Categories[c.ID] = c
	}
	for _, c := range channels {
		ls.Channels[c.ID] = c
	}
	return ls
}

func findAction(plan *Plan, rt ResourceType, name string) *Action {
	for i := range plan.Actions {
		a := &plan.Actions[i]
		if a.ResourceType == rt && a.Name == name {
			return a
		}
	}
	return nil
}

func findFieldChange(changes []FieldChange, field string) *FieldChange {
	for i := range changes {
		if changes[i].Field == field {
			return &changes[i]
		}
	}
	return nil
}

// TestPlanNoChanges verifies an empty plan when config matches live state exactly.
func TestPlanNoChanges(t *testing.T) {
	cfg := &config.Config{
		ServerID: "123",
		Roles: map[string]config.RoleConfig{
			"admin": {Color: "#FF0000", Hoist: true, Permissions: []string{"administrator"}},
		},
		Categories: map[string]config.CategoryConfig{
			"General": {
				Position: 0,
				Channels: map[string]config.ChannelConfig{
					"general": {Type: "text", Topic: "Hello"},
				},
			},
		},
	}

	st := state.NewState("123")
	st.SetRole("admin", "role-111")
	st.SetCategory("General", "cat-222")
	st.SetChannel("General", "general", "ch-333")

	topic := "Hello"
	live := buildLiveState(
		[]*discord.Role{
			{ID: "role-111", Name: "admin", Color: 0xFF0000, Hoist: true, Permissions: "8"},
		},
		[]*discord.Channel{
			{ID: "cat-222", Type: discord.ChannelTypeGuildCategory, Name: "General", Position: 0},
		},
		[]*discord.Channel{
			{ID: "ch-333", Type: discord.ChannelTypeGuildText, Name: "general", Topic: &topic},
		},
	)

	p := NewPlanner(cfg, st, live)
	plan, err := p.Plan()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan.HasChanges() {
		t.Errorf("expected no changes, got %d actions: %v", len(plan.Actions), plan.Actions)
	}
}

// TestPlanCreateRole verifies a create action for a role not in state.
func TestPlanCreateRole(t *testing.T) {
	cfg := &config.Config{
		ServerID: "123",
		Roles: map[string]config.RoleConfig{
			"admin": {Color: "#FF0000"},
		},
		Categories: map[string]config.CategoryConfig{},
	}

	st := state.NewState("123")
	live := buildLiveState(nil, nil, nil)

	p := NewPlanner(cfg, st, live)
	plan, err := p.Plan()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if plan.ToCreate != 1 {
		t.Errorf("expected ToCreate=1, got %d", plan.ToCreate)
	}

	action := findAction(plan, ResourceRole, "admin")
	if action == nil {
		t.Fatal("expected create action for role 'admin'")
	}
	if action.Type != ActionCreate {
		t.Errorf("expected ActionCreate, got %s", action.Type)
	}
	if action.DiscordID != "" {
		t.Errorf("expected empty DiscordID for create, got %q", action.DiscordID)
	}
}

// TestPlanUpdateRole verifies an update action when role color changes.
func TestPlanUpdateRole(t *testing.T) {
	cfg := &config.Config{
		ServerID: "123",
		Roles: map[string]config.RoleConfig{
			"admin": {Color: "#FF0000", Permissions: []string{}},
		},
		Categories: map[string]config.CategoryConfig{},
	}

	st := state.NewState("123")
	st.SetRole("admin", "role-111")

	live := buildLiveState(
		[]*discord.Role{
			// Different color: 0x00FF00 instead of 0xFF0000
			{ID: "role-111", Name: "admin", Color: 0x00FF00, Permissions: "0"},
		},
		nil, nil,
	)

	p := NewPlanner(cfg, st, live)
	plan, err := p.Plan()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if plan.ToUpdate != 1 {
		t.Errorf("expected ToUpdate=1, got %d", plan.ToUpdate)
	}

	action := findAction(plan, ResourceRole, "admin")
	if action == nil {
		t.Fatal("expected update action for role 'admin'")
	}
	if action.Type != ActionUpdate {
		t.Errorf("expected ActionUpdate, got %s", action.Type)
	}
	if action.DiscordID != "role-111" {
		t.Errorf("expected DiscordID 'role-111', got %q", action.DiscordID)
	}

	change := findFieldChange(action.Changes, "color")
	if change == nil {
		t.Fatal("expected 'color' field change")
	}
	if change.NewValue != "#FF0000" {
		t.Errorf("expected new color '#FF0000', got %q", change.NewValue)
	}
}

// TestPlanDeleteRole verifies a delete action for a role in state but not config.
func TestPlanDeleteRole(t *testing.T) {
	cfg := &config.Config{
		ServerID:   "123",
		Roles:      map[string]config.RoleConfig{},
		Categories: map[string]config.CategoryConfig{},
	}

	st := state.NewState("123")
	st.SetRole("oldmod", "role-999")

	live := buildLiveState(
		[]*discord.Role{{ID: "role-999", Name: "oldmod", Permissions: "0"}},
		nil, nil,
	)

	p := NewPlanner(cfg, st, live)
	plan, err := p.Plan()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if plan.ToDelete != 1 {
		t.Errorf("expected ToDelete=1, got %d", plan.ToDelete)
	}

	action := findAction(plan, ResourceRole, "oldmod")
	if action == nil {
		t.Fatal("expected delete action for role 'oldmod'")
	}
	if action.Type != ActionDelete {
		t.Errorf("expected ActionDelete, got %s", action.Type)
	}
	if action.DiscordID != "role-999" {
		t.Errorf("expected DiscordID 'role-999', got %q", action.DiscordID)
	}
}

// TestPlanCreateCategory verifies a create action for a category not in state.
func TestPlanCreateCategory(t *testing.T) {
	cfg := &config.Config{
		ServerID: "123",
		Roles:    map[string]config.RoleConfig{},
		Categories: map[string]config.CategoryConfig{
			"General": {Position: 0, Channels: map[string]config.ChannelConfig{}},
		},
	}

	st := state.NewState("123")
	live := buildLiveState(nil, nil, nil)

	p := NewPlanner(cfg, st, live)
	plan, err := p.Plan()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	action := findAction(plan, ResourceCategory, "General")
	if action == nil {
		t.Fatal("expected create action for category 'General'")
	}
	if action.Type != ActionCreate {
		t.Errorf("expected ActionCreate, got %s", action.Type)
	}
}

// TestPlanUpdateCategory verifies an update action when category position changes.
func TestPlanUpdateCategory(t *testing.T) {
	cfg := &config.Config{
		ServerID: "123",
		Roles:    map[string]config.RoleConfig{},
		Categories: map[string]config.CategoryConfig{
			"General": {Position: 2, Channels: map[string]config.ChannelConfig{}},
		},
	}

	st := state.NewState("123")
	st.SetCategory("General", "cat-111")

	live := buildLiveState(
		nil,
		[]*discord.Channel{
			{ID: "cat-111", Type: discord.ChannelTypeGuildCategory, Name: "General", Position: 0},
		},
		nil,
	)

	p := NewPlanner(cfg, st, live)
	plan, err := p.Plan()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	action := findAction(plan, ResourceCategory, "General")
	if action == nil {
		t.Fatal("expected update action for category 'General'")
	}
	if action.Type != ActionUpdate {
		t.Errorf("expected ActionUpdate, got %s", action.Type)
	}
	change := findFieldChange(action.Changes, "position")
	if change == nil {
		t.Fatal("expected 'position' field change")
	}
	if change.OldValue != "0" || change.NewValue != "2" {
		t.Errorf("expected position change 0->2, got %q->%q", change.OldValue, change.NewValue)
	}
}

// TestPlanDeleteCategory verifies a delete action for a category in state but not config.
func TestPlanDeleteCategory(t *testing.T) {
	cfg := &config.Config{
		ServerID:   "123",
		Roles:      map[string]config.RoleConfig{},
		Categories: map[string]config.CategoryConfig{},
	}

	st := state.NewState("123")
	st.SetCategory("OldCat", "cat-999")

	live := buildLiveState(
		nil,
		[]*discord.Channel{{ID: "cat-999", Type: discord.ChannelTypeGuildCategory, Name: "OldCat"}},
		nil,
	)

	p := NewPlanner(cfg, st, live)
	plan, err := p.Plan()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	action := findAction(plan, ResourceCategory, "OldCat")
	if action == nil {
		t.Fatal("expected delete action for category 'OldCat'")
	}
	if action.Type != ActionDelete {
		t.Errorf("expected ActionDelete, got %s", action.Type)
	}
}

// TestPlanCreateChannel verifies a create action for a channel not in state.
func TestPlanCreateChannel(t *testing.T) {
	cfg := &config.Config{
		ServerID: "123",
		Roles:    map[string]config.RoleConfig{},
		Categories: map[string]config.CategoryConfig{
			"General": {
				Channels: map[string]config.ChannelConfig{
					"welcome": {Type: "text"},
				},
			},
		},
	}

	st := state.NewState("123")
	st.SetCategory("General", "cat-111")

	live := buildLiveState(
		nil,
		[]*discord.Channel{{ID: "cat-111", Type: discord.ChannelTypeGuildCategory, Name: "General"}},
		nil,
	)

	p := NewPlanner(cfg, st, live)
	plan, err := p.Plan()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	action := findAction(plan, ResourceChannel, "General/welcome")
	if action == nil {
		t.Fatal("expected create action for channel 'General/welcome'")
	}
	if action.Type != ActionCreate {
		t.Errorf("expected ActionCreate, got %s", action.Type)
	}
}

// TestPlanUpdateChannelTopic verifies an update action when topic changes.
func TestPlanUpdateChannelTopic(t *testing.T) {
	cfg := &config.Config{
		ServerID: "123",
		Roles:    map[string]config.RoleConfig{},
		Categories: map[string]config.CategoryConfig{
			"General": {
				Channels: map[string]config.ChannelConfig{
					"general": {Type: "text", Topic: "New topic"},
				},
			},
		},
	}

	st := state.NewState("123")
	st.SetCategory("General", "cat-111")
	st.SetChannel("General", "general", "ch-222")

	oldTopic := "Old topic"
	live := buildLiveState(
		nil,
		[]*discord.Channel{{ID: "cat-111", Type: discord.ChannelTypeGuildCategory, Name: "General"}},
		[]*discord.Channel{
			{ID: "ch-222", Type: discord.ChannelTypeGuildText, Name: "general", Topic: &oldTopic},
		},
	)

	p := NewPlanner(cfg, st, live)
	plan, err := p.Plan()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	action := findAction(plan, ResourceChannel, "General/general")
	if action == nil {
		t.Fatal("expected update action for channel 'General/general'")
	}
	if action.Type != ActionUpdate {
		t.Errorf("expected ActionUpdate, got %s", action.Type)
	}
	change := findFieldChange(action.Changes, "topic")
	if change == nil {
		t.Fatal("expected 'topic' field change")
	}
	if change.OldValue != "Old topic" || change.NewValue != "New topic" {
		t.Errorf("expected topic change 'Old topic'->'New topic', got %q->%q", change.OldValue, change.NewValue)
	}
}

// TestPlanUpdateChannelType verifies an update action when channel type changes.
func TestPlanUpdateChannelType(t *testing.T) {
	cfg := &config.Config{
		ServerID: "123",
		Roles:    map[string]config.RoleConfig{},
		Categories: map[string]config.CategoryConfig{
			"General": {
				Channels: map[string]config.ChannelConfig{
					"chan": {Type: "announcement"},
				},
			},
		},
	}

	st := state.NewState("123")
	st.SetCategory("General", "cat-111")
	st.SetChannel("General", "chan", "ch-222")

	live := buildLiveState(
		nil,
		[]*discord.Channel{{ID: "cat-111", Type: discord.ChannelTypeGuildCategory, Name: "General"}},
		[]*discord.Channel{
			{ID: "ch-222", Type: discord.ChannelTypeGuildText, Name: "chan"},
		},
	)

	p := NewPlanner(cfg, st, live)
	plan, err := p.Plan()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	action := findAction(plan, ResourceChannel, "General/chan")
	if action == nil {
		t.Fatal("expected update action for channel 'General/chan'")
	}
	change := findFieldChange(action.Changes, "type")
	if change == nil {
		t.Fatal("expected 'type' field change")
	}
	if change.NewValue != "announcement" {
		t.Errorf("expected new type 'announcement', got %q", change.NewValue)
	}
}

// TestPlanDeleteChannel verifies a delete action for a channel in state but not config.
func TestPlanDeleteChannel(t *testing.T) {
	cfg := &config.Config{
		ServerID: "123",
		Roles:    map[string]config.RoleConfig{},
		Categories: map[string]config.CategoryConfig{
			"General": {Channels: map[string]config.ChannelConfig{}},
		},
	}

	st := state.NewState("123")
	st.SetCategory("General", "cat-111")
	st.SetChannel("General", "old-channel", "ch-999")

	live := buildLiveState(
		nil,
		[]*discord.Channel{{ID: "cat-111", Type: discord.ChannelTypeGuildCategory, Name: "General"}},
		[]*discord.Channel{{ID: "ch-999", Type: discord.ChannelTypeGuildText, Name: "old-channel"}},
	)

	p := NewPlanner(cfg, st, live)
	plan, err := p.Plan()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	action := findAction(plan, ResourceChannel, "General/old-channel")
	if action == nil {
		t.Fatal("expected delete action for channel 'General/old-channel'")
	}
	if action.Type != ActionDelete {
		t.Errorf("expected ActionDelete, got %s", action.Type)
	}
}

// TestPlanDeleteChannelWhenCategoryDeleted verifies channels in a removed category get delete actions.
func TestPlanDeleteChannelWhenCategoryDeleted(t *testing.T) {
	cfg := &config.Config{
		ServerID:   "123",
		Roles:      map[string]config.RoleConfig{},
		Categories: map[string]config.CategoryConfig{},
	}

	st := state.NewState("123")
	st.SetCategory("OldCat", "cat-999")
	st.SetChannel("OldCat", "channel-a", "ch-111")
	st.SetChannel("OldCat", "channel-b", "ch-222")

	live := buildLiveState(
		nil,
		[]*discord.Channel{{ID: "cat-999", Type: discord.ChannelTypeGuildCategory, Name: "OldCat"}},
		[]*discord.Channel{
			{ID: "ch-111", Type: discord.ChannelTypeGuildText, Name: "channel-a"},
			{ID: "ch-222", Type: discord.ChannelTypeGuildText, Name: "channel-b"},
		},
	)

	p := NewPlanner(cfg, st, live)
	plan, err := p.Plan()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Expect: delete category + delete 2 channels
	if plan.ToDelete < 3 {
		t.Errorf("expected at least 3 deletes (category + 2 channels), got %d", plan.ToDelete)
	}

	actionA := findAction(plan, ResourceChannel, "OldCat/channel-a")
	if actionA == nil || actionA.Type != ActionDelete {
		t.Error("expected delete action for OldCat/channel-a")
	}
	actionB := findAction(plan, ResourceChannel, "OldCat/channel-b")
	if actionB == nil || actionB.Type != ActionDelete {
		t.Error("expected delete action for OldCat/channel-b")
	}
}

// TestPlanExternallyDeletedRole verifies that an externally deleted role gets a create action.
func TestPlanExternallyDeletedRole(t *testing.T) {
	cfg := &config.Config{
		ServerID: "123",
		Roles: map[string]config.RoleConfig{
			"admin": {Color: "#FF0000"},
		},
		Categories: map[string]config.CategoryConfig{},
	}

	st := state.NewState("123")
	st.SetRole("admin", "role-111") // in state

	// But NOT in live Discord (externally deleted)
	live := buildLiveState(nil, nil, nil)

	p := NewPlanner(cfg, st, live)
	plan, err := p.Plan()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	action := findAction(plan, ResourceRole, "admin")
	if action == nil {
		t.Fatal("expected create action for externally deleted role 'admin'")
	}
	if action.Type != ActionCreate {
		t.Errorf("expected ActionCreate for externally deleted role, got %s", action.Type)
	}
}

// TestPlanExternallyDeletedCategory verifies that an externally deleted category gets a create action.
func TestPlanExternallyDeletedCategory(t *testing.T) {
	cfg := &config.Config{
		ServerID: "123",
		Roles:    map[string]config.RoleConfig{},
		Categories: map[string]config.CategoryConfig{
			"General": {Channels: map[string]config.ChannelConfig{}},
		},
	}

	st := state.NewState("123")
	st.SetCategory("General", "cat-111") // in state but not in live

	live := buildLiveState(nil, nil, nil)

	p := NewPlanner(cfg, st, live)
	plan, err := p.Plan()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	action := findAction(plan, ResourceCategory, "General")
	if action == nil {
		t.Fatal("expected create action for externally deleted category 'General'")
	}
	if action.Type != ActionCreate {
		t.Errorf("expected ActionCreate, got %s", action.Type)
	}
}

// TestPlanExternallyDeletedChannel verifies that an externally deleted channel gets a create action.
func TestPlanExternallyDeletedChannel(t *testing.T) {
	cfg := &config.Config{
		ServerID: "123",
		Roles:    map[string]config.RoleConfig{},
		Categories: map[string]config.CategoryConfig{
			"General": {
				Channels: map[string]config.ChannelConfig{
					"welcome": {Type: "text"},
				},
			},
		},
	}

	st := state.NewState("123")
	st.SetCategory("General", "cat-111")
	st.SetChannel("General", "welcome", "ch-999") // in state but not in live

	live := buildLiveState(
		nil,
		[]*discord.Channel{{ID: "cat-111", Type: discord.ChannelTypeGuildCategory, Name: "General"}},
		nil, // channel NOT in live
	)

	p := NewPlanner(cfg, st, live)
	plan, err := p.Plan()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	action := findAction(plan, ResourceChannel, "General/welcome")
	if action == nil {
		t.Fatal("expected create action for externally deleted channel")
	}
	if action.Type != ActionCreate {
		t.Errorf("expected ActionCreate, got %s", action.Type)
	}
}

// TestPlanMixedChanges verifies a plan with multiple creates, updates, and deletes.
func TestPlanMixedChanges(t *testing.T) {
	cfg := &config.Config{
		ServerID: "123",
		Roles: map[string]config.RoleConfig{
			"admin":  {Color: "#FF0000", Permissions: []string{"administrator"}},
			"member": {Color: "#00AA00"}, // new role
		},
		Categories: map[string]config.CategoryConfig{
			"General": {
				Position: 0,
				Channels: map[string]config.ChannelConfig{
					"general":  {Type: "text", Topic: "Updated topic"},
					"new-chan": {Type: "text"},
				},
			},
		},
	}

	st := state.NewState("123")
	st.SetRole("admin", "role-111")
	st.SetRole("oldmod", "role-old") // will be deleted
	st.SetCategory("General", "cat-111")
	st.SetChannel("General", "general", "ch-111")
	st.SetChannel("General", "old-channel", "ch-old") // will be deleted

	oldTopic := "Old topic"
	live := buildLiveState(
		[]*discord.Role{
			{ID: "role-111", Name: "admin", Color: 0xFF0000, Permissions: "8"},
			{ID: "role-old", Name: "oldmod", Permissions: "0"},
		},
		[]*discord.Channel{
			{ID: "cat-111", Type: discord.ChannelTypeGuildCategory, Name: "General", Position: 0},
		},
		[]*discord.Channel{
			{ID: "ch-111", Type: discord.ChannelTypeGuildText, Name: "general", Topic: &oldTopic},
			{ID: "ch-old", Type: discord.ChannelTypeGuildText, Name: "old-channel"},
		},
	)

	p := NewPlanner(cfg, st, live)
	plan, err := p.Plan()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !plan.HasChanges() {
		t.Fatal("expected changes in plan")
	}

	// Creates: member role + new-chan channel
	if plan.ToCreate < 2 {
		t.Errorf("expected at least 2 creates, got %d", plan.ToCreate)
	}
	// Updates: general channel topic
	if plan.ToUpdate < 1 {
		t.Errorf("expected at least 1 update, got %d", plan.ToUpdate)
	}
	// Deletes: oldmod role + old-channel
	if plan.ToDelete < 2 {
		t.Errorf("expected at least 2 deletes, got %d", plan.ToDelete)
	}
}

// TestPlanHasChanges verifies HasChanges returns true when there are actions.
func TestPlanHasChanges(t *testing.T) {
	plan := &Plan{
		Actions:  []Action{{Type: ActionCreate, ResourceType: ResourceRole, Name: "admin"}},
		ToCreate: 1,
	}
	if !plan.HasChanges() {
		t.Error("expected HasChanges=true")
	}
}

// TestPlanNoChangesHasChanges verifies HasChanges returns false for an empty plan.
func TestPlanNoChangesHasChanges(t *testing.T) {
	plan := &Plan{}
	if plan.HasChanges() {
		t.Error("expected HasChanges=false for empty plan")
	}
}

// TestPlanSummary verifies the summary string format.
func TestPlanSummary(t *testing.T) {
	tests := []struct {
		plan     Plan
		expected string
	}{
		{
			plan:     Plan{ToCreate: 3, ToUpdate: 2, ToDelete: 1},
			expected: "Plan: 3 to add, 2 to change, 1 to destroy.",
		},
		{
			plan:     Plan{ToCreate: 0, ToUpdate: 0, ToDelete: 0},
			expected: "Plan: 0 to add, 0 to change, 0 to destroy.",
		},
		{
			plan:     Plan{ToCreate: 1, ToUpdate: 0, ToDelete: 0},
			expected: "Plan: 1 to add, 0 to change, 0 to destroy.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			got := tt.plan.Summary()
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

// TestPlanUpdateRoleHoist verifies hoist field change detection.
func TestPlanUpdateRoleHoist(t *testing.T) {
	cfg := &config.Config{
		ServerID: "123",
		Roles: map[string]config.RoleConfig{
			"mod": {Hoist: true, Permissions: []string{}},
		},
		Categories: map[string]config.CategoryConfig{},
	}

	st := state.NewState("123")
	st.SetRole("mod", "role-111")

	live := buildLiveState(
		[]*discord.Role{{ID: "role-111", Name: "mod", Hoist: false, Permissions: "0"}},
		nil, nil,
	)

	p := NewPlanner(cfg, st, live)
	plan, err := p.Plan()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	action := findAction(plan, ResourceRole, "mod")
	if action == nil || action.Type != ActionUpdate {
		t.Fatal("expected update action for role 'mod'")
	}
	change := findFieldChange(action.Changes, "hoist")
	if change == nil {
		t.Fatal("expected hoist field change")
	}
	if change.OldValue != "false" || change.NewValue != "true" {
		t.Errorf("expected hoist false->true, got %q->%q", change.OldValue, change.NewValue)
	}
}

// TestPlanUpdateRoleMentionable verifies mentionable field change detection.
func TestPlanUpdateRoleMentionable(t *testing.T) {
	cfg := &config.Config{
		ServerID: "123",
		Roles: map[string]config.RoleConfig{
			"mod": {Mentionable: false, Permissions: []string{}},
		},
		Categories: map[string]config.CategoryConfig{},
	}

	st := state.NewState("123")
	st.SetRole("mod", "role-111")

	live := buildLiveState(
		[]*discord.Role{{ID: "role-111", Name: "mod", Mentionable: true, Permissions: "0"}},
		nil, nil,
	)

	p := NewPlanner(cfg, st, live)
	plan, err := p.Plan()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	action := findAction(plan, ResourceRole, "mod")
	if action == nil || action.Type != ActionUpdate {
		t.Fatal("expected update action for role 'mod'")
	}
	change := findFieldChange(action.Changes, "mentionable")
	if change == nil {
		t.Fatal("expected mentionable field change")
	}
}

// TestPlanUpdateRolePermissions verifies permission bitfield change detection.
func TestPlanUpdateRolePermissions(t *testing.T) {
	cfg := &config.Config{
		ServerID: "123",
		Roles: map[string]config.RoleConfig{
			"mod": {Permissions: []string{"kick_members"}},
		},
		Categories: map[string]config.CategoryConfig{},
	}

	st := state.NewState("123")
	st.SetRole("mod", "role-111")

	// Permissions "0" = no permissions
	live := buildLiveState(
		[]*discord.Role{{ID: "role-111", Name: "mod", Permissions: "0"}},
		nil, nil,
	)

	p := NewPlanner(cfg, st, live)
	plan, err := p.Plan()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	action := findAction(plan, ResourceRole, "mod")
	if action == nil || action.Type != ActionUpdate {
		t.Fatal("expected update action for role 'mod'")
	}
	change := findFieldChange(action.Changes, "permissions")
	if change == nil {
		t.Fatal("expected permissions field change")
	}
	if change.OldValue != "0" {
		t.Errorf("expected old permissions '0', got %q", change.OldValue)
	}
}

// TestPlanUpdateChannelNSFW verifies NSFW field change detection.
func TestPlanUpdateChannelNSFW(t *testing.T) {
	cfg := &config.Config{
		ServerID: "123",
		Roles:    map[string]config.RoleConfig{},
		Categories: map[string]config.CategoryConfig{
			"General": {
				Channels: map[string]config.ChannelConfig{
					"adult-chat": {Type: "text", NSFW: true},
				},
			},
		},
	}

	st := state.NewState("123")
	st.SetCategory("General", "cat-111")
	st.SetChannel("General", "adult-chat", "ch-222")

	live := buildLiveState(
		nil,
		[]*discord.Channel{{ID: "cat-111", Type: discord.ChannelTypeGuildCategory, Name: "General"}},
		[]*discord.Channel{
			{ID: "ch-222", Type: discord.ChannelTypeGuildText, Name: "adult-chat", NSFW: false},
		},
	)

	p := NewPlanner(cfg, st, live)
	plan, err := p.Plan()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	action := findAction(plan, ResourceChannel, "General/adult-chat")
	if action == nil || action.Type != ActionUpdate {
		t.Fatal("expected update action for channel")
	}
	change := findFieldChange(action.Changes, "nsfw")
	if change == nil {
		t.Fatal("expected nsfw field change")
	}
}

// TestPlanUpdateChannelSlowmode verifies slowmode field change detection.
func TestPlanUpdateChannelSlowmode(t *testing.T) {
	cfg := &config.Config{
		ServerID: "123",
		Roles:    map[string]config.RoleConfig{},
		Categories: map[string]config.CategoryConfig{
			"General": {
				Channels: map[string]config.ChannelConfig{
					"slow": {Type: "text", SlowMode: 10},
				},
			},
		},
	}

	st := state.NewState("123")
	st.SetCategory("General", "cat-111")
	st.SetChannel("General", "slow", "ch-222")

	live := buildLiveState(
		nil,
		[]*discord.Channel{{ID: "cat-111", Type: discord.ChannelTypeGuildCategory, Name: "General"}},
		[]*discord.Channel{
			{ID: "ch-222", Type: discord.ChannelTypeGuildText, Name: "slow", RateLimitPerUser: 0},
		},
	)

	p := NewPlanner(cfg, st, live)
	plan, err := p.Plan()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	action := findAction(plan, ResourceChannel, "General/slow")
	if action == nil || action.Type != ActionUpdate {
		t.Fatal("expected update action for slowmode channel")
	}
	change := findFieldChange(action.Changes, "slowmode")
	if change == nil {
		t.Fatal("expected slowmode field change")
	}
	if change.NewValue != "10" {
		t.Errorf("expected new slowmode '10', got %q", change.NewValue)
	}
}

// TestPlanUpdateVoiceChannelBitrate verifies bitrate field change detection for voice channels.
func TestPlanUpdateVoiceChannelBitrate(t *testing.T) {
	cfg := &config.Config{
		ServerID: "123",
		Roles:    map[string]config.RoleConfig{},
		Categories: map[string]config.CategoryConfig{
			"Voice": {
				Channels: map[string]config.ChannelConfig{
					"music": {Type: "voice", Bitrate: 128000},
				},
			},
		},
	}

	st := state.NewState("123")
	st.SetCategory("Voice", "cat-111")
	st.SetChannel("Voice", "music", "ch-222")

	live := buildLiveState(
		nil,
		[]*discord.Channel{{ID: "cat-111", Type: discord.ChannelTypeGuildCategory, Name: "Voice"}},
		[]*discord.Channel{
			{ID: "ch-222", Type: discord.ChannelTypeGuildVoice, Name: "music", Bitrate: 64000},
		},
	)

	p := NewPlanner(cfg, st, live)
	plan, err := p.Plan()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	action := findAction(plan, ResourceChannel, "Voice/music")
	if action == nil || action.Type != ActionUpdate {
		t.Fatal("expected update action for voice channel")
	}
	change := findFieldChange(action.Changes, "bitrate")
	if change == nil {
		t.Fatal("expected bitrate field change")
	}
	if change.NewValue != "128000" {
		t.Errorf("expected new bitrate '128000', got %q", change.NewValue)
	}
}

// TestPlanUpdateVoiceChannelUserLimit verifies user_limit change detection.
func TestPlanUpdateVoiceChannelUserLimit(t *testing.T) {
	cfg := &config.Config{
		ServerID: "123",
		Roles:    map[string]config.RoleConfig{},
		Categories: map[string]config.CategoryConfig{
			"Voice": {
				Channels: map[string]config.ChannelConfig{
					"limited": {Type: "voice", UserLimit: 5},
				},
			},
		},
	}

	st := state.NewState("123")
	st.SetCategory("Voice", "cat-111")
	st.SetChannel("Voice", "limited", "ch-222")

	live := buildLiveState(
		nil,
		[]*discord.Channel{{ID: "cat-111", Type: discord.ChannelTypeGuildCategory, Name: "Voice"}},
		[]*discord.Channel{
			{ID: "ch-222", Type: discord.ChannelTypeGuildVoice, Name: "limited", UserLimit: 0},
		},
	)

	p := NewPlanner(cfg, st, live)
	plan, err := p.Plan()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	action := findAction(plan, ResourceChannel, "Voice/limited")
	if action == nil || action.Type != ActionUpdate {
		t.Fatal("expected update action for voice channel")
	}
	change := findFieldChange(action.Changes, "user_limit")
	if change == nil {
		t.Fatal("expected user_limit field change")
	}
}

// TestPlanSortedOutput verifies plan actions are generated in sorted key order.
func TestPlanSortedOutput(t *testing.T) {
	cfg := &config.Config{
		ServerID: "123",
		Roles: map[string]config.RoleConfig{
			"zrole": {},
			"arole": {},
			"mrole": {},
		},
		Categories: map[string]config.CategoryConfig{},
	}

	st := state.NewState("123")
	live := buildLiveState(nil, nil, nil)

	p := NewPlanner(cfg, st, live)
	plan, err := p.Plan()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// All roles should be creates, in sorted order
	var names []string
	for _, a := range plan.Actions {
		if a.ResourceType == ResourceRole {
			names = append(names, a.Name)
		}
	}
	if len(names) != 3 {
		t.Fatalf("expected 3 role actions, got %d", len(names))
	}
	if names[0] != "arole" || names[1] != "mrole" || names[2] != "zrole" {
		t.Errorf("expected sorted role order [arole mrole zrole], got %v", names)
	}
}

// TestChannelTypeToInt verifies conversion of channel type strings.
func TestChannelTypeToInt(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"text", discord.ChannelTypeGuildText},
		{"voice", discord.ChannelTypeGuildVoice},
		{"announcement", discord.ChannelTypeGuildAnnouncement},
		{"stage", discord.ChannelTypeGuildStage},
		{"forum", discord.ChannelTypeGuildForum},
		{"unknown", discord.ChannelTypeGuildText}, // default
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := channelTypeToInt(tt.input)
			if got != tt.expected {
				t.Errorf("channelTypeToInt(%q) = %d, want %d", tt.input, got, tt.expected)
			}
		})
	}
}

// TestParseChannelKey verifies channel key parsing.
func TestParseChannelKey(t *testing.T) {
	tests := []struct {
		key     string
		cat     string
		ch      string
		wantErr bool
	}{
		{"General/welcome", "General", "welcome", false},
		{"Voice/general-voice", "Voice", "general-voice", false},
		{"noslash", "", "", true},
		{"A/B", "A", "B", false},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			cat, ch, err := parseChannelKey(tt.key)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if cat != tt.cat || ch != tt.ch {
				t.Errorf("parseChannelKey(%q) = (%q, %q), want (%q, %q)", tt.key, cat, ch, tt.cat, tt.ch)
			}
		})
	}
}

// TestPlanMultipleCategories verifies correct handling of multiple categories.
func TestPlanMultipleCategories(t *testing.T) {
	cfg := &config.Config{
		ServerID: "123",
		Roles:    map[string]config.RoleConfig{},
		Categories: map[string]config.CategoryConfig{
			"General": {
				Channels: map[string]config.ChannelConfig{
					"general": {Type: "text"},
				},
			},
			"Voice": {
				Channels: map[string]config.ChannelConfig{
					"music": {Type: "voice"},
				},
			},
		},
	}

	st := state.NewState("123")
	live := buildLiveState(nil, nil, nil)

	p := NewPlanner(cfg, st, live)
	plan, err := p.Plan()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 2 categories + 2 channels to create
	if plan.ToCreate != 4 {
		t.Errorf("expected 4 creates, got %d", plan.ToCreate)
	}

	if !strings.Contains(plan.Summary(), "4 to add") {
		t.Errorf("unexpected summary: %s", plan.Summary())
	}
}

// --- Settings tests ---

// TestPlanSettingsUpdate verifies that differing settings produce an update action.
func TestPlanSettingsUpdate(t *testing.T) {
	cfg := &config.Config{
		ServerID: "guild-123",
		Settings: &config.ServerSettings{
			VerificationLevel:           "high",
			ExplicitContentFilter:       "all_members",
			DefaultMessageNotifications: "only_mentions",
			AFKTimeout:                  300,
		},
	}

	st := state.NewState("guild-123")
	live := &LiveState{
		Guild: &discord.Guild{
			ID:                          "guild-123",
			Name:                        "Test",
			VerificationLevel:           1, // low
			ExplicitContentFilter:       0, // disabled
			DefaultMessageNotifications: 0, // all_messages
			AFKTimeout:                  0,
		},
		Roles:      make(map[string]*discord.Role),
		Categories: make(map[string]*discord.Channel),
		Channels:   make(map[string]*discord.Channel),
	}

	p := NewPlanner(cfg, st, live)
	plan, err := p.Plan()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	action := findAction(plan, ResourceSettings, "server_settings")
	if action == nil {
		t.Fatal("expected update action for server_settings")
	}
	if action.Type != ActionUpdate {
		t.Errorf("expected ActionUpdate, got %s", action.Type)
	}
	if action.DiscordID != "guild-123" {
		t.Errorf("expected DiscordID 'guild-123', got %q", action.DiscordID)
	}
	change := findFieldChange(action.Changes, "verification_level")
	if change == nil {
		t.Fatal("expected verification_level change")
	}
	if change.NewValue != "high" {
		t.Errorf("expected new verification_level 'high', got %q", change.NewValue)
	}
}

// TestPlanSettingsNoChange verifies no action when settings match live.
func TestPlanSettingsNoChange(t *testing.T) {
	cfg := &config.Config{
		ServerID: "guild-123",
		Settings: &config.ServerSettings{
			VerificationLevel: "medium",
		},
	}

	st := state.NewState("guild-123")
	live := &LiveState{
		Guild: &discord.Guild{
			ID:                "guild-123",
			VerificationLevel: 2, // medium
		},
		Roles:      make(map[string]*discord.Role),
		Categories: make(map[string]*discord.Channel),
		Channels:   make(map[string]*discord.Channel),
	}

	p := NewPlanner(cfg, st, live)
	plan, err := p.Plan()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	action := findAction(plan, ResourceSettings, "server_settings")
	if action != nil {
		t.Errorf("expected no settings action, got: %+v", action)
	}
}

// TestPlanSettingsNil verifies no action when settings are nil in config.
func TestPlanSettingsNil(t *testing.T) {
	cfg := &config.Config{
		ServerID: "guild-123",
		Settings: nil,
	}

	st := state.NewState("guild-123")
	live := &LiveState{
		Guild: &discord.Guild{
			ID:                "guild-123",
			VerificationLevel: 2,
		},
		Roles:      make(map[string]*discord.Role),
		Categories: make(map[string]*discord.Channel),
		Channels:   make(map[string]*discord.Channel),
	}

	p := NewPlanner(cfg, st, live)
	plan, err := p.Plan()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	action := findAction(plan, ResourceSettings, "server_settings")
	if action != nil {
		t.Errorf("expected no settings action when settings nil, got: %+v", action)
	}
}

// --- @everyone role tests ---

// TestPlanEveryoneRoleUpdate verifies @everyone permissions differ → update with guild ID.
func TestPlanEveryoneRoleUpdate(t *testing.T) {
	cfg := &config.Config{
		ServerID: "guild-123",
		Roles: map[string]config.RoleConfig{
			"@everyone": {Permissions: []string{"view_channel", "send_messages"}},
		},
	}

	st := state.NewState("guild-123")
	// @everyone role ID == guild ID in Discord.
	live := &LiveState{
		Roles: map[string]*discord.Role{
			"guild-123": {ID: "guild-123", Name: "@everyone", Permissions: "0"},
		},
		Categories: make(map[string]*discord.Channel),
		Channels:   make(map[string]*discord.Channel),
	}

	p := NewPlanner(cfg, st, live)
	plan, err := p.Plan()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	action := findAction(plan, ResourceRole, "@everyone")
	if action == nil {
		t.Fatal("expected update action for @everyone")
	}
	if action.Type != ActionUpdate {
		t.Errorf("expected ActionUpdate, got %s", action.Type)
	}
	if action.DiscordID != "guild-123" {
		t.Errorf("expected DiscordID == guild ID 'guild-123', got %q", action.DiscordID)
	}
	change := findFieldChange(action.Changes, "permissions")
	if change == nil {
		t.Fatal("expected permissions change for @everyone")
	}
}

// TestPlanEveryoneRoleNoChange verifies no action when @everyone permissions match.
func TestPlanEveryoneRoleNoChange(t *testing.T) {
	// view_channel (1<<10=1024) | send_messages (1<<11=2048) = 3072
	cfg := &config.Config{
		ServerID: "guild-123",
		Roles: map[string]config.RoleConfig{
			"@everyone": {Permissions: []string{"view_channel", "send_messages"}},
		},
	}

	st := state.NewState("guild-123")
	live := &LiveState{
		Roles: map[string]*discord.Role{
			"guild-123": {ID: "guild-123", Name: "@everyone", Permissions: "3072"},
		},
		Categories: make(map[string]*discord.Channel),
		Channels:   make(map[string]*discord.Channel),
	}

	p := NewPlanner(cfg, st, live)
	plan, err := p.Plan()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	action := findAction(plan, ResourceRole, "@everyone")
	if action != nil {
		t.Errorf("expected no action for @everyone when permissions match, got: %+v", action)
	}
}

// TestPlanEveryoneRoleNotDeleted verifies @everyone is never in the delete list.
func TestPlanEveryoneRoleNotDeleted(t *testing.T) {
	// @everyone is in state but NOT in config.Roles — it should NOT be deleted.
	cfg := &config.Config{
		ServerID: "guild-123",
		Roles:    map[string]config.RoleConfig{},
	}

	st := state.NewState("guild-123")
	st.SetRole("@everyone", "guild-123")

	live := &LiveState{
		Roles: map[string]*discord.Role{
			"guild-123": {ID: "guild-123", Name: "@everyone", Permissions: "0"},
		},
		Categories: make(map[string]*discord.Channel),
		Channels:   make(map[string]*discord.Channel),
	}

	p := NewPlanner(cfg, st, live)
	plan, err := p.Plan()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, a := range plan.Actions {
		if a.ResourceType == ResourceRole && a.Name == "@everyone" && a.Type == ActionDelete {
			t.Error("@everyone should never be in the delete list")
		}
	}
}

// --- Top-level channel tests ---

// TestPlanCreateTopLevelChannel verifies create action for a channel in config.Channels but not in state.
func TestPlanCreateTopLevelChannel(t *testing.T) {
	cfg := &config.Config{
		ServerID:   "guild-123",
		Channels:   map[string]config.ChannelConfig{"announcements": {Type: "announcement"}},
		Categories: map[string]config.CategoryConfig{},
	}

	st := state.NewState("guild-123")
	live := &LiveState{
		Roles:      make(map[string]*discord.Role),
		Categories: make(map[string]*discord.Channel),
		Channels:   make(map[string]*discord.Channel),
	}

	p := NewPlanner(cfg, st, live)
	plan, err := p.Plan()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	action := findAction(plan, ResourceChannel, "/announcements")
	if action == nil {
		t.Fatal("expected create action for top-level channel '/announcements'")
	}
	if action.Type != ActionCreate {
		t.Errorf("expected ActionCreate, got %s", action.Type)
	}
}

// TestPlanUpdateTopLevelChannel verifies update action when top-level channel topic changes.
func TestPlanUpdateTopLevelChannel(t *testing.T) {
	cfg := &config.Config{
		ServerID:   "guild-123",
		Channels:   map[string]config.ChannelConfig{"announcements": {Type: "announcement", Topic: "New topic"}},
		Categories: map[string]config.CategoryConfig{},
	}

	st := state.NewState("guild-123")
	st.SetChannel("", "announcements", "ch-999")

	oldTopic := "Old topic"
	live := &LiveState{
		Roles:      make(map[string]*discord.Role),
		Categories: make(map[string]*discord.Channel),
		Channels: map[string]*discord.Channel{
			"ch-999": {ID: "ch-999", Type: discord.ChannelTypeGuildAnnouncement, Name: "announcements", Topic: &oldTopic},
		},
	}

	p := NewPlanner(cfg, st, live)
	plan, err := p.Plan()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	action := findAction(plan, ResourceChannel, "/announcements")
	if action == nil {
		t.Fatal("expected update action for top-level channel '/announcements'")
	}
	if action.Type != ActionUpdate {
		t.Errorf("expected ActionUpdate, got %s", action.Type)
	}
	change := findFieldChange(action.Changes, "topic")
	if change == nil {
		t.Fatal("expected topic field change")
	}
	if change.OldValue != "Old topic" || change.NewValue != "New topic" {
		t.Errorf("expected topic change 'Old topic'->'New topic', got %q->%q", change.OldValue, change.NewValue)
	}
}

// TestPlanDeleteTopLevelChannel verifies delete action for a top-level channel in state but not config.
func TestPlanDeleteTopLevelChannel(t *testing.T) {
	cfg := &config.Config{
		ServerID:   "guild-123",
		Channels:   map[string]config.ChannelConfig{},
		Categories: map[string]config.CategoryConfig{},
	}

	st := state.NewState("guild-123")
	st.SetChannel("", "old-chan", "ch-888")

	live := &LiveState{
		Roles:      make(map[string]*discord.Role),
		Categories: make(map[string]*discord.Channel),
		Channels: map[string]*discord.Channel{
			"ch-888": {ID: "ch-888", Type: discord.ChannelTypeGuildText, Name: "old-chan"},
		},
	}

	p := NewPlanner(cfg, st, live)
	plan, err := p.Plan()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	action := findAction(plan, ResourceChannel, "/old-chan")
	if action == nil {
		t.Fatal("expected delete action for top-level channel '/old-chan'")
	}
	if action.Type != ActionDelete {
		t.Errorf("expected ActionDelete, got %s", action.Type)
	}
	if action.DiscordID != "ch-888" {
		t.Errorf("expected DiscordID 'ch-888', got %q", action.DiscordID)
	}
}
