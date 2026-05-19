package discord

// Channel type constants for Discord API.
const (
	ChannelTypeGuildText         = 0
	ChannelTypeGuildVoice        = 2
	ChannelTypeGuildCategory     = 4
	ChannelTypeGuildAnnouncement = 5
	ChannelTypeGuildStage        = 13
	ChannelTypeGuildForum        = 15
)

// Overwrite type constants for Discord permission overwrites.
const (
	OverwriteTypeRole   = 0
	OverwriteTypeMember = 1
)

// Guild represents a Discord guild (server).
type Guild struct {
	ID                          string  `json:"id"`
	Name                        string  `json:"name"`
	VerificationLevel           int     `json:"verification_level"`
	ExplicitContentFilter       int     `json:"explicit_content_filter"`
	DefaultMessageNotifications int     `json:"default_message_notifications"`
	AFKChannelID                *string `json:"afk_channel_id"`
	AFKTimeout                  int     `json:"afk_timeout"`
	SystemChannelID             *string `json:"system_channel_id"`
}

// ModifyGuildParams contains parameters for modifying a Discord guild (all fields optional).
type ModifyGuildParams struct {
	VerificationLevel           *int    `json:"verification_level,omitempty"`
	ExplicitContentFilter       *int    `json:"explicit_content_filter,omitempty"`
	DefaultMessageNotifications *int    `json:"default_message_notifications,omitempty"`
	AFKChannelID                *string `json:"afk_channel_id,omitempty"`
	AFKTimeout                  *int    `json:"afk_timeout,omitempty"`
	SystemChannelID             *string `json:"system_channel_id,omitempty"`
}

// Channel represents a Discord channel or category.
type Channel struct {
	ID                   string                `json:"id"`
	Type                 int                   `json:"type"`
	GuildID              string                `json:"guild_id"`
	Name                 string                `json:"name"`
	Position             int                   `json:"position"`
	Topic                *string               `json:"topic"`
	NSFW                 bool                  `json:"nsfw"`
	Bitrate              int                   `json:"bitrate"`
	UserLimit            int                   `json:"user_limit"`
	RateLimitPerUser     int                   `json:"rate_limit_per_user"`
	ParentID             *string               `json:"parent_id"`
	PermissionOverwrites []PermissionOverwrite `json:"permission_overwrites"`
}

// PermissionOverwrite represents a role or member permission overwrite on a channel.
type PermissionOverwrite struct {
	ID    string `json:"id"`
	Type  int    `json:"type"`
	Allow string `json:"allow"`
	Deny  string `json:"deny"`
}

// Role represents a Discord role.
type Role struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Color       int    `json:"color"`
	Hoist       bool   `json:"hoist"`
	Mentionable bool   `json:"mentionable"`
	Permissions string `json:"permissions"`
	Position    int    `json:"position"`
}

// CreateChannelParams contains parameters for creating a Discord channel.
type CreateChannelParams struct {
	Name                 string                `json:"name"`
	Type                 int                   `json:"type"`
	Topic                string                `json:"topic,omitempty"`
	Position             int                   `json:"position,omitempty"`
	NSFW                 bool                  `json:"nsfw,omitempty"`
	Bitrate              int                   `json:"bitrate,omitempty"`
	UserLimit            int                   `json:"user_limit,omitempty"`
	RateLimitPerUser     int                   `json:"rate_limit_per_user,omitempty"`
	ParentID             string                `json:"parent_id,omitempty"`
	PermissionOverwrites []PermissionOverwrite `json:"permission_overwrites,omitempty"`
}

// UpdateChannelParams contains parameters for updating a Discord channel (all fields optional).
type UpdateChannelParams struct {
	Name             *string `json:"name,omitempty"`
	Type             *int    `json:"type,omitempty"`
	Topic            *string `json:"topic,omitempty"`
	Position         *int    `json:"position,omitempty"`
	NSFW             *bool   `json:"nsfw,omitempty"`
	Bitrate          *int    `json:"bitrate,omitempty"`
	UserLimit        *int    `json:"user_limit,omitempty"`
	RateLimitPerUser *int    `json:"rate_limit_per_user,omitempty"`
	ParentID         *string `json:"parent_id,omitempty"`
}

// CreateRoleParams contains parameters for creating a Discord role.
type CreateRoleParams struct {
	Name        string `json:"name"`
	Permissions string `json:"permissions"`
	Color       int    `json:"color"`
	Hoist       bool   `json:"hoist"`
	Mentionable bool   `json:"mentionable"`
}

// UpdateRoleParams contains parameters for updating a Discord role (all fields optional).
type UpdateRoleParams struct {
	Name        *string `json:"name,omitempty"`
	Permissions *string `json:"permissions,omitempty"`
	Color       *int    `json:"color,omitempty"`
	Hoist       *bool   `json:"hoist,omitempty"`
	Mentionable *bool   `json:"mentionable,omitempty"`
}

// EditChannelPermissionsParams contains parameters for editing a channel permission overwrite.
type EditChannelPermissionsParams struct {
	Allow string `json:"allow"`
	Deny  string `json:"deny"`
	Type  int    `json:"type"`
}
