package radist

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// jsonHandler returns an HTTP handler that responds with a JSON body and status.
func jsonHandler(t *testing.T, status int, body any) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if err := json.NewEncoder(w).Encode(body); err != nil {
			t.Errorf("failed to encode response: %v", err)
		}
	})
}

// requestCapture is an HTTP handler that records the incoming request and
// delegates to an inner handler.
type requestCapture struct {
	inner   http.Handler
	method  string
	path    string
	headers http.Header
	body    []byte
}

func (rc *requestCapture) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rc.method = r.Method
	rc.path = r.URL.EscapedPath()
	rc.headers = r.Header.Clone()
	if r.Body != nil {
		rc.body, _ = io.ReadAll(r.Body)
	}
	rc.inner.ServeHTTP(w, r)
}

func newTestClient(t *testing.T, handler http.Handler, opts ...Option) (*Client, *requestCapture) {
	t.Helper()

	cap := &requestCapture{inner: handler}
	srv := httptest.NewServer(cap)
	t.Cleanup(srv.Close)

	allOpts := append([]Option{
		WithAPIBaseURL(srv.URL),
		WithProjectID("project_123"),
		WithHTTPClient(srv.Client()),
	}, opts...)

	client, err := NewClient("rad_sk_test", allOpts...)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	return client, cap
}

func assertAuthorization(t *testing.T, cap *requestCapture) {
	t.Helper()
	if got := cap.headers.Get("Authorization"); got != "Bearer rad_sk_test" {
		t.Errorf("Authorization header = %q, want %q", got, "Bearer rad_sk_test")
	}
	if got := cap.headers.Get("Accept"); got != "application/json" {
		t.Errorf("Accept header = %q, want %q", got, "application/json")
	}
}

// --- NewClient ---

func TestNewClient_MissingToken(t *testing.T) {
	t.Setenv("RADIST_KEY", "")
	_, err := NewClient("")
	if err == nil {
		t.Fatal("expected error for missing secret token, got nil")
	}
}

func TestNewClient_TokenFromEnv(t *testing.T) {
	t.Setenv("RADIST_KEY", "rad_sk_from_env")
	t.Setenv("RADIST_PROJECT_ID", "")
	c, err := NewClient("")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if c.secretToken != "rad_sk_from_env" {
		t.Errorf("secretToken = %q, want %q", c.secretToken, "rad_sk_from_env")
	}
}

func TestNewClient_TrailingSlashStripped(t *testing.T) {
	c, err := NewClient("rad_sk_test", WithAPIBaseURL("https://api.example.com///"))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if c.apiBaseURL != "https://api.example.com" {
		t.Errorf("apiBaseURL = %q, want %q", c.apiBaseURL, "https://api.example.com")
	}
}

// --- CreateP2PConnection ---

func TestCreateP2PConnection_Success(t *testing.T) {
	handler := jsonHandler(t, http.StatusOK, map[string]any{
		"callId":        "call_123",
		"callTokens":    []string{"tok_a", "tok_b"},
		"accessControl": "private",
	})
	client, cap := newTestClient(t, handler)

	conn, err := client.CreateP2PConnection(context.Background(), nil)
	if err != nil {
		t.Fatalf("CreateP2PConnection: %v", err)
	}

	if conn.CallID != "call_123" {
		t.Errorf("CallID = %q, want %q", conn.CallID, "call_123")
	}
	if len(conn.CallTokens) != 2 || conn.CallTokens[0] != "tok_a" || conn.CallTokens[1] != "tok_b" {
		t.Errorf("CallTokens = %v, want [tok_a tok_b]", conn.CallTokens)
	}
	if conn.AccessControl != "private" {
		t.Errorf("AccessControl = %q, want %q", conn.AccessControl, "private")
	}
	if cap.method != http.MethodPost {
		t.Errorf("method = %q, want POST", cap.method)
	}
	if cap.path != "/api/v1/projects/project_123/calls" {
		t.Errorf("path = %q, want /api/v1/projects/project_123/calls", cap.path)
	}
	assertAuthorization(t, cap)

	var reqBody map[string]string
	if err := json.Unmarshal(cap.body, &reqBody); err != nil {
		t.Fatalf("failed to parse request body: %v", err)
	}
	if reqBody["accessControl"] != "private" {
		t.Errorf("request body accessControl = %q, want %q", reqBody["accessControl"], "private")
	}
}

func TestCreateP2PConnection_ProjectIDEncoded(t *testing.T) {
	handler := jsonHandler(t, http.StatusOK, map[string]any{
		"callId":        "call_123",
		"callTokens":    []string{"tok_a", "tok_b"},
		"accessControl": "private",
	})
	client, cap := newTestClient(t, handler, WithProjectID("project one"))

	if _, err := client.CreateP2PConnection(context.Background(), nil); err != nil {
		t.Fatalf("CreateP2PConnection: %v", err)
	}
	if cap.path != "/api/v1/projects/project%20one/calls" {
		t.Errorf("path = %q, want /api/v1/projects/project%%20one/calls", cap.path)
	}
}

func TestCreateP2PConnection_OverrideProjectID(t *testing.T) {
	handler := jsonHandler(t, http.StatusOK, map[string]any{
		"callId":        "call_abc",
		"callTokens":    []string{"tok_a", "tok_b"},
		"accessControl": "private",
	})
	client, cap := newTestClient(t, handler)

	opts := &CreateP2PConnectionOptions{ProjectID: "other_project"}
	if _, err := client.CreateP2PConnection(context.Background(), opts); err != nil {
		t.Fatalf("CreateP2PConnection: %v", err)
	}
	if cap.path != "/api/v1/projects/other_project/calls" {
		t.Errorf("path = %q, want /api/v1/projects/other_project/calls", cap.path)
	}
}

func TestCreateP2PConnection_AccessControlPublic(t *testing.T) {
	handler := jsonHandler(t, http.StatusOK, map[string]any{
		"callId":        "call_123",
		"callTokens":    []string{"tok_a", "tok_b"},
		"accessControl": "public",
	})
	client, cap := newTestClient(t, handler)

	opts := &CreateP2PConnectionOptions{AccessControl: "public"}
	conn, err := client.CreateP2PConnection(context.Background(), opts)
	if err != nil {
		t.Fatalf("CreateP2PConnection: %v", err)
	}
	if conn.AccessControl != "public" {
		t.Errorf("AccessControl = %q, want %q", conn.AccessControl, "public")
	}

	var reqBody map[string]string
	if err := json.Unmarshal(cap.body, &reqBody); err != nil {
		t.Fatalf("failed to parse request body: %v", err)
	}
	if reqBody["accessControl"] != "public" {
		t.Errorf("request body accessControl = %q, want %q", reqBody["accessControl"], "public")
	}
}

func TestCreateP2PConnection_APIError(t *testing.T) {
	handler := jsonHandler(t, http.StatusUnauthorized, map[string]any{
		"error": "invalid token",
	})
	client, _ := newTestClient(t, handler)

	_, err := client.CreateP2PConnection(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error type = %T, want *APIError", err)
	}
	if apiErr.Status != http.StatusUnauthorized {
		t.Errorf("Status = %d, want %d", apiErr.Status, http.StatusUnauthorized)
	}
}

func TestCreateP2PConnection_MalformedResponse(t *testing.T) {
	handler := jsonHandler(t, http.StatusOK, map[string]any{
		"callTokens": []string{},
	})
	client, _ := newTestClient(t, handler)

	_, err := client.CreateP2PConnection(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for malformed response, got nil")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error type = %T, want *APIError", err)
	}
}

func TestCreateP2PConnection_MissingProjectID(t *testing.T) {
	c, err := NewClient("rad_sk_test")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	_, err = c.CreateP2PConnection(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for missing project ID, got nil")
	}
}

// --- CreateRoom ---

func TestCreateRoom_Success(t *testing.T) {
	handler := jsonHandler(t, http.StatusOK, map[string]any{
		"roomId":     "room_123",
		"roomTokens": []string{"room_a", "room_b"},
	})
	client, cap := newTestClient(t, handler)

	room, err := client.CreateRoom(context.Background(), nil)
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}

	if room.RoomID != "room_123" {
		t.Errorf("RoomID = %q, want %q", room.RoomID, "room_123")
	}
	if len(room.RoomTokens) != 2 {
		t.Errorf("RoomTokens length = %d, want 2", len(room.RoomTokens))
	}
	if cap.path != "/api/v1/projects/project_123/rooms" {
		t.Errorf("path = %q, want /api/v1/projects/project_123/rooms", cap.path)
	}
	assertAuthorization(t, cap)
}

func TestCreateRoom_APIError(t *testing.T) {
	handler := jsonHandler(t, http.StatusForbidden, map[string]any{
		"error": "forbidden",
	})
	client, _ := newTestClient(t, handler)

	_, err := client.CreateRoom(context.Background(), nil)
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

// --- MintConnectionToken ---

func TestMintConnectionToken_CallID(t *testing.T) {
	handler := jsonHandler(t, http.StatusOK, map[string]any{
		"callToken": "call_token_123",
	})
	client, cap := newTestClient(t, handler)

	tok, err := client.MintConnectionToken(context.Background(), MintConnectionTokenOptions{
		CallID: "call/123",
	})
	if err != nil {
		t.Fatalf("MintConnectionToken: %v", err)
	}
	if tok.Token != "call_token_123" {
		t.Errorf("Token = %q, want %q", tok.Token, "call_token_123")
	}
	if cap.path != "/api/v1/projects/project_123/p2p/calls/call%2F123/token" {
		t.Errorf("path = %q, want /api/v1/projects/project_123/p2p/calls/call%%2F123/token", cap.path)
	}
}

func TestMintConnectionToken_RoomID(t *testing.T) {
	handler := jsonHandler(t, http.StatusOK, map[string]any{
		"roomToken": "room_token_123",
	})
	client, cap := newTestClient(t, handler)

	tok, err := client.MintConnectionToken(context.Background(), MintConnectionTokenOptions{
		RoomID: "room/123",
	})
	if err != nil {
		t.Fatalf("MintConnectionToken: %v", err)
	}
	if tok.Token != "room_token_123" {
		t.Errorf("Token = %q, want %q", tok.Token, "room_token_123")
	}
	if cap.path != "/api/v1/projects/project_123/rooms/room%2F123/token" {
		t.Errorf("path = %q, want /api/v1/projects/project_123/rooms/room%%2F123/token", cap.path)
	}
}

func TestMintConnectionToken_NeitherCallNorRoom(t *testing.T) {
	client, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	_, err := client.MintConnectionToken(context.Background(), MintConnectionTokenOptions{})
	if err == nil {
		t.Fatal("expected error when neither CallID nor RoomID provided")
	}
}

func TestMintConnectionToken_APIError(t *testing.T) {
	handler := jsonHandler(t, http.StatusNotFound, map[string]any{
		"error": "call not found",
	})
	client, _ := newTestClient(t, handler)

	_, err := client.MintConnectionToken(context.Background(), MintConnectionTokenOptions{CallID: "missing"})
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error type = %T, want *APIError", err)
	}
	if apiErr.Status != http.StatusNotFound {
		t.Errorf("Status = %d, want %d", apiErr.Status, http.StatusNotFound)
	}
}
