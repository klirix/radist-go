package radist

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
)

// SpaceAccessType is the access policy for a persistent [Space].
type SpaceAccessType string

const (
	// SpaceAccessPublic allows anyone with the space's participant URL to join.
	SpaceAccessPublic SpaceAccessType = "public"
	// SpaceAccessPassword requires a password to join the space.
	SpaceAccessPassword SpaceAccessType = "password"
	// SpaceAccessKnock requires host approval before a participant may join.
	SpaceAccessKnock SpaceAccessType = "knock"
)

// Space is a persistent, reusable meeting space that can be joined
// repeatedly without minting a new call or room each time.
type Space struct {
	// ID is the unique identifier for this space.
	ID string `json:"id"`
	// ProjectID is the project this space belongs to.
	ProjectID string `json:"projectId"`
	// Slug is the URL-friendly identifier for this space.
	Slug string `json:"slug"`
	// Name is the human-readable display name for this space.
	Name string `json:"name"`
	// AccessType is the access policy for this space.
	AccessType SpaceAccessType `json:"accessType"`
	// ActiveRoomID is the ID of the room currently backing this space, if any.
	ActiveRoomID *string `json:"activeRoomId"`
	// CreatedAt is the ISO timestamp this space was created.
	CreatedAt string `json:"createdAt"`
	// UpdatedAt is the ISO timestamp this space was last updated.
	UpdatedAt string `json:"updatedAt"`
	// ParticipantURL is the URL participants use to join this space.
	// It is present in [Client.ListSpaces] results but absent elsewhere.
	ParticipantURL string `json:"participantUrl,omitempty"`
}

// CreateSpaceOptions holds options for [Client.CreateSpace].
type CreateSpaceOptions struct {
	// ProjectID overrides the client-level project ID for this call only.
	ProjectID string
	// Name is the human-readable display name for the space.
	Name string
	// AccessType sets the access policy for the space.
	AccessType SpaceAccessType
	// Password sets the join password. Required when AccessType is
	// [SpaceAccessPassword]; omitted from the request when empty.
	Password string
}

// UpdateSpaceOptions holds options for [Client.UpdateSpace]. Only the fields
// that are set (non-empty) are sent in the update request; all fields are optional.
type UpdateSpaceOptions struct {
	// ProjectID overrides the client-level project ID for this call only.
	ProjectID string
	// Slug, if set, updates the space's URL-friendly identifier.
	Slug string
	// Name, if set, updates the space's display name.
	Name string
	// AccessType, if set, updates the space's access policy.
	AccessType SpaceAccessType
	// Password, if set, updates the space's join password.
	Password string
}

// SpaceOptions holds per-call options shared by [Client.ListSpaces],
// [Client.GetSpace], [Client.DeleteSpace], and [Client.RotateSpaceHostToken].
// All fields are optional; a nil pointer is treated as an empty value.
type SpaceOptions struct {
	// ProjectID overrides the client-level project ID for this call only.
	ProjectID string
}

// SpaceCreated is the result of [Client.CreateSpace].
type SpaceCreated struct {
	// Space is the newly created space.
	Space Space
	// ParticipantURL is the URL participants use to join the space.
	ParticipantURL string
	// HostURL is the URL the host uses to join the space with host privileges.
	HostURL string
	// HostToken is the minted host token for the space.
	HostToken string
}

// SpaceHostToken is the result of [Client.RotateSpaceHostToken].
type SpaceHostToken struct {
	// ParticipantURL is the URL participants use to join the space.
	ParticipantURL string
	// HostURL is the URL the host uses to join the space with host privileges.
	HostURL string
	// HostToken is the newly minted host token for the space.
	HostToken string
}

// CreateSpace creates a new persistent Space and returns it along with join
// URLs and a host token.
func (c *Client) CreateSpace(ctx context.Context, opts CreateSpaceOptions) (*SpaceCreated, error) {
	projectID, err := c.resolveProjectID(opts.ProjectID)
	if err != nil {
		return nil, err
	}

	body := map[string]any{
		"name":       opts.Name,
		"accessType": opts.AccessType,
	}
	if opts.Password != "" {
		body["password"] = opts.Password
	}

	path := "/api/v1/projects/" + url.PathEscape(projectID) + "/spaces"
	status, respBody, err := c.request(ctx, http.MethodPost, path, body)
	if err != nil {
		return nil, err
	}

	var data struct {
		Space          Space  `json:"space"`
		ParticipantURL string `json:"participantUrl"`
		HostURL        string `json:"hostUrl"`
		HostToken      string `json:"hostToken"`
		Error          string `json:"error"`
	}
	_ = json.Unmarshal(respBody, &data)

	if status < 200 || status >= 300 || data.Space.ID == "" || data.HostToken == "" {
		return nil, newAPIError(status, data.Error)
	}

	return &SpaceCreated{
		Space:          data.Space,
		ParticipantURL: data.ParticipantURL,
		HostURL:        data.HostURL,
		HostToken:      data.HostToken,
	}, nil
}

// ListSpaces lists all persistent Spaces in a project.
func (c *Client) ListSpaces(ctx context.Context, opts *SpaceOptions) ([]Space, error) {
	projectID, err := c.resolveProjectID(opts.projectID())
	if err != nil {
		return nil, err
	}

	path := "/api/v1/projects/" + url.PathEscape(projectID) + "/spaces"
	status, respBody, err := c.request(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var data struct {
		Spaces []Space `json:"spaces"`
		Error  string  `json:"error"`
	}
	_ = json.Unmarshal(respBody, &data)

	if status < 200 || status >= 300 {
		return nil, newAPIError(status, data.Error)
	}

	return data.Spaces, nil
}

// GetSpace fetches a single Space by ID. The backend has no dedicated
// get-by-ID endpoint, so this lists all spaces in the project and finds the
// matching one; it returns a 404 [APIError] if no space with that ID exists.
func (c *Client) GetSpace(ctx context.Context, spaceID string, opts *SpaceOptions) (*Space, error) {
	spaces, err := c.ListSpaces(ctx, opts)
	if err != nil {
		return nil, err
	}

	for i := range spaces {
		if spaces[i].ID == spaceID {
			return &spaces[i], nil
		}
	}

	return nil, newAPIError(http.StatusNotFound, "Space not found.")
}

// UpdateSpace updates a Space's mutable fields. Only the fields set on opts
// (non-empty) are sent in the update request.
func (c *Client) UpdateSpace(ctx context.Context, spaceID string, opts UpdateSpaceOptions) (*Space, error) {
	projectID, err := c.resolveProjectID(opts.ProjectID)
	if err != nil {
		return nil, err
	}

	body := map[string]any{}
	if opts.Slug != "" {
		body["slug"] = opts.Slug
	}
	if opts.Name != "" {
		body["name"] = opts.Name
	}
	if opts.AccessType != "" {
		body["accessType"] = opts.AccessType
	}
	if opts.Password != "" {
		body["password"] = opts.Password
	}

	path := "/api/v1/projects/" + url.PathEscape(projectID) + "/spaces/" + url.PathEscape(spaceID)
	status, respBody, err := c.request(ctx, http.MethodPatch, path, body)
	if err != nil {
		return nil, err
	}

	var data struct {
		Space Space  `json:"space"`
		Error string `json:"error"`
	}
	_ = json.Unmarshal(respBody, &data)

	if status < 200 || status >= 300 {
		return nil, newAPIError(status, data.Error)
	}

	return &data.Space, nil
}

// DeleteSpace deletes a Space by ID.
func (c *Client) DeleteSpace(ctx context.Context, spaceID string, opts *SpaceOptions) error {
	projectID, err := c.resolveProjectID(opts.projectID())
	if err != nil {
		return err
	}

	path := "/api/v1/projects/" + url.PathEscape(projectID) + "/spaces/" + url.PathEscape(spaceID)
	status, respBody, err := c.request(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return err
	}

	if status < 200 || status >= 300 {
		var data struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(respBody, &data)
		return newAPIError(status, data.Error)
	}

	return nil
}

// RotateSpaceHostToken invalidates a Space's current host token and mints a
// new one, returning the refreshed join URLs and token.
func (c *Client) RotateSpaceHostToken(ctx context.Context, spaceID string, opts *SpaceOptions) (*SpaceHostToken, error) {
	projectID, err := c.resolveProjectID(opts.projectID())
	if err != nil {
		return nil, err
	}

	path := "/api/v1/projects/" + url.PathEscape(projectID) + "/spaces/" + url.PathEscape(spaceID) + "/rotate-host-token"
	status, respBody, err := c.request(ctx, http.MethodPost, path, nil)
	if err != nil {
		return nil, err
	}

	var data struct {
		ParticipantURL string `json:"participantUrl"`
		HostURL        string `json:"hostUrl"`
		HostToken      string `json:"hostToken"`
		Error          string `json:"error"`
	}
	_ = json.Unmarshal(respBody, &data)

	if status < 200 || status >= 300 || data.HostToken == "" {
		return nil, newAPIError(status, data.Error)
	}

	return &SpaceHostToken{
		ParticipantURL: data.ParticipantURL,
		HostURL:        data.HostURL,
		HostToken:      data.HostToken,
	}, nil
}

func (o *SpaceOptions) projectID() string {
	if o == nil {
		return ""
	}
	return o.ProjectID
}
