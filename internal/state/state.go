package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

// ResourceState holds the Discord ID of a managed resource.
type ResourceState struct {
	DiscordID string `json:"discord_id"`
}

// State tracks all managed resources and their Discord IDs.
type State struct {
	Version    int                      `json:"version"`
	ServerID   string                   `json:"server_id"`
	Roles      map[string]ResourceState `json:"roles"`
	Categories map[string]ResourceState `json:"categories"`
	Channels   map[string]ResourceState `json:"channels"` // key: "CategoryName/channel-name"
}

// NewState creates a new empty state for the given server.
func NewState(serverID string) *State {
	return &State{
		Version:    1,
		ServerID:   serverID,
		Roles:      make(map[string]ResourceState),
		Categories: make(map[string]ResourceState),
		Channels:   make(map[string]ResourceState),
	}
}

// LoadState reads a state file from disk. If the file does not exist, it returns a new empty state.
func LoadState(path string) (*State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return NewState(""), nil
		}
		return nil, fmt.Errorf("reading state file %q: %w", path, err)
	}

	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parsing state file %q: %w", path, err)
	}

	// Ensure maps are initialized even if empty in JSON.
	if s.Roles == nil {
		s.Roles = make(map[string]ResourceState)
	}
	if s.Categories == nil {
		s.Categories = make(map[string]ResourceState)
	}
	if s.Channels == nil {
		s.Channels = make(map[string]ResourceState)
	}

	return &s, nil
}

// SaveState writes the state to disk as JSON.
func SaveState(s *State, path string) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling state: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing state file %q: %w", path, err)
	}
	return nil
}

// GetRoleID returns the Discord ID for a role by logical name.
func (s *State) GetRoleID(name string) (string, bool) {
	rs, ok := s.Roles[name]
	return rs.DiscordID, ok
}

// GetCategoryID returns the Discord ID for a category by logical name.
func (s *State) GetCategoryID(name string) (string, bool) {
	rs, ok := s.Categories[name]
	return rs.DiscordID, ok
}

// GetChannelID returns the Discord ID for a channel by category and channel name.
func (s *State) GetChannelID(categoryName, channelName string) (string, bool) {
	key := ChannelKey(categoryName, channelName)
	rs, ok := s.Channels[key]
	return rs.DiscordID, ok
}

// SetRole records or updates a role's Discord ID in state.
func (s *State) SetRole(name, discordID string) {
	s.Roles[name] = ResourceState{DiscordID: discordID}
}

// SetCategory records or updates a category's Discord ID in state.
func (s *State) SetCategory(name, discordID string) {
	s.Categories[name] = ResourceState{DiscordID: discordID}
}

// SetChannel records or updates a channel's Discord ID in state.
func (s *State) SetChannel(categoryName, channelName, discordID string) {
	key := ChannelKey(categoryName, channelName)
	s.Channels[key] = ResourceState{DiscordID: discordID}
}

// DeleteRole removes a role from state.
func (s *State) DeleteRole(name string) {
	delete(s.Roles, name)
}

// DeleteCategory removes a category from state.
func (s *State) DeleteCategory(name string) {
	delete(s.Categories, name)
}

// DeleteChannel removes a channel from state.
func (s *State) DeleteChannel(categoryName, channelName string) {
	key := ChannelKey(categoryName, channelName)
	delete(s.Channels, key)
}

// ChannelKey returns the state map key for a channel: "CategoryName/channel-name".
func ChannelKey(categoryName, channelName string) string {
	return categoryName + "/" + channelName
}
