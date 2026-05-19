package state

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewState(t *testing.T) {
	s := NewState("123456789")

	if s.Version != 1 {
		t.Errorf("expected Version=1, got %d", s.Version)
	}
	if s.ServerID != "123456789" {
		t.Errorf("expected ServerID=%q, got %q", "123456789", s.ServerID)
	}
	if s.Roles == nil {
		t.Error("expected Roles map to be initialized")
	}
	if s.Categories == nil {
		t.Error("expected Categories map to be initialized")
	}
	if s.Channels == nil {
		t.Error("expected Channels map to be initialized")
	}
	if len(s.Roles) != 0 {
		t.Errorf("expected empty Roles, got %d entries", len(s.Roles))
	}
	if len(s.Categories) != 0 {
		t.Errorf("expected empty Categories, got %d entries", len(s.Categories))
	}
	if len(s.Channels) != 0 {
		t.Errorf("expected empty Channels, got %d entries", len(s.Channels))
	}
}

func TestLoadStateFileNotExist(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent.state.json")

	s, err := LoadState(path)
	if err != nil {
		t.Fatalf("expected no error for missing file, got: %v", err)
	}
	if s == nil {
		t.Fatal("expected non-nil state")
	}
	if s.Version != 1 {
		t.Errorf("expected Version=1, got %d", s.Version)
	}
	if s.Roles == nil || s.Categories == nil || s.Channels == nil {
		t.Error("expected all maps to be initialized")
	}
}

func TestLoadStateMalformed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.state.json")

	if err := os.WriteFile(path, []byte("not valid json {{{"), 0644); err != nil {
		t.Fatalf("writing temp file: %v", err)
	}

	_, err := LoadState(path)
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

func TestSaveAndLoadState(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.state.json")

	original := NewState("123456789012345678")
	original.SetRole("admin", "987654321098765432")
	original.SetRole("moderator", "987654321098765433")
	original.SetCategory("General", "111111111111111111")
	original.SetCategory("Voice", "111111111111111112")
	original.SetChannel("General", "welcome", "222222222222222222")
	original.SetChannel("General", "general-chat", "222222222222222223")
	original.SetChannel("Voice", "general-voice", "222222222222222225")

	if err := SaveState(original, path); err != nil {
		t.Fatalf("SaveState error: %v", err)
	}

	loaded, err := LoadState(path)
	if err != nil {
		t.Fatalf("LoadState error: %v", err)
	}

	if loaded.Version != original.Version {
		t.Errorf("expected Version=%d, got %d", original.Version, loaded.Version)
	}
	if loaded.ServerID != original.ServerID {
		t.Errorf("expected ServerID=%q, got %q", original.ServerID, loaded.ServerID)
	}

	// Roles
	if id, ok := loaded.GetRoleID("admin"); !ok || id != "987654321098765432" {
		t.Errorf("expected admin role ID, got ok=%v id=%q", ok, id)
	}
	if id, ok := loaded.GetRoleID("moderator"); !ok || id != "987654321098765433" {
		t.Errorf("expected moderator role ID, got ok=%v id=%q", ok, id)
	}

	// Categories
	if id, ok := loaded.GetCategoryID("General"); !ok || id != "111111111111111111" {
		t.Errorf("expected General category ID, got ok=%v id=%q", ok, id)
	}
	if id, ok := loaded.GetCategoryID("Voice"); !ok || id != "111111111111111112" {
		t.Errorf("expected Voice category ID, got ok=%v id=%q", ok, id)
	}

	// Channels
	if id, ok := loaded.GetChannelID("General", "welcome"); !ok || id != "222222222222222222" {
		t.Errorf("expected General/welcome channel ID, got ok=%v id=%q", ok, id)
	}
	if id, ok := loaded.GetChannelID("General", "general-chat"); !ok || id != "222222222222222223" {
		t.Errorf("expected General/general-chat channel ID, got ok=%v id=%q", ok, id)
	}
	if id, ok := loaded.GetChannelID("Voice", "general-voice"); !ok || id != "222222222222222225" {
		t.Errorf("expected Voice/general-voice channel ID, got ok=%v id=%q", ok, id)
	}
}

func TestStateRoleOperations(t *testing.T) {
	s := NewState("123")

	// Set
	s.SetRole("admin", "111")
	s.SetRole("member", "222")

	// Get
	id, ok := s.GetRoleID("admin")
	if !ok {
		t.Error("expected admin role to exist")
	}
	if id != "111" {
		t.Errorf("expected ID '111', got %q", id)
	}

	id, ok = s.GetRoleID("member")
	if !ok {
		t.Error("expected member role to exist")
	}
	if id != "222" {
		t.Errorf("expected ID '222', got %q", id)
	}

	// Get non-existent
	_, ok = s.GetRoleID("nonexistent")
	if ok {
		t.Error("expected non-existent role to return false")
	}

	// Update
	s.SetRole("admin", "999")
	id, _ = s.GetRoleID("admin")
	if id != "999" {
		t.Errorf("expected updated ID '999', got %q", id)
	}

	// Delete
	s.DeleteRole("admin")
	_, ok = s.GetRoleID("admin")
	if ok {
		t.Error("expected deleted role to not exist")
	}

	// Delete non-existent (should not panic)
	s.DeleteRole("nonexistent")
}

func TestStateCategoryOperations(t *testing.T) {
	s := NewState("123")

	// Set
	s.SetCategory("General", "cat-111")
	s.SetCategory("Voice", "cat-222")

	// Get
	id, ok := s.GetCategoryID("General")
	if !ok {
		t.Error("expected General category to exist")
	}
	if id != "cat-111" {
		t.Errorf("expected ID 'cat-111', got %q", id)
	}

	id, ok = s.GetCategoryID("Voice")
	if !ok {
		t.Error("expected Voice category to exist")
	}
	if id != "cat-222" {
		t.Errorf("expected ID 'cat-222', got %q", id)
	}

	// Get non-existent
	_, ok = s.GetCategoryID("NonExistent")
	if ok {
		t.Error("expected non-existent category to return false")
	}

	// Update
	s.SetCategory("General", "cat-999")
	id, _ = s.GetCategoryID("General")
	if id != "cat-999" {
		t.Errorf("expected updated ID 'cat-999', got %q", id)
	}

	// Delete
	s.DeleteCategory("General")
	_, ok = s.GetCategoryID("General")
	if ok {
		t.Error("expected deleted category to not exist")
	}

	// Delete non-existent (should not panic)
	s.DeleteCategory("NonExistent")
}

func TestStateChannelOperations(t *testing.T) {
	s := NewState("123")

	// Set
	s.SetChannel("General", "welcome", "ch-111")
	s.SetChannel("General", "general-chat", "ch-222")
	s.SetChannel("Voice", "general-voice", "ch-333")

	// Get
	id, ok := s.GetChannelID("General", "welcome")
	if !ok {
		t.Error("expected General/welcome to exist")
	}
	if id != "ch-111" {
		t.Errorf("expected ID 'ch-111', got %q", id)
	}

	id, ok = s.GetChannelID("General", "general-chat")
	if !ok {
		t.Error("expected General/general-chat to exist")
	}
	if id != "ch-222" {
		t.Errorf("expected ID 'ch-222', got %q", id)
	}

	id, ok = s.GetChannelID("Voice", "general-voice")
	if !ok {
		t.Error("expected Voice/general-voice to exist")
	}
	if id != "ch-333" {
		t.Errorf("expected ID 'ch-333', got %q", id)
	}

	// Get non-existent
	_, ok = s.GetChannelID("General", "nonexistent")
	if ok {
		t.Error("expected non-existent channel to return false")
	}

	// Update
	s.SetChannel("General", "welcome", "ch-999")
	id, _ = s.GetChannelID("General", "welcome")
	if id != "ch-999" {
		t.Errorf("expected updated ID 'ch-999', got %q", id)
	}

	// Delete
	s.DeleteChannel("General", "welcome")
	_, ok = s.GetChannelID("General", "welcome")
	if ok {
		t.Error("expected deleted channel to not exist")
	}

	// Other channels unaffected
	_, ok = s.GetChannelID("General", "general-chat")
	if !ok {
		t.Error("expected General/general-chat to still exist after deleting welcome")
	}

	// Delete non-existent (should not panic)
	s.DeleteChannel("General", "nonexistent")
}

func TestChannelKey(t *testing.T) {
	tests := []struct {
		category string
		channel  string
		expected string
	}{
		{"General", "welcome", "General/welcome"},
		{"Voice", "general-voice", "Voice/general-voice"},
		{"My Category", "my-channel", "My Category/my-channel"},
		{"A", "B", "A/B"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			got := ChannelKey(tt.category, tt.channel)
			if got != tt.expected {
				t.Errorf("ChannelKey(%q, %q) = %q, want %q", tt.category, tt.channel, got, tt.expected)
			}
		})
	}
}

func TestLoadStateWithNilMaps(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.state.json")

	// Write state with null maps
	content := `{"version": 1, "server_id": "123", "roles": null, "categories": null, "channels": null}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing temp file: %v", err)
	}

	s, err := LoadState(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Roles == nil {
		t.Error("expected Roles map to be initialized after load")
	}
	if s.Categories == nil {
		t.Error("expected Categories map to be initialized after load")
	}
	if s.Channels == nil {
		t.Error("expected Channels map to be initialized after load")
	}
}

func TestSaveStateCreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "new.state.json")

	s := NewState("server-123")
	s.SetRole("admin", "role-id-1")

	if err := SaveState(s, path); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("expected state file to be created")
	}
}

func TestStateChannelKeyUsedInMap(t *testing.T) {
	s := NewState("123")
	s.SetChannel("General", "welcome", "ch-111")

	// Verify the key exists in the underlying map
	key := ChannelKey("General", "welcome")
	rs, ok := s.Channels[key]
	if !ok {
		t.Errorf("expected key %q to exist in Channels map", key)
	}
	if rs.DiscordID != "ch-111" {
		t.Errorf("expected DiscordID 'ch-111', got %q", rs.DiscordID)
	}
}
