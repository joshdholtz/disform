package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigBasic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")
	content := `
server_id: "123456789012345678"
roles:
  admin:
    color: "#FF0000"
    hoist: true
    mentionable: true
    permissions:
      - administrator
categories:
  General:
    position: 0
    channels:
      general:
        type: text
        topic: "Hello"
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing temp config: %v", err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.ServerID != "123456789012345678" {
		t.Errorf("expected server_id %q, got %q", "123456789012345678", cfg.ServerID)
	}

	role, ok := cfg.Roles["admin"]
	if !ok {
		t.Fatal("expected role 'admin'")
	}
	if role.Color != "#FF0000" {
		t.Errorf("expected color #FF0000, got %q", role.Color)
	}
	if !role.Hoist {
		t.Error("expected hoist=true")
	}
	if !role.Mentionable {
		t.Error("expected mentionable=true")
	}
	if len(role.Permissions) != 1 || role.Permissions[0] != "administrator" {
		t.Errorf("unexpected permissions: %v", role.Permissions)
	}

	cat, ok := cfg.Categories["General"]
	if !ok {
		t.Fatal("expected category 'General'")
	}
	if cat.Position != 0 {
		t.Errorf("expected position 0, got %d", cat.Position)
	}
	ch, ok := cat.Channels["general"]
	if !ok {
		t.Fatal("expected channel 'general'")
	}
	if ch.Type != "text" {
		t.Errorf("expected type 'text', got %q", ch.Type)
	}
}

func TestLoadConfigFileNotFound(t *testing.T) {
	_, err := LoadConfig("/nonexistent/path/config.yml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadConfigMalformedYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(path, []byte(":\ninvalid: [yaml"), 0644); err != nil {
		t.Fatalf("writing temp file: %v", err)
	}
	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected error for malformed YAML")
	}
}

func TestValidateMissingServerID(t *testing.T) {
	cfg := &Config{}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for missing server_id")
	}
}

func TestValidateInvalidChannelType(t *testing.T) {
	cfg := &Config{
		ServerID: "123",
		Categories: map[string]CategoryConfig{
			"General": {
				Channels: map[string]ChannelConfig{
					"bad": {Type: "invalid_type"},
				},
			},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for invalid channel type")
	}
}

func TestValidateInvalidPermission(t *testing.T) {
	cfg := &Config{
		ServerID: "123",
		Roles: map[string]RoleConfig{
			"admin": {Permissions: []string{"nonexistent_permission"}},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for invalid permission")
	}
}

func TestValidateInvalidColor(t *testing.T) {
	cfg := &Config{
		ServerID: "123",
		Roles: map[string]RoleConfig{
			"admin": {Color: "notacolor"},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for invalid color")
	}
}

func TestValidateInvalidChannelPermission(t *testing.T) {
	cfg := &Config{
		ServerID: "123",
		Categories: map[string]CategoryConfig{
			"General": {
				Channels: map[string]ChannelConfig{
					"welcome": {
						Type: "text",
						Permissions: map[string]PermissionOverwriteConfig{
							"@everyone": {
								Allow: []string{"bad_permission"},
							},
						},
					},
				},
			},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for invalid channel permission")
	}
}

func TestValidateInvalidDenyPermission(t *testing.T) {
	cfg := &Config{
		ServerID: "123",
		Categories: map[string]CategoryConfig{
			"General": {
				Channels: map[string]ChannelConfig{
					"welcome": {
						Type: "text",
						Permissions: map[string]PermissionOverwriteConfig{
							"@everyone": {
								Deny: []string{"totally_fake"},
							},
						},
					},
				},
			},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for invalid deny permission")
	}
}

func TestValidateValidConfig(t *testing.T) {
	cfg := &Config{
		ServerID: "123456789012345678",
		Roles: map[string]RoleConfig{
			"admin": {
				Color:       "#FF0000",
				Permissions: []string{"administrator"},
			},
			"mod": {
				Permissions: []string{"kick_members", "manage_messages"},
			},
		},
		Categories: map[string]CategoryConfig{
			"General": {
				Channels: map[string]ChannelConfig{
					"welcome": {
						Type: "text",
						Permissions: map[string]PermissionOverwriteConfig{
							"@everyone": {
								Deny: []string{"send_messages"},
							},
						},
					},
					"general-voice": {
						Type:    "voice",
						Bitrate: 64000,
					},
				},
			},
		},
	}
	if err := Validate(cfg); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestColorToInt(t *testing.T) {
	tests := []struct {
		input    string
		expected int
		wantErr  bool
	}{
		{"#FF0000", 0xFF0000, false},
		{"#00FF00", 0x00FF00, false},
		{"#0000FF", 0x0000FF, false},
		{"#FFA500", 0xFFA500, false},
		{"#000000", 0, false},
		{"#FFFFFF", 0xFFFFFF, false},
		{"notacolor", 0, true},
		{"#GG0000", 0, true},
		{"#FF00", 0, true},
		{"FF0000", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ColorToInt(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for input %q", tt.input)
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if got != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, got)
			}
		})
	}
}

func TestPermissionsToInt(t *testing.T) {
	tests := []struct {
		name     string
		perms    []string
		expected int64
		wantErr  bool
	}{
		{
			name:     "administrator",
			perms:    []string{"administrator"},
			expected: 1 << 3,
		},
		{
			name:     "multiple permissions",
			perms:    []string{"kick_members", "ban_members"},
			expected: (1 << 1) | (1 << 2),
		},
		{
			name:     "empty permissions",
			perms:    []string{},
			expected: 0,
		},
		{
			name:    "invalid permission",
			perms:   []string{"nonexistent"},
			wantErr: true,
		},
		{
			name:     "high bit permission",
			perms:    []string{"moderate_members"},
			expected: 1 << 40,
		},
		{
			name:     "view channel",
			perms:    []string{"view_channel"},
			expected: 1 << 10,
		},
		{
			name:     "send messages",
			perms:    []string{"send_messages"},
			expected: 1 << 11,
		},
		{
			name:     "manage threads",
			perms:    []string{"manage_threads"},
			expected: 1 << 34,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := PermissionsToInt(tt.perms)
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
			if got != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, got)
			}
		})
	}
}

func TestLoadConfigNilMaps(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")
	content := `server_id: "123456789012345678"`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing temp file: %v", err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Roles == nil {
		t.Error("expected Roles map to be initialized")
	}
	if cfg.Categories == nil {
		t.Error("expected Categories map to be initialized")
	}
}

func TestValidChannelTypes(t *testing.T) {
	expected := []string{"text", "voice", "announcement", "forum", "stage"}
	for _, ct := range expected {
		if !ValidChannelTypes[ct] {
			t.Errorf("expected %q to be a valid channel type", ct)
		}
	}
	if ValidChannelTypes["invalid"] {
		t.Error("expected 'invalid' to not be a valid channel type")
	}
}

func TestAllPermissionsValid(t *testing.T) {
	allPerms := []string{
		"create_instant_invite", "kick_members", "ban_members", "administrator",
		"manage_channels", "manage_guild", "add_reactions", "view_audit_log",
		"priority_speaker", "stream", "view_channel", "send_messages",
		"send_tts_messages", "manage_messages", "embed_links", "attach_files",
		"read_message_history", "mention_everyone", "use_external_emojis",
		"view_guild_insights", "connect", "speak", "mute_members", "deafen_members",
		"move_members", "use_vad", "change_nickname", "manage_nicknames",
		"manage_roles", "manage_webhooks", "manage_emojis", "use_slash_commands",
		"request_to_speak", "manage_threads", "create_public_threads",
		"create_private_threads", "use_external_stickers", "send_messages_in_threads",
		"use_embedded_activities", "moderate_members",
	}

	result, err := PermissionsToInt(allPerms)
	if err != nil {
		t.Fatalf("unexpected error converting all permissions: %v", err)
	}
	if result == 0 {
		t.Error("expected non-zero permissions result")
	}
}

func TestValidateServerSettings(t *testing.T) {
	cfg := &Config{
		ServerID: "123",
		Settings: &ServerSettings{
			VerificationLevel:           "medium",
			ExplicitContentFilter:       "all_members",
			DefaultMessageNotifications: "only_mentions",
			AFKTimeout:                  300,
		},
	}
	if err := Validate(cfg); err != nil {
		t.Fatalf("unexpected error for valid settings: %v", err)
	}
}

func TestValidateInvalidVerificationLevel(t *testing.T) {
	cfg := &Config{
		ServerID: "123",
		Settings: &ServerSettings{
			VerificationLevel: "extreme",
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for invalid verification_level")
	}
}

func TestValidateInvalidContentFilter(t *testing.T) {
	cfg := &Config{
		ServerID: "123",
		Settings: &ServerSettings{
			ExplicitContentFilter: "unknown_filter",
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for invalid explicit_content_filter")
	}
}

func TestValidateInvalidAFKTimeout(t *testing.T) {
	cfg := &Config{
		ServerID: "123",
		Settings: &ServerSettings{
			AFKTimeout: 999,
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for invalid afk_timeout")
	}
}

func TestVerificationLevelToInt(t *testing.T) {
	tests := []struct {
		input    string
		expected int
		wantErr  bool
	}{
		{"none", 0, false},
		{"low", 1, false},
		{"medium", 2, false},
		{"high", 3, false},
		{"very_high", 4, false},
		{"invalid", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := VerificationLevelToInt(tt.input)
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
			if got != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, got)
			}
		})
	}
}

func TestContentFilterToInt(t *testing.T) {
	tests := []struct {
		input    string
		expected int
		wantErr  bool
	}{
		{"disabled", 0, false},
		{"members_without_roles", 1, false},
		{"all_members", 2, false},
		{"invalid", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ContentFilterToInt(tt.input)
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
			if got != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, got)
			}
		})
	}
}

func TestDefaultNotificationsToInt(t *testing.T) {
	tests := []struct {
		input    string
		expected int
		wantErr  bool
	}{
		{"all_messages", 0, false},
		{"only_mentions", 1, false},
		{"invalid", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := DefaultNotificationsToInt(tt.input)
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
			if got != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, got)
			}
		})
	}
}

func TestValidateTopLevelChannels(t *testing.T) {
	cfg := &Config{
		ServerID: "123",
		Channels: map[string]ChannelConfig{
			"announcements": {Type: "announcement"},
			"general":       {Type: "text", Topic: "Main chat"},
		},
	}
	if err := Validate(cfg); err != nil {
		t.Fatalf("unexpected error for valid top-level channels: %v", err)
	}

	// Invalid type in top-level channel.
	cfg2 := &Config{
		ServerID: "123",
		Channels: map[string]ChannelConfig{
			"bad": {Type: "invalid_type"},
		},
	}
	err := Validate(cfg2)
	if err == nil {
		t.Fatal("expected error for invalid top-level channel type")
	}
}
