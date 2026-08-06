// Package radist provides a server-side client for the Radist API,
// enabling creation of P2P calls, SFU rooms, and connection tokens
// from trusted server environments.
package radist

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
)

const defaultAPIBaseURL = "https://radist.tech"

// Client is a server-side Radist API client. Create one with [NewClient].
type Client struct {
	secretToken string
	projectID   string
	apiBaseURL  string
	httpClient  *http.Client
}

// Option configures a [Client].
type Option func(*Client)

// WithProjectID sets the default project ID used when one is not provided
// to individual request methods. Overrides RADIST_PROJECT_ID.
func WithProjectID(projectID string) Option {
	return func(c *Client) {
		c.projectID = strings.TrimSpace(projectID)
	}
}

// WithAPIBaseURL overrides the Radist API base URL. Trailing slashes are removed.
func WithAPIBaseURL(apiBaseURL string) Option {
	return func(c *Client) {
		c.apiBaseURL = strings.TrimRight(strings.TrimSpace(apiBaseURL), "/")
	}
}

// WithHTTPClient replaces the default [http.DefaultClient] used for requests.
func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

// NewClient creates a new Radist server-side client.
//
// secretToken may be empty, in which case the RADIST_KEY environment variable
// is used. If neither is set, NewClient returns an error.
//
// The default project ID is read from RADIST_PROJECT_ID unless overridden
// with [WithProjectID].
func NewClient(secretToken string, opts ...Option) (*Client, error) {
	resolved := strings.TrimSpace(secretToken)
	if resolved == "" {
		resolved = strings.TrimSpace(os.Getenv("RADIST_KEY"))
	}
	if resolved == "" {
		return nil, fmt.Errorf("radist: missing secret token; pass it to NewClient or set RADIST_KEY")
	}

	apiBaseURL := strings.TrimSpace(os.Getenv("RADIST_API_BASE_URL"))
	if apiBaseURL == "" {
		apiBaseURL = defaultAPIBaseURL
	}

	c := &Client{
		secretToken: resolved,
		projectID:   strings.TrimSpace(os.Getenv("RADIST_PROJECT_ID")),
		apiBaseURL:  strings.TrimRight(apiBaseURL, "/"),
		httpClient:  http.DefaultClient,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c, nil
}

// P2PConnection is the result of [Client.CreateP2PConnection].
type P2PConnection struct {
	// CallID is the unique identifier for this call.
	CallID string
	// CallTokens contains two server-minted participant tokens.
	// Give one token to each approved participant.
	CallTokens []string
	// AccessControl is the access policy for this call: "public" or "private".
	// Private calls (the default) only accept tokens minted server-side.
	AccessControl string
}

// Room is the result of [Client.CreateRoom].
type Room struct {
	// RoomID is the unique identifier for this SFU room.
	RoomID string
	// RoomTokens contains two server-minted participant tokens.
	RoomTokens []string
}

// ConnectionToken is the result of [Client.MintConnectionToken].
type ConnectionToken struct {
	// Token is the minted connection token.
	Token string
}

// CreateP2PConnectionOptions holds per-call options for [Client.CreateP2PConnection].
// All fields are optional; a nil pointer is treated as an empty value.
type CreateP2PConnectionOptions struct {
	// ProjectID overrides the client-level project ID for this call only.
	ProjectID string
	// AccessControl sets the access policy for the call. Use "public" to allow
	// anyone with a public key to join, or "private" (the default) to restrict
	// token minting to your server.
	AccessControl string
}

// CreateRoomOptions holds per-call options for [Client.CreateRoom].
// All fields are optional; a nil pointer is treated as an empty value.
type CreateRoomOptions struct {
	// ProjectID overrides the client-level project ID for this call only.
	ProjectID string
}

// MintConnectionTokenOptions holds options for [Client.MintConnectionToken].
// Provide exactly one of CallID or RoomID.
type MintConnectionTokenOptions struct {
	// ProjectID overrides the client-level project ID for this call only.
	ProjectID string
	// CallID mints a token for a P2P call created by [Client.CreateP2PConnection].
	CallID string
	// RoomID mints a token for an SFU room created by [Client.CreateRoom].
	RoomID string
}

// CreateP2PConnection creates a new P2P call and returns its ID and two participant tokens.
func (c *Client) CreateP2PConnection(ctx context.Context, opts *CreateP2PConnectionOptions) (*P2PConnection, error) {
	projectID, err := c.resolveProjectID(opts.projectID())
	if err != nil {
		return nil, err
	}

	accessControl := opts.accessControl()
	body := map[string]string{"accessControl": accessControl}

	path := "/api/v1/projects/" + url.PathEscape(projectID) + "/calls"
	status, respBody, err := c.post(ctx, path, body)
	if err != nil {
		return nil, err
	}

	var data struct {
		CallID        string   `json:"callId"`
		CallTokens    []string `json:"callTokens"`
		AccessControl string   `json:"accessControl"`
		Error         string   `json:"error"`
	}
	_ = json.Unmarshal(respBody, &data)

	if status < 200 || status >= 300 || data.CallID == "" || len(data.CallTokens) < 2 {
		return nil, newAPIError(status, data.Error)
	}

	return &P2PConnection{CallID: data.CallID, CallTokens: data.CallTokens, AccessControl: data.AccessControl}, nil
}

// CreateRoom creates a new SFU room and returns its ID and two participant tokens.
func (c *Client) CreateRoom(ctx context.Context, opts *CreateRoomOptions) (*Room, error) {
	projectID, err := c.resolveProjectID(opts.projectID())
	if err != nil {
		return nil, err
	}

	path := "/api/v1/projects/" + url.PathEscape(projectID) + "/rooms"
	status, body, err := c.post(ctx, path, nil)
	if err != nil {
		return nil, err
	}

	var data struct {
		RoomID     string   `json:"roomId"`
		RoomTokens []string `json:"roomTokens"`
		Error      string   `json:"error"`
	}
	_ = json.Unmarshal(body, &data)

	if status < 200 || status >= 300 || data.RoomID == "" || len(data.RoomTokens) < 2 {
		return nil, newAPIError(status, data.Error)
	}

	return &Room{RoomID: data.RoomID, RoomTokens: data.RoomTokens}, nil
}

// MintConnectionToken mints a connection token for a P2P call or SFU room.
// Exactly one of opts.CallID or opts.RoomID must be set.
func (c *Client) MintConnectionToken(ctx context.Context, opts MintConnectionTokenOptions) (*ConnectionToken, error) {
	if opts.CallID == "" && opts.RoomID == "" {
		return nil, fmt.Errorf("radist: provide either CallID or RoomID")
	}

	projectID, err := c.resolveProjectID(opts.ProjectID)
	if err != nil {
		return nil, err
	}

	var path string
	if opts.CallID != "" {
		path = "/api/v1/projects/" + url.PathEscape(projectID) + "/p2p/calls/" + url.PathEscape(opts.CallID) + "/token"
	} else {
		path = "/api/v1/projects/" + url.PathEscape(projectID) + "/rooms/" + url.PathEscape(opts.RoomID) + "/token"
	}

	status, body, err := c.post(ctx, path, nil)
	if err != nil {
		return nil, err
	}

	var data struct {
		CallToken string `json:"callToken"`
		RoomToken string `json:"roomToken"`
		Error     string `json:"error"`
	}
	_ = json.Unmarshal(body, &data)

	token := data.CallToken
	if token == "" {
		token = data.RoomToken
	}

	if status < 200 || status >= 300 || token == "" {
		return nil, newAPIError(status, data.Error)
	}

	return &ConnectionToken{Token: token}, nil
}

func (c *Client) post(ctx context.Context, path string, body any) (int, []byte, error) {
	var bodyReader io.Reader
	extraHeaders := map[string]string{}
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return 0, nil, fmt.Errorf("radist: failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(payload)
		extraHeaders["Content-Type"] = "application/json"
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.apiBaseURL+path, bodyReader)
	if err != nil {
		return 0, nil, fmt.Errorf("radist: failed to build request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.secretToken)
	for k, v := range extraHeaders {
		req.Header.Set(k, v)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("radist: request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, fmt.Errorf("radist: failed to read response body: %w", err)
	}

	return resp.StatusCode, body, nil
}

func (c *Client) resolveProjectID(override string) (string, error) {
	id := strings.TrimSpace(override)
	if id == "" {
		id = c.projectID
	}
	if id == "" {
		return "", fmt.Errorf("radist: missing project ID; pass it via options or set RADIST_PROJECT_ID")
	}
	return id, nil
}

func newAPIError(status int, message string) *APIError {
	if message == "" {
		message = fmt.Sprintf("Radist API request failed with status %d", status)
	}
	return &APIError{Status: status, Message: message}
}

func (o *CreateP2PConnectionOptions) projectID() string {
	if o == nil {
		return ""
	}
	return o.ProjectID
}

func (o *CreateP2PConnectionOptions) accessControl() string {
	if o == nil || o.AccessControl == "" {
		return "private"
	}
	return o.AccessControl
}

func (o *CreateRoomOptions) projectID() string {
	if o == nil {
		return ""
	}
	return o.ProjectID
}
