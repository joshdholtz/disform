// Package integration_test wires together the real planner, applier, and
// discord.HTTPClient against a fake Discord HTTP server. This catches bugs
// that neither layer's unit tests can find: wrong JSON field names, incorrect
// HTTP methods/paths, bad permission bitfield math, and wrong apply ordering.
package integration_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/joshholtz/disform/internal/applier"
	"github.com/joshholtz/disform/internal/config"
	"github.com/joshholtz/disform/internal/discord"
	"github.com/joshholtz/disform/internal/planner"
	"github.com/joshholtz/disform/internal/state"
)

// --- Fake Discord server ---

type recordedRequest struct {
	Method string
	Path   string
	Body   json.RawMessage
}

// fakeDiscord is a minimal fake Discord REST API for integration testing.
type fakeDiscord struct {
	t        *testing.T
	srv      *httptest.Server
	requests []recordedRequest
	counter  int
}

func newFakeDiscord(t *testing.T) *fakeDiscord {
	t.Helper()
	fd := &fakeDiscord{t: t, counter: 100}
	fd.srv = httptest.NewServer(http.HandlerFunc(fd.handle))
	t.Cleanup(fd.srv.Close)
	return fd
}

func (fd *fakeDiscord) client() discord.Client {
	return discord.NewHTTPClientWithBase("test-token", fd.srv.URL)
}

func (fd *fakeDiscord) nextID() string {
	fd.counter++
	return fmt.Sprintf("%d", fd.counter)
}

// handle routes incoming requests and returns plausible responses.
func (fd *fakeDiscord) handle(w http.ResponseWriter, r *http.Request) {
	var body json.RawMessage
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&body)
	}
	fd.requests = append(fd.requests, recordedRequest{
		Method: r.Method,
		Path:   r.URL.Path,
		Body:   body,
	})

	w.Header().Set("Content-Type", "application/json")
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")

	switch {
	// POST /guilds/{id}/roles
	case r.Method == http.MethodPost && len(parts) == 3 && parts[0] == "guilds" && parts[2] == "roles":
		var p discord.CreateRoleParams
		_ = json.Unmarshal(body, &p)
		_ = json.NewEncoder(w).Encode(discord.Role{ID: fd.nextID(), Name: p.Name, Color: p.Color, Hoist: p.Hoist, Mentionable: p.Mentionable, Permissions: p.Permissions})

	// PATCH /guilds/{id}/roles/{roleID}
	case r.Method == http.MethodPatch && len(parts) == 4 && parts[0] == "guilds" && parts[2] == "roles":
		var p discord.UpdateRoleParams
		_ = json.Unmarshal(body, &p)
		role := discord.Role{ID: parts[3]}
		if p.Name != nil {
			role.Name = *p.Name
		}
		_ = json.NewEncoder(w).Encode(role)

	// DELETE /guilds/{id}/roles/{roleID}
	case r.Method == http.MethodDelete && len(parts) == 4 && parts[0] == "guilds" && parts[2] == "roles":
		w.WriteHeader(http.StatusNoContent)

	// POST /guilds/{id}/channels
	case r.Method == http.MethodPost && len(parts) == 3 && parts[0] == "guilds" && parts[2] == "channels":
		var p discord.CreateChannelParams
		_ = json.Unmarshal(body, &p)
		_ = json.NewEncoder(w).Encode(discord.Channel{ID: fd.nextID(), Name: p.Name, Type: p.Type})

	// PATCH /channels/{id}  (no sub-path)
	case r.Method == http.MethodPatch && len(parts) == 2 && parts[0] == "channels":
		var p discord.UpdateChannelParams
		_ = json.Unmarshal(body, &p)
		ch := discord.Channel{ID: parts[1]}
		if p.Name != nil {
			ch.Name = *p.Name
		}
		_ = json.NewEncoder(w).Encode(ch)

	// DELETE /channels/{id}
	case r.Method == http.MethodDelete && len(parts) == 2 && parts[0] == "channels":
		w.WriteHeader(http.StatusNoContent)

	// PUT /channels/{id}/permissions/{overwriteID}
	case r.Method == http.MethodPut && len(parts) == 4 && parts[0] == "channels" && parts[2] == "permissions":
		w.WriteHeader(http.StatusNoContent)

	// PATCH /guilds/{id}  (modify guild settings — no sub-path)
	case r.Method == http.MethodPatch && len(parts) == 2 && parts[0] == "guilds":
		_ = json.NewEncoder(w).Encode(discord.Guild{ID: parts[1]})

	default:
		fd.t.Errorf("unhandled fake Discord request: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	}
}

// findRequests returns all recorded requests matching the method and path substring.
func (fd *fakeDiscord) findRequests(method, pathSubstr string) []recordedRequest {
	var out []recordedRequest
	for _, req := range fd.requests {
		if req.Method == method && strings.Contains(req.Path, pathSubstr) {
			out = append(out, req)
		}
	}
	return out
}

// requestOrder returns "METHOD /path" for every recorded request, in order.
func (fd *fakeDiscord) requestOrder() []string {
	out := make([]string, len(fd.requests))
	for i, req := range fd.requests {
		out[i] = req.Method + " " + req.Path
	}
	return out
}

// --- Helpers ---

func buildLive(guild *discord.Guild, roles []*discord.Role, cats []*discord.Channel, chans []*discord.Channel) *planner.LiveState {
	ls := &planner.LiveState{
		Guild:      guild,
		Roles:      make(map[string]*discord.Role),
		Categories: make(map[string]*discord.Channel),
		Channels:   make(map[string]*discord.Channel),
	}
	for _, r := range roles {
		ls.Roles[r.ID] = r
	}
	for _, c := range cats {
		ls.Categories[c.ID] = c
	}
	for _, c := range chans {
		ls.Channels[c.ID] = c
	}
	return ls
}

func mustPlan(t *testing.T, cfg *config.Config, st *state.State, live *planner.LiveState) *planner.Plan {
	t.Helper()
	plan, err := planner.NewPlanner(cfg, st, live).Plan()
	if err != nil {
		t.Fatalf("plan error: %v", err)
	}
	return plan
}

func mustApply(t *testing.T, fd *fakeDiscord, cfg *config.Config, st *state.State, plan *planner.Plan) {
	t.Helper()
	if err := applier.NewApplier(fd.client(), st, cfg).Apply(plan); err != nil {
		t.Fatalf("apply error: %v", err)
	}
}

// --- Tests ---

// TestIntegrationCreateRolePayload verifies the full JSON body sent to Discord when creating a role.
func TestIntegrationCreateRolePayload(t *testing.T) {
	fd := newFakeDiscord(t)
	cfg := &config.Config{
		ServerID: "guild-1",
		Roles: map[string]config.RoleConfig{
			"admin": {Color: "#FF0000", Hoist: true, Mentionable: true, Permissions: []string{"administrator"}},
		},
		Categories: map[string]config.CategoryConfig{},
	}
	st := state.NewState("guild-1")
	live := buildLive(&discord.Guild{ID: "guild-1"}, nil, nil, nil)

	mustApply(t, fd, cfg, st, mustPlan(t, cfg, st, live))

	reqs := fd.findRequests(http.MethodPost, "/guilds/guild-1/roles")
	if len(reqs) != 1 {
		t.Fatalf("expected 1 POST /roles, got %d", len(reqs))
	}

	var body discord.CreateRoleParams
	if err := json.Unmarshal(reqs[0].Body, &body); err != nil {
		t.Fatalf("could not parse role body: %v", err)
	}
	if body.Name != "admin" {
		t.Errorf("expected name 'admin', got %q", body.Name)
	}
	if body.Color != 0xFF0000 {
		t.Errorf("expected color 0xFF0000, got %d", body.Color)
	}
	if !body.Hoist {
		t.Error("expected hoist=true")
	}
	if !body.Mentionable {
		t.Error("expected mentionable=true")
	}
	// administrator = 1<<3 = 8
	if body.Permissions != "8" {
		t.Errorf("expected permissions '8' (administrator), got %q", body.Permissions)
	}
}

// TestIntegrationDisplayNameSentToDiscord verifies the name: field (not the YAML key) is sent to Discord.
func TestIntegrationDisplayNameSentToDiscord(t *testing.T) {
	fd := newFakeDiscord(t)
	cfg := &config.Config{
		ServerID: "guild-1",
		Roles: map[string]config.RoleConfig{
			"admin-role": {Name: "Admin", Permissions: []string{}},
		},
		Categories: map[string]config.CategoryConfig{
			"general-cat": {
				Name:     "General",
				Position: 0,
				Channels: map[string]config.ChannelConfig{
					"welcome-chan": {Name: "welcome", Type: "text"},
				},
			},
		},
	}
	st := state.NewState("guild-1")
	live := buildLive(&discord.Guild{ID: "guild-1"}, nil, nil, nil)

	mustApply(t, fd, cfg, st, mustPlan(t, cfg, st, live))

	// Role should be created as "Admin", not "admin-role".
	roleReqs := fd.findRequests(http.MethodPost, "/guilds/guild-1/roles")
	if len(roleReqs) != 1 {
		t.Fatalf("expected 1 POST /roles, got %d", len(roleReqs))
	}
	var roleBody discord.CreateRoleParams
	_ = json.Unmarshal(roleReqs[0].Body, &roleBody)
	if roleBody.Name != "Admin" {
		t.Errorf("expected Discord role name 'Admin', got %q", roleBody.Name)
	}

	// Category should be created as "General", not "general-cat".
	chanReqs := fd.findRequests(http.MethodPost, "/guilds/guild-1/channels")
	if len(chanReqs) < 2 {
		t.Fatalf("expected at least 2 POST /channels (category + channel), got %d", len(chanReqs))
	}
	var catBody discord.CreateChannelParams
	_ = json.Unmarshal(chanReqs[0].Body, &catBody)
	if catBody.Name != "General" {
		t.Errorf("expected category name 'General', got %q", catBody.Name)
	}

	// Channel should be created as "welcome", not "welcome-chan".
	var chanBody discord.CreateChannelParams
	_ = json.Unmarshal(chanReqs[1].Body, &chanBody)
	if chanBody.Name != "welcome" {
		t.Errorf("expected channel name 'welcome', got %q", chanBody.Name)
	}
}

// TestIntegrationChannelCreatedWithParentID verifies that a channel inside a category
// is created with the category's Discord ID as parent_id.
func TestIntegrationChannelCreatedWithParentID(t *testing.T) {
	fd := newFakeDiscord(t)
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
	live := buildLive(&discord.Guild{ID: "guild-1"}, nil, nil, nil)

	mustApply(t, fd, cfg, st, mustPlan(t, cfg, st, live))

	chanReqs := fd.findRequests(http.MethodPost, "/guilds/guild-1/channels")
	if len(chanReqs) < 2 {
		t.Fatalf("expected 2 POST /channels (category + channel), got %d", len(chanReqs))
	}

	// First call creates the category and returns an ID.
	var catBody discord.CreateChannelParams
	_ = json.Unmarshal(chanReqs[0].Body, &catBody)
	if catBody.Type != discord.ChannelTypeGuildCategory {
		t.Errorf("expected first POST to be a category, type=%d", catBody.Type)
	}

	// Second call creates the channel and should reference the category ID.
	catID, ok := st.GetCategoryID("General")
	if !ok || catID == "" {
		t.Fatal("expected 'General' category in state after apply")
	}
	var chBody discord.CreateChannelParams
	_ = json.Unmarshal(chanReqs[1].Body, &chBody)
	if chBody.ParentID != catID {
		t.Errorf("expected channel parent_id %q, got %q", catID, chBody.ParentID)
	}
}

// TestIntegrationPermissionBitfields verifies that permission names are correctly
// converted to bitfield integers in the request body sent to Discord.
// For new channels, permissions are embedded as permission_overwrites in the POST body.
func TestIntegrationPermissionBitfields(t *testing.T) {
	fd := newFakeDiscord(t)
	cfg := &config.Config{
		ServerID: "guild-1",
		Roles:    map[string]config.RoleConfig{},
		Categories: map[string]config.CategoryConfig{
			"General": {
				Channels: map[string]config.ChannelConfig{
					"welcome": {
						Type: "text",
						Permissions: map[string]config.PermissionOverwriteConfig{
							"@everyone": {
								// send_messages = 1<<11 = 2048
								// view_channel  = 1<<10 = 1024
								Deny:  []string{"send_messages"},
								Allow: []string{"view_channel"},
							},
						},
					},
				},
			},
		},
	}
	st := state.NewState("guild-1")
	live := buildLive(&discord.Guild{ID: "guild-1"}, nil, nil, nil)

	mustApply(t, fd, cfg, st, mustPlan(t, cfg, st, live))

	// New channels embed permissions in the POST body as permission_overwrites.
	chanReqs := fd.findRequests(http.MethodPost, "/guilds/guild-1/channels")
	// Find the non-category channel POST (type != 4).
	var chBody discord.CreateChannelParams
	for _, req := range chanReqs {
		var b discord.CreateChannelParams
		_ = json.Unmarshal(req.Body, &b)
		if b.Type != discord.ChannelTypeGuildCategory {
			chBody = b
			break
		}
	}

	if len(chBody.PermissionOverwrites) == 0 {
		t.Fatal("expected permission_overwrites in channel POST body")
	}

	ow := chBody.PermissionOverwrites[0]
	// send_messages = bit 11 = 2048
	if ow.Deny != "2048" {
		t.Errorf("expected deny='2048' (send_messages), got %q", ow.Deny)
	}
	// view_channel = bit 10 = 1024
	if ow.Allow != "1024" {
		t.Errorf("expected allow='1024' (view_channel), got %q", ow.Allow)
	}
	// @everyone overwrite ID == guild/server ID
	if ow.ID != "guild-1" {
		t.Errorf("expected @everyone overwrite ID 'guild-1', got %q", ow.ID)
	}
}

// TestIntegrationApplyCreatesInOrder verifies roles are created before categories,
// and categories before channels.
func TestIntegrationApplyCreatesInOrder(t *testing.T) {
	fd := newFakeDiscord(t)
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
	live := buildLive(&discord.Guild{ID: "guild-1"}, nil, nil, nil)

	mustApply(t, fd, cfg, st, mustPlan(t, cfg, st, live))

	order := fd.requestOrder()

	roleIdx, catIdx, chanIdx := -1, -1, -1
	for i, entry := range order {
		switch {
		case strings.HasSuffix(entry, "/roles") && strings.HasPrefix(entry, "POST"):
			roleIdx = i
		case strings.HasSuffix(entry, "/channels") && strings.HasPrefix(entry, "POST") && catIdx == -1:
			catIdx = i
		case strings.HasSuffix(entry, "/channels") && strings.HasPrefix(entry, "POST") && catIdx != -1:
			chanIdx = i
		}
	}

	if roleIdx == -1 {
		t.Fatal("no POST /roles found")
	}
	if catIdx == -1 {
		t.Fatal("no POST /channels (category) found")
	}
	if chanIdx == -1 {
		t.Fatal("no POST /channels (channel) found")
	}
	if roleIdx > catIdx {
		t.Errorf("role should be created before category (got role@%d, category@%d)", roleIdx, catIdx)
	}
	if catIdx > chanIdx {
		t.Errorf("category should be created before channel (got category@%d, channel@%d)", catIdx, chanIdx)
	}
}

// TestIntegrationApplyDeletesInOrder verifies channels are deleted before categories.
func TestIntegrationApplyDeletesInOrder(t *testing.T) {
	fd := newFakeDiscord(t)
	cfg := &config.Config{
		ServerID:   "guild-1",
		Roles:      map[string]config.RoleConfig{},
		Categories: map[string]config.CategoryConfig{},
	}
	st := state.NewState("guild-1")
	st.SetCategory("General", "cat-101")
	st.SetChannel("General", "welcome", "ch-102")
	live := buildLive(
		&discord.Guild{ID: "guild-1"},
		nil,
		[]*discord.Channel{{ID: "cat-101", Type: discord.ChannelTypeGuildCategory, Name: "General"}},
		[]*discord.Channel{{ID: "ch-102", Type: discord.ChannelTypeGuildText, Name: "welcome"}},
	)

	mustApply(t, fd, cfg, st, mustPlan(t, cfg, st, live))

	order := fd.requestOrder()
	chanDelIdx, catDelIdx := -1, -1
	for i, entry := range order {
		if entry == "DELETE /channels/ch-102" {
			chanDelIdx = i
		}
		if entry == "DELETE /channels/cat-101" {
			catDelIdx = i
		}
	}

	if chanDelIdx == -1 {
		t.Fatal("expected DELETE /channels/ch-102")
	}
	if catDelIdx == -1 {
		t.Fatal("expected DELETE /channels/cat-101")
	}
	if chanDelIdx > catDelIdx {
		t.Errorf("channel should be deleted before category (chan@%d, cat@%d)", chanDelIdx, catDelIdx)
	}
}

// TestIntegrationStateHasDiscordIDsAfterApply verifies that state is updated
// with the IDs returned by the fake Discord server after a successful apply.
func TestIntegrationStateHasDiscordIDsAfterApply(t *testing.T) {
	fd := newFakeDiscord(t)
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
	live := buildLive(&discord.Guild{ID: "guild-1"}, nil, nil, nil)

	mustApply(t, fd, cfg, st, mustPlan(t, cfg, st, live))

	roleID, ok := st.GetRoleID("admin")
	if !ok || roleID == "" {
		t.Error("expected 'admin' role ID in state after apply")
	}

	catID, ok := st.GetCategoryID("General")
	if !ok || catID == "" {
		t.Error("expected 'General' category ID in state after apply")
	}

	chanID, ok := st.GetChannelID("General", "welcome")
	if !ok || chanID == "" {
		t.Error("expected 'General/welcome' channel ID in state after apply")
	}

	// IDs should be distinct from each other.
	if roleID == catID || catID == chanID || roleID == chanID {
		t.Errorf("expected distinct IDs: role=%s cat=%s chan=%s", roleID, catID, chanID)
	}
}

// TestIntegrationRenameChannelIsPatchNotDeleteCreate verifies that changing a
// channel's display name (via name:) results in a PATCH, not DELETE+POST.
func TestIntegrationRenameChannelIsPatchNotDeleteCreate(t *testing.T) {
	fd := newFakeDiscord(t)
	cfg := &config.Config{
		ServerID: "guild-1",
		Roles:    map[string]config.RoleConfig{},
		Categories: map[string]config.CategoryConfig{
			"General": {
				Channels: map[string]config.ChannelConfig{
					// Stable key is "welcome-chan", Discord name is "welcome"
					"welcome-chan": {Name: "welcome", Type: "text"},
				},
			},
		},
	}
	st := state.NewState("guild-1")
	st.SetCategory("General", "cat-101")
	st.SetChannel("General", "welcome-chan", "ch-102")

	// Live Discord still shows the old name before the rename.
	live := buildLive(
		&discord.Guild{ID: "guild-1"},
		nil,
		[]*discord.Channel{{ID: "cat-101", Type: discord.ChannelTypeGuildCategory, Name: "General"}},
		[]*discord.Channel{{ID: "ch-102", Type: discord.ChannelTypeGuildText, Name: "welcome-chan"}},
	)

	mustApply(t, fd, cfg, st, mustPlan(t, cfg, st, live))

	// Should be a PATCH, not a DELETE + POST.
	deletes := fd.findRequests(http.MethodDelete, "/channels/ch-102")
	if len(deletes) != 0 {
		t.Error("channel rename should not DELETE the channel")
	}
	posts := fd.findRequests(http.MethodPost, "/guilds/guild-1/channels")
	if len(posts) != 0 {
		t.Error("channel rename should not POST a new channel")
	}
	patches := fd.findRequests(http.MethodPatch, "/channels/ch-102")
	if len(patches) == 0 {
		t.Fatal("expected PATCH /channels/ch-102 for rename")
	}

	// Verify the PATCH body contains the new name.
	var body discord.UpdateChannelParams
	_ = json.Unmarshal(patches[0].Body, &body)
	if body.Name == nil || *body.Name != "welcome" {
		t.Errorf("expected PATCH name='welcome', got %v", body.Name)
	}
}

// TestIntegrationTopLevelChannelNoParentID verifies that a top-level channel
// (not in any category) is created without a parent_id field.
func TestIntegrationTopLevelChannelNoParentID(t *testing.T) {
	fd := newFakeDiscord(t)
	cfg := &config.Config{
		ServerID:   "guild-1",
		Roles:      map[string]config.RoleConfig{},
		Categories: map[string]config.CategoryConfig{},
		Channels: map[string]config.ChannelConfig{
			"rules": {Type: "text", Topic: "Server rules"},
		},
	}
	st := state.NewState("guild-1")
	live := buildLive(&discord.Guild{ID: "guild-1"}, nil, nil, nil)

	mustApply(t, fd, cfg, st, mustPlan(t, cfg, st, live))

	reqs := fd.findRequests(http.MethodPost, "/guilds/guild-1/channels")
	if len(reqs) != 1 {
		t.Fatalf("expected 1 POST /channels, got %d", len(reqs))
	}

	var body discord.CreateChannelParams
	_ = json.Unmarshal(reqs[0].Body, &body)
	if body.ParentID != "" {
		t.Errorf("expected empty ParentID for top-level channel, got %q", body.ParentID)
	}
	if body.Name != "rules" {
		t.Errorf("expected name 'rules', got %q", body.Name)
	}
	if body.Topic != "Server rules" {
		t.Errorf("expected topic 'Server rules', got %q", body.Topic)
	}

	// Verify state uses empty-category key.
	id, ok := st.GetChannelID("", "rules")
	if !ok || id == "" {
		t.Error("expected top-level channel in state under empty category")
	}
}

// TestIntegrationUpdateRoleNamePatch verifies that a role rename sends PATCH
// with the new name, rather than deleting and recreating the role.
func TestIntegrationUpdateRoleNamePatch(t *testing.T) {
	fd := newFakeDiscord(t)
	cfg := &config.Config{
		ServerID: "guild-1",
		Roles: map[string]config.RoleConfig{
			"admin-role": {Name: "Admin", Permissions: []string{}},
		},
		Categories: map[string]config.CategoryConfig{},
	}
	st := state.NewState("guild-1")
	st.SetRole("admin-role", "role-101")

	// Live Discord still has the old name.
	live := buildLive(
		&discord.Guild{ID: "guild-1"},
		[]*discord.Role{{ID: "role-101", Name: "admin-role", Permissions: "0"}},
		nil, nil,
	)

	mustApply(t, fd, cfg, st, mustPlan(t, cfg, st, live))

	// No delete, no create.
	if len(fd.findRequests(http.MethodDelete, "/roles/role-101")) != 0 {
		t.Error("role rename should not DELETE the role")
	}
	if len(fd.findRequests(http.MethodPost, "/guilds/guild-1/roles")) != 0 {
		t.Error("role rename should not POST a new role")
	}

	patches := fd.findRequests(http.MethodPatch, "/guilds/guild-1/roles/role-101")
	if len(patches) == 0 {
		t.Fatal("expected PATCH /guilds/guild-1/roles/role-101")
	}

	var body discord.UpdateRoleParams
	_ = json.Unmarshal(patches[0].Body, &body)
	if body.Name == nil || *body.Name != "Admin" {
		t.Errorf("expected PATCH name='Admin', got %v", body.Name)
	}
}

// TestIntegrationNoRequestsWhenNothingChanged verifies that apply makes no HTTP
// calls when the config already matches the live Discord state.
func TestIntegrationNoRequestsWhenNothingChanged(t *testing.T) {
	fd := newFakeDiscord(t)

	topic := "Welcome!"
	cfg := &config.Config{
		ServerID: "guild-1",
		Roles: map[string]config.RoleConfig{
			"admin": {Color: "#FF0000", Permissions: []string{"administrator"}},
		},
		Categories: map[string]config.CategoryConfig{
			"General": {
				Position: 0,
				Channels: map[string]config.ChannelConfig{
					"welcome": {Type: "text", Topic: topic},
				},
			},
		},
	}
	st := state.NewState("guild-1")
	st.SetRole("admin", "role-101")
	st.SetCategory("General", "cat-102")
	st.SetChannel("General", "welcome", "ch-103")

	live := buildLive(
		&discord.Guild{ID: "guild-1"},
		[]*discord.Role{{ID: "role-101", Name: "admin", Color: 0xFF0000, Permissions: "8"}},
		[]*discord.Channel{{ID: "cat-102", Type: discord.ChannelTypeGuildCategory, Name: "General", Position: 0}},
		[]*discord.Channel{{ID: "ch-103", Type: discord.ChannelTypeGuildText, Name: "welcome", Topic: &topic}},
	)

	plan := mustPlan(t, cfg, st, live)
	if plan.HasChanges() {
		t.Fatalf("expected no changes, got: %v", plan.Actions)
	}

	mustApply(t, fd, cfg, st, plan)

	if len(fd.requests) != 0 {
		t.Errorf("expected no HTTP calls for no-op plan, got: %v", fd.requestOrder())
	}
}
