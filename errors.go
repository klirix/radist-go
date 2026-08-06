package radist

import "fmt"

// APIError is returned when the Radist API responds with a non-success status
// or an unexpected response body.
type APIError struct {
	// Status is the HTTP status code returned by the API.
	Status int
	// Message is the human-readable error description.
	Message string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("radist: API error (status %d): %s", e.Status, e.Message)
}
