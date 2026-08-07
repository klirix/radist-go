package radist

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
)

// --- CreateSpace ---

func TestCreateSpace_Success(t *testing.T) {
	handler := jsonHandler(t, http.StatusOK, map[string]any{
		"space": map[string]any{
			"id":           "space_123",
			"projectId":    "project_123",
			"slug":         "team-standup",
			"name":         "Team Standup",
			"accessType":   "public",
			"activeRoomId": nil,
			"createdAt":    "2026-01-01T00:00:00Z",
			"updatedAt":    "2026-01-01T00:00:00Z",
		},
		"participantUrl": "https://radist.tech/s/team-standup",
		"hostUrl":        "https://radist.tech/s/team-standup?host=1",
		"hostToken":      "host_tok_123",
	})
	client, cap := newTestClient(t, handler)

	created, err := client.CreateSpace(context.Background(), CreateSpaceOptions{
		Name:       "Team Standup",
		AccessType: SpaceAccessPublic,
	})
	if err != nil {
		t.Fatalf("CreateSpace: %v", err)
	}

	if created.Space.ID != "space_123" {
		t.Errorf("Space.ID = %q, want %q", created.Space.ID, "space_123")
	}
	if created.Space.AccessType != SpaceAccessPublic {
		t.Errorf("Space.AccessType = %q, want %q", created.Space.AccessType, SpaceAccessPublic)
	}
	if created.Space.ParticipantURL != "" {
		t.Errorf("Space.ParticipantURL = %q, want empty (create response space omits it)", created.Space.ParticipantURL)
	}
	if created.ParticipantURL != "https://radist.tech/s/team-standup" {
		t.Errorf("ParticipantURL = %q, want %q", created.ParticipantURL, "https://radist.tech/s/team-standup")
	}
	if created.HostToken != "host_tok_123" {
		t.Errorf("HostToken = %q, want %q", created.HostToken, "host_tok_123")
	}
	if cap.method != http.MethodPost {
		t.Errorf("method = %q, want POST", cap.method)
	}
	if cap.path != "/api/v1/projects/project_123/spaces" {
		t.Errorf("path = %q, want /api/v1/projects/project_123/spaces", cap.path)
	}
	assertAuthorization(t, cap)

	var reqBody map[string]any
	if err := json.Unmarshal(cap.body, &reqBody); err != nil {
		t.Fatalf("failed to parse request body: %v", err)
	}
	if _, ok := reqBody["password"]; ok {
		t.Errorf("request body should not include password key when unset, got %v", reqBody)
	}
	if reqBody["name"] != "Team Standup" {
		t.Errorf("request body name = %v, want %q", reqBody["name"], "Team Standup")
	}
	if reqBody["accessType"] != "public" {
		t.Errorf("request body accessType = %v, want %q", reqBody["accessType"], "public")
	}
}

func TestCreateSpace_PasswordIncludedWhenSet(t *testing.T) {
	handler := jsonHandler(t, http.StatusOK, map[string]any{
		"space": map[string]any{
			"id":         "space_123",
			"projectId":  "project_123",
			"slug":       "secret-room",
			"name":       "Secret Room",
			"accessType": "password",
			"createdAt":  "2026-01-01T00:00:00Z",
			"updatedAt":  "2026-01-01T00:00:00Z",
		},
		"participantUrl": "https://radist.tech/s/secret-room",
		"hostUrl":        "https://radist.tech/s/secret-room?host=1",
		"hostToken":      "host_tok_456",
	})
	client, cap := newTestClient(t, handler)

	_, err := client.CreateSpace(context.Background(), CreateSpaceOptions{
		Name:       "Secret Room",
		AccessType: SpaceAccessPassword,
		Password:   "hunter2",
	})
	if err != nil {
		t.Fatalf("CreateSpace: %v", err)
	}

	var reqBody map[string]any
	if err := json.Unmarshal(cap.body, &reqBody); err != nil {
		t.Fatalf("failed to parse request body: %v", err)
	}
	if reqBody["password"] != "hunter2" {
		t.Errorf("request body password = %v, want %q", reqBody["password"], "hunter2")
	}
}

func TestCreateSpace_APIError(t *testing.T) {
	handler := jsonHandler(t, http.StatusForbidden, map[string]any{
		"error": "forbidden",
	})
	client, _ := newTestClient(t, handler)

	_, err := client.CreateSpace(context.Background(), CreateSpaceOptions{
		Name:       "Team Standup",
		AccessType: SpaceAccessPublic,
	})
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error type = %T, want *APIError", err)
	}
	if apiErr.Status != http.StatusForbidden {
		t.Errorf("Status = %d, want %d", apiErr.Status, http.StatusForbidden)
	}
	if apiErr.Message != "forbidden" {
		t.Errorf("Message = %q, want %q", apiErr.Message, "forbidden")
	}
}

func TestCreateSpace_MissingProjectID(t *testing.T) {
	c, err := NewClient("rad_sk_test")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	_, err = c.CreateSpace(context.Background(), CreateSpaceOptions{Name: "x", AccessType: SpaceAccessPublic})
	if err == nil {
		t.Fatal("expected error for missing project ID, got nil")
	}
}

// --- ListSpaces ---

func TestListSpaces_Success(t *testing.T) {
	handler := jsonHandler(t, http.StatusOK, map[string]any{
		"spaces": []map[string]any{
			{
				"id":             "space_1",
				"projectId":      "project_123",
				"slug":           "one",
				"name":           "One",
				"accessType":     "public",
				"activeRoomId":   nil,
				"createdAt":      "2026-01-01T00:00:00Z",
				"updatedAt":      "2026-01-01T00:00:00Z",
				"participantUrl": "https://radist.tech/s/one",
			},
			{
				"id":             "space_2",
				"projectId":      "project_123",
				"slug":           "two",
				"name":           "Two",
				"accessType":     "knock",
				"activeRoomId":   "room_9",
				"createdAt":      "2026-01-02T00:00:00Z",
				"updatedAt":      "2026-01-02T00:00:00Z",
				"participantUrl": "https://radist.tech/s/two",
			},
		},
	})
	client, cap := newTestClient(t, handler)

	spaces, err := client.ListSpaces(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListSpaces: %v", err)
	}
	if len(spaces) != 2 {
		t.Fatalf("len(spaces) = %d, want 2", len(spaces))
	}
	if spaces[0].ParticipantURL != "https://radist.tech/s/one" {
		t.Errorf("spaces[0].ParticipantURL = %q, want %q", spaces[0].ParticipantURL, "https://radist.tech/s/one")
	}
	if spaces[1].ActiveRoomID == nil || *spaces[1].ActiveRoomID != "room_9" {
		t.Errorf("spaces[1].ActiveRoomID = %v, want room_9", spaces[1].ActiveRoomID)
	}
	if cap.method != http.MethodGet {
		t.Errorf("method = %q, want GET", cap.method)
	}
	if cap.path != "/api/v1/projects/project_123/spaces" {
		t.Errorf("path = %q, want /api/v1/projects/project_123/spaces", cap.path)
	}
	assertAuthorization(t, cap)
}

func TestListSpaces_APIError(t *testing.T) {
	handler := jsonHandler(t, http.StatusUnauthorized, map[string]any{
		"error": "invalid token",
	})
	client, _ := newTestClient(t, handler)

	_, err := client.ListSpaces(context.Background(), nil)
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error type = %T, want *APIError", err)
	}
	if apiErr.Status != http.StatusUnauthorized {
		t.Errorf("Status = %d, want %d", apiErr.Status, http.StatusUnauthorized)
	}
}

// --- GetSpace ---

func TestGetSpace_Found(t *testing.T) {
	handler := jsonHandler(t, http.StatusOK, map[string]any{
		"spaces": []map[string]any{
			{
				"id":         "space_1",
				"projectId":  "project_123",
				"slug":       "one",
				"name":       "One",
				"accessType": "public",
				"createdAt":  "2026-01-01T00:00:00Z",
				"updatedAt":  "2026-01-01T00:00:00Z",
			},
			{
				"id":         "space_2",
				"projectId":  "project_123",
				"slug":       "two",
				"name":       "Two",
				"accessType": "knock",
				"createdAt":  "2026-01-02T00:00:00Z",
				"updatedAt":  "2026-01-02T00:00:00Z",
			},
		},
	})
	client, _ := newTestClient(t, handler)

	space, err := client.GetSpace(context.Background(), "space_2", nil)
	if err != nil {
		t.Fatalf("GetSpace: %v", err)
	}
	if space.ID != "space_2" {
		t.Errorf("ID = %q, want %q", space.ID, "space_2")
	}
	if space.Name != "Two" {
		t.Errorf("Name = %q, want %q", space.Name, "Two")
	}
}

func TestGetSpace_NotFound(t *testing.T) {
	handler := jsonHandler(t, http.StatusOK, map[string]any{
		"spaces": []map[string]any{
			{
				"id":         "space_1",
				"projectId":  "project_123",
				"slug":       "one",
				"name":       "One",
				"accessType": "public",
				"createdAt":  "2026-01-01T00:00:00Z",
				"updatedAt":  "2026-01-01T00:00:00Z",
			},
		},
	})
	client, _ := newTestClient(t, handler)

	_, err := client.GetSpace(context.Background(), "space_missing", nil)
	if err == nil {
		t.Fatal("expected error for missing space, got nil")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error type = %T, want *APIError", err)
	}
	if apiErr.Status != http.StatusNotFound {
		t.Errorf("Status = %d, want %d", apiErr.Status, http.StatusNotFound)
	}
	if apiErr.Message != "Space not found." {
		t.Errorf("Message = %q, want %q", apiErr.Message, "Space not found.")
	}
}

// --- UpdateSpace ---

func TestUpdateSpace_OnlySendsProvidedFields(t *testing.T) {
	handler := jsonHandler(t, http.StatusOK, map[string]any{
		"space": map[string]any{
			"id":         "space_1",
			"projectId":  "project_123",
			"slug":       "one",
			"name":       "New Name",
			"accessType": "public",
			"createdAt":  "2026-01-01T00:00:00Z",
			"updatedAt":  "2026-01-03T00:00:00Z",
		},
	})
	client, cap := newTestClient(t, handler)

	space, err := client.UpdateSpace(context.Background(), "space_1", UpdateSpaceOptions{
		Name: "New Name",
	})
	if err != nil {
		t.Fatalf("UpdateSpace: %v", err)
	}
	if space.Name != "New Name" {
		t.Errorf("Name = %q, want %q", space.Name, "New Name")
	}
	if cap.method != http.MethodPatch {
		t.Errorf("method = %q, want PATCH", cap.method)
	}
	if cap.path != "/api/v1/projects/project_123/spaces/space_1" {
		t.Errorf("path = %q, want /api/v1/projects/project_123/spaces/space_1", cap.path)
	}

	var reqBody map[string]any
	if err := json.Unmarshal(cap.body, &reqBody); err != nil {
		t.Fatalf("failed to parse request body: %v", err)
	}
	if len(reqBody) != 1 {
		t.Fatalf("request body = %v, want exactly one key (name)", reqBody)
	}
	if reqBody["name"] != "New Name" {
		t.Errorf("request body name = %v, want %q", reqBody["name"], "New Name")
	}
}

func TestUpdateSpace_AllFields(t *testing.T) {
	handler := jsonHandler(t, http.StatusOK, map[string]any{
		"space": map[string]any{
			"id":         "space_1",
			"projectId":  "project_123",
			"slug":       "new-slug",
			"name":       "New Name",
			"accessType": "password",
			"createdAt":  "2026-01-01T00:00:00Z",
			"updatedAt":  "2026-01-03T00:00:00Z",
		},
	})
	client, cap := newTestClient(t, handler)

	_, err := client.UpdateSpace(context.Background(), "space_1", UpdateSpaceOptions{
		Slug:       "new-slug",
		Name:       "New Name",
		AccessType: SpaceAccessPassword,
		Password:   "newpass",
	})
	if err != nil {
		t.Fatalf("UpdateSpace: %v", err)
	}

	var reqBody map[string]any
	if err := json.Unmarshal(cap.body, &reqBody); err != nil {
		t.Fatalf("failed to parse request body: %v", err)
	}
	if len(reqBody) != 4 {
		t.Fatalf("request body = %v, want exactly 4 keys", reqBody)
	}
}

func TestUpdateSpace_APIError(t *testing.T) {
	handler := jsonHandler(t, http.StatusNotFound, map[string]any{
		"error": "space not found",
	})
	client, _ := newTestClient(t, handler)

	_, err := client.UpdateSpace(context.Background(), "space_missing", UpdateSpaceOptions{Name: "x"})
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error type = %T, want *APIError", err)
	}
	if apiErr.Status != http.StatusNotFound {
		t.Errorf("Status = %d, want %d", apiErr.Status, http.StatusNotFound)
	}
}

// --- DeleteSpace ---

func TestDeleteSpace_Success(t *testing.T) {
	handler := jsonHandler(t, http.StatusOK, map[string]any{"success": true})
	client, cap := newTestClient(t, handler)

	err := client.DeleteSpace(context.Background(), "space_1", nil)
	if err != nil {
		t.Fatalf("DeleteSpace: %v", err)
	}
	if cap.method != http.MethodDelete {
		t.Errorf("method = %q, want DELETE", cap.method)
	}
	if cap.path != "/api/v1/projects/project_123/spaces/space_1" {
		t.Errorf("path = %q, want /api/v1/projects/project_123/spaces/space_1", cap.path)
	}
	assertAuthorization(t, cap)
}

func TestDeleteSpace_APIError(t *testing.T) {
	handler := jsonHandler(t, http.StatusForbidden, map[string]any{"error": "forbidden"})
	client, _ := newTestClient(t, handler)

	err := client.DeleteSpace(context.Background(), "space_1", nil)
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error type = %T, want *APIError", err)
	}
	if apiErr.Status != http.StatusForbidden {
		t.Errorf("Status = %d, want %d", apiErr.Status, http.StatusForbidden)
	}
}

// --- RotateSpaceHostToken ---

func TestRotateSpaceHostToken_Success(t *testing.T) {
	handler := jsonHandler(t, http.StatusOK, map[string]any{
		"participantUrl": "https://radist.tech/s/one",
		"hostUrl":        "https://radist.tech/s/one?host=1",
		"hostToken":      "host_tok_new",
	})
	client, cap := newTestClient(t, handler)

	tok, err := client.RotateSpaceHostToken(context.Background(), "space_1", nil)
	if err != nil {
		t.Fatalf("RotateSpaceHostToken: %v", err)
	}
	if tok.HostToken != "host_tok_new" {
		t.Errorf("HostToken = %q, want %q", tok.HostToken, "host_tok_new")
	}
	if tok.ParticipantURL != "https://radist.tech/s/one" {
		t.Errorf("ParticipantURL = %q, want %q", tok.ParticipantURL, "https://radist.tech/s/one")
	}
	if cap.method != http.MethodPost {
		t.Errorf("method = %q, want POST", cap.method)
	}
	if cap.path != "/api/v1/projects/project_123/spaces/space_1/rotate-host-token" {
		t.Errorf("path = %q, want /api/v1/projects/project_123/spaces/space_1/rotate-host-token", cap.path)
	}
}

func TestRotateSpaceHostToken_APIError(t *testing.T) {
	handler := jsonHandler(t, http.StatusNotFound, map[string]any{"error": "space not found"})
	client, _ := newTestClient(t, handler)

	_, err := client.RotateSpaceHostToken(context.Background(), "space_missing", nil)
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error type = %T, want *APIError", err)
	}
	if apiErr.Status != http.StatusNotFound {
		t.Errorf("Status = %d, want %d", apiErr.Status, http.StatusNotFound)
	}
}

// --- Project ID / path escaping ---

func TestSpaces_ProjectIDEncoded(t *testing.T) {
	handler := jsonHandler(t, http.StatusOK, map[string]any{"spaces": []map[string]any{}})
	client, cap := newTestClient(t, handler, WithProjectID("project one"))

	if _, err := client.ListSpaces(context.Background(), nil); err != nil {
		t.Fatalf("ListSpaces: %v", err)
	}
	if cap.path != "/api/v1/projects/project%20one/spaces" {
		t.Errorf("path = %q, want /api/v1/projects/project%%20one/spaces", cap.path)
	}
}

func TestSpaces_SpaceIDEncoded(t *testing.T) {
	handler := jsonHandler(t, http.StatusOK, map[string]any{"success": true})
	client, cap := newTestClient(t, handler)

	if err := client.DeleteSpace(context.Background(), "space/1", nil); err != nil {
		t.Fatalf("DeleteSpace: %v", err)
	}
	if cap.path != "/api/v1/projects/project_123/spaces/space%2F1" {
		t.Errorf("path = %q, want /api/v1/projects/project_123/spaces/space%%2F1", cap.path)
	}
}
