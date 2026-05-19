package cmd

import (
	"encoding/json"
	"fmt"
	"sync/atomic"

	"github.com/joshholtz/disform/internal/discord"
)

// dryRunRecord captures a single would-be API call.
type dryRunRecord struct {
	Method string
	Path   string
	Body   interface{}
}

// dryRunClient implements discord.Client but records calls instead of sending them.
// Fake IDs are returned for creates so the applier can chain dependent calls
// (e.g. channel needs the category ID that was "created" earlier).
type dryRunClient struct {
	guildID string
	records []dryRunRecord
	counter atomic.Int64
}

func newDryRunClient(guildID string) *dryRunClient {
	return &dryRunClient{guildID: guildID}
}

func (c *dryRunClient) fakeID() string {
	n := c.counter.Add(1)
	return fmt.Sprintf("<dry-run-id-%d>", n)
}

func (c *dryRunClient) record(method, path string, body interface{}) {
	c.records = append(c.records, dryRunRecord{Method: method, Path: path, Body: body})
}

func (c *dryRunClient) PrintRecords() {
	if len(c.records) == 0 {
		fmt.Println("  No API calls would be made.")
		return
	}
	for _, r := range c.records {
		fmt.Printf("  %s %s\n", r.Method, r.Path)
		if r.Body != nil {
			b, _ := json.MarshalIndent(r.Body, "  ", "  ")
			fmt.Printf("  %s\n", b)
		}
		fmt.Println()
	}
}

func (c *dryRunClient) GetGuild(guildID string) (*discord.Guild, error) {
	return &discord.Guild{ID: guildID}, nil
}

func (c *dryRunClient) GetChannels(guildID string) ([]*discord.Channel, error) {
	return nil, nil
}

func (c *dryRunClient) GetRoles(guildID string) ([]*discord.Role, error) {
	return nil, nil
}

func (c *dryRunClient) CreateChannel(guildID string, params discord.CreateChannelParams) (*discord.Channel, error) {
	c.record("POST", fmt.Sprintf("/guilds/%s/channels", guildID), params)
	return &discord.Channel{ID: c.fakeID(), Name: params.Name, Type: params.Type}, nil
}

func (c *dryRunClient) UpdateChannel(channelID string, params discord.UpdateChannelParams) (*discord.Channel, error) {
	c.record("PATCH", fmt.Sprintf("/channels/%s", channelID), params)
	return &discord.Channel{ID: channelID}, nil
}

func (c *dryRunClient) DeleteChannel(channelID string) error {
	c.record("DELETE", fmt.Sprintf("/channels/%s", channelID), nil)
	return nil
}

func (c *dryRunClient) CreateRole(guildID string, params discord.CreateRoleParams) (*discord.Role, error) {
	c.record("POST", fmt.Sprintf("/guilds/%s/roles", guildID), params)
	return &discord.Role{ID: c.fakeID(), Name: params.Name}, nil
}

func (c *dryRunClient) UpdateRole(guildID, roleID string, params discord.UpdateRoleParams) (*discord.Role, error) {
	c.record("PATCH", fmt.Sprintf("/guilds/%s/roles/%s", guildID, roleID), params)
	return &discord.Role{ID: roleID}, nil
}

func (c *dryRunClient) DeleteRole(guildID, roleID string) error {
	c.record("DELETE", fmt.Sprintf("/guilds/%s/roles/%s", guildID, roleID), nil)
	return nil
}

func (c *dryRunClient) EditChannelPermissions(channelID, overwriteID string, params discord.EditChannelPermissionsParams) error {
	c.record("PUT", fmt.Sprintf("/channels/%s/permissions/%s", channelID, overwriteID), params)
	return nil
}

func (c *dryRunClient) DeleteChannelPermission(channelID, overwriteID string) error {
	c.record("DELETE", fmt.Sprintf("/channels/%s/permissions/%s", channelID, overwriteID), nil)
	return nil
}

func (c *dryRunClient) ModifyGuild(guildID string, params discord.ModifyGuildParams) (*discord.Guild, error) {
	c.record("PATCH", fmt.Sprintf("/guilds/%s", guildID), params)
	return &discord.Guild{ID: guildID}, nil
}
