package discord

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client defines the interface for interacting with the Discord API.
type Client interface {
	GetGuild(guildID string) (*Guild, error)
	GetChannels(guildID string) ([]*Channel, error)
	GetRoles(guildID string) ([]*Role, error)
	CreateChannel(guildID string, params CreateChannelParams) (*Channel, error)
	UpdateChannel(channelID string, params UpdateChannelParams) (*Channel, error)
	DeleteChannel(channelID string) error
	CreateRole(guildID string, params CreateRoleParams) (*Role, error)
	UpdateRole(guildID, roleID string, params UpdateRoleParams) (*Role, error)
	DeleteRole(guildID, roleID string) error
	EditChannelPermissions(channelID, overwriteID string, params EditChannelPermissionsParams) error
	DeleteChannelPermission(channelID, overwriteID string) error
}

// HTTPClient is the concrete Discord API client using HTTP.
type HTTPClient struct {
	token   string
	baseURL string
	http    *http.Client
}

// NewHTTPClient creates an HTTPClient with the default Discord API base URL.
func NewHTTPClient(token string) *HTTPClient {
	return NewHTTPClientWithBase(token, "https://discord.com/api/v10")
}

// NewHTTPClientWithBase creates an HTTPClient with a custom base URL (useful for testing).
func NewHTTPClientWithBase(token, baseURL string) *HTTPClient {
	return &HTTPClient{
		token:   token,
		baseURL: baseURL,
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

// rateLimitResponse is used to parse Discord's 429 rate limit response.
type rateLimitResponse struct {
	RetryAfter float64 `json:"retry_after"`
}

// do performs an HTTP request, handling auth, JSON encoding, and rate limits.
func (c *HTTPClient) do(method, path string, body interface{}) ([]byte, int, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, 0, fmt.Errorf("marshalling request body: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	return c.doWithBody(method, path, reqBody)
}

// doWithBody performs the actual HTTP request with retry on rate limit.
func (c *HTTPClient) doWithBody(method, path string, body io.Reader) ([]byte, int, error) {
	url := c.baseURL + path

	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = io.ReadAll(body)
		if err != nil {
			return nil, 0, fmt.Errorf("reading request body: %w", err)
		}
	}

	doOnce := func() ([]byte, int, error) {
		var reqBody io.Reader
		if bodyBytes != nil {
			reqBody = bytes.NewReader(bodyBytes)
		}

		req, err := http.NewRequest(method, url, reqBody)
		if err != nil {
			return nil, 0, fmt.Errorf("creating request: %w", err)
		}

		req.Header.Set("Authorization", "Bot "+c.token)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		resp, err := c.http.Do(req)
		if err != nil {
			return nil, 0, fmt.Errorf("executing request: %w", err)
		}
		defer resp.Body.Close()

		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, resp.StatusCode, fmt.Errorf("reading response body: %w", err)
		}

		return respBody, resp.StatusCode, nil
	}

	respBody, statusCode, err := doOnce()
	if err != nil {
		return nil, statusCode, err
	}

	// Handle rate limiting with a single retry.
	if statusCode == http.StatusTooManyRequests {
		var rl rateLimitResponse
		if jsonErr := json.Unmarshal(respBody, &rl); jsonErr == nil && rl.RetryAfter > 0 {
			time.Sleep(time.Duration(rl.RetryAfter * float64(time.Second)))
		} else {
			time.Sleep(time.Second)
		}

		respBody, statusCode, err = doOnce()
		if err != nil {
			return nil, statusCode, err
		}
	}

	if statusCode < 200 || statusCode >= 300 {
		return nil, statusCode, fmt.Errorf("discord API error (status %d): %s", statusCode, string(respBody))
	}

	return respBody, statusCode, nil
}

// request is a helper that sets Content-Type for non-nil bodies.
func (c *HTTPClient) request(method, path string, body interface{}) ([]byte, error) {
	data, _, err := c.do(method, path, body)
	return data, err
}

// requestNoBody performs a request without a body (for DELETE etc.).
func (c *HTTPClient) requestNoBody(method, path string) error {
	_, _, err := c.do(method, path, nil)
	return err
}

// GetGuild fetches a Discord guild by ID.
func (c *HTTPClient) GetGuild(guildID string) (*Guild, error) {
	data, err := c.request("GET", "/guilds/"+guildID, nil)
	if err != nil {
		return nil, fmt.Errorf("GetGuild %q: %w", guildID, err)
	}
	var guild Guild
	if err := json.Unmarshal(data, &guild); err != nil {
		return nil, fmt.Errorf("GetGuild %q: parsing response: %w", guildID, err)
	}
	return &guild, nil
}

// GetChannels fetches all channels in a guild.
func (c *HTTPClient) GetChannels(guildID string) ([]*Channel, error) {
	data, err := c.request("GET", "/guilds/"+guildID+"/channels", nil)
	if err != nil {
		return nil, fmt.Errorf("GetChannels %q: %w", guildID, err)
	}
	var channels []*Channel
	if err := json.Unmarshal(data, &channels); err != nil {
		return nil, fmt.Errorf("GetChannels %q: parsing response: %w", guildID, err)
	}
	return channels, nil
}

// GetRoles fetches all roles in a guild.
func (c *HTTPClient) GetRoles(guildID string) ([]*Role, error) {
	data, err := c.request("GET", "/guilds/"+guildID+"/roles", nil)
	if err != nil {
		return nil, fmt.Errorf("GetRoles %q: %w", guildID, err)
	}
	var roles []*Role
	if err := json.Unmarshal(data, &roles); err != nil {
		return nil, fmt.Errorf("GetRoles %q: parsing response: %w", guildID, err)
	}
	return roles, nil
}

// CreateChannel creates a new channel in a guild.
func (c *HTTPClient) CreateChannel(guildID string, params CreateChannelParams) (*Channel, error) {
	data, err := c.request("POST", "/guilds/"+guildID+"/channels", params)
	if err != nil {
		return nil, fmt.Errorf("CreateChannel in guild %q: %w", guildID, err)
	}
	var ch Channel
	if err := json.Unmarshal(data, &ch); err != nil {
		return nil, fmt.Errorf("CreateChannel in guild %q: parsing response: %w", guildID, err)
	}
	return &ch, nil
}

// UpdateChannel updates an existing channel.
func (c *HTTPClient) UpdateChannel(channelID string, params UpdateChannelParams) (*Channel, error) {
	data, err := c.request("PATCH", "/channels/"+channelID, params)
	if err != nil {
		return nil, fmt.Errorf("UpdateChannel %q: %w", channelID, err)
	}
	var ch Channel
	if err := json.Unmarshal(data, &ch); err != nil {
		return nil, fmt.Errorf("UpdateChannel %q: parsing response: %w", channelID, err)
	}
	return &ch, nil
}

// DeleteChannel deletes a channel.
func (c *HTTPClient) DeleteChannel(channelID string) error {
	if err := c.requestNoBody("DELETE", "/channels/"+channelID); err != nil {
		return fmt.Errorf("DeleteChannel %q: %w", channelID, err)
	}
	return nil
}

// CreateRole creates a new role in a guild.
func (c *HTTPClient) CreateRole(guildID string, params CreateRoleParams) (*Role, error) {
	data, err := c.request("POST", "/guilds/"+guildID+"/roles", params)
	if err != nil {
		return nil, fmt.Errorf("CreateRole in guild %q: %w", guildID, err)
	}
	var role Role
	if err := json.Unmarshal(data, &role); err != nil {
		return nil, fmt.Errorf("CreateRole in guild %q: parsing response: %w", guildID, err)
	}
	return &role, nil
}

// UpdateRole updates an existing role.
func (c *HTTPClient) UpdateRole(guildID, roleID string, params UpdateRoleParams) (*Role, error) {
	data, err := c.request("PATCH", "/guilds/"+guildID+"/roles/"+roleID, params)
	if err != nil {
		return nil, fmt.Errorf("UpdateRole %q in guild %q: %w", roleID, guildID, err)
	}
	var role Role
	if err := json.Unmarshal(data, &role); err != nil {
		return nil, fmt.Errorf("UpdateRole %q: parsing response: %w", roleID, err)
	}
	return &role, nil
}

// DeleteRole deletes a role from a guild.
func (c *HTTPClient) DeleteRole(guildID, roleID string) error {
	if err := c.requestNoBody("DELETE", "/guilds/"+guildID+"/roles/"+roleID); err != nil {
		return fmt.Errorf("DeleteRole %q in guild %q: %w", roleID, guildID, err)
	}
	return nil
}

// EditChannelPermissions creates or updates a permission overwrite on a channel.
func (c *HTTPClient) EditChannelPermissions(channelID, overwriteID string, params EditChannelPermissionsParams) error {
	if _, err := c.request("PUT", "/channels/"+channelID+"/permissions/"+overwriteID, params); err != nil {
		return fmt.Errorf("EditChannelPermissions channel %q overwrite %q: %w", channelID, overwriteID, err)
	}
	return nil
}

// DeleteChannelPermission removes a permission overwrite from a channel.
func (c *HTTPClient) DeleteChannelPermission(channelID, overwriteID string) error {
	if err := c.requestNoBody("DELETE", "/channels/"+channelID+"/permissions/"+overwriteID); err != nil {
		return fmt.Errorf("DeleteChannelPermission channel %q overwrite %q: %w", channelID, overwriteID, err)
	}
	return nil
}
