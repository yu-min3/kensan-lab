package e2e

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// HTTPClient is a helper for making HTTP requests in tests
type HTTPClient struct {
	t      *testing.T
	token  string
	client *http.Client
}

// NewHTTPClient creates a new test HTTP client
func NewHTTPClient(t *testing.T) *HTTPClient {
	return &HTTPClient{
		t:      t,
		client: &http.Client{},
	}
}

// SetToken sets the authentication token
func (c *HTTPClient) SetToken(token string) {
	c.token = token
}

// Get makes a GET request
func (c *HTTPClient) Get(url string) *Response {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(c.t, err)
	return c.do(req)
}

// GetRaw makes a GET request without testing assertions (for health checks)
func (c *HTTPClient) GetRaw(url string) *Response {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return &Response{StatusCode: 0}
	}

	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return &Response{StatusCode: 0}
	}

	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	return &Response{
		StatusCode: resp.StatusCode,
		Body:       body,
		Headers:    resp.Header,
	}
}

// Post makes a POST request with JSON body
func (c *HTTPClient) Post(url string, body interface{}) *Response {
	return c.doWithBody(http.MethodPost, url, body)
}

// Put makes a PUT request with JSON body
func (c *HTTPClient) Put(url string, body interface{}) *Response {
	return c.doWithBody(http.MethodPut, url, body)
}

// Delete makes a DELETE request
func (c *HTTPClient) Delete(url string) *Response {
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	require.NoError(c.t, err)
	return c.do(req)
}

// doWithBody makes a request with JSON body
func (c *HTTPClient) doWithBody(method, url string, body interface{}) *Response {
	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		require.NoError(c.t, err)
		bodyReader = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	require.NoError(c.t, err)
	req.Header.Set("Content-Type", "application/json")

	return c.do(req)
}

// do executes the request
func (c *HTTPClient) do(req *http.Request) *Response {
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.client.Do(req)
	require.NoError(c.t, err)

	body, err := io.ReadAll(resp.Body)
	require.NoError(c.t, err)
	resp.Body.Close()

	return &Response{
		t:          c.t,
		StatusCode: resp.StatusCode,
		Body:       body,
		Headers:    resp.Header,
	}
}

// Response represents an HTTP response
type Response struct {
	t          *testing.T
	StatusCode int
	Body       []byte
	Headers    http.Header
}

// AssertStatus asserts the response status code
func (r *Response) AssertStatus(expected int) *Response {
	require.Equal(r.t, expected, r.StatusCode, "unexpected status code, body: %s", string(r.Body))
	return r
}

// JSON decodes the response body as JSON
func (r *Response) JSON(v interface{}) *Response {
	err := json.Unmarshal(r.Body, v)
	require.NoError(r.t, err, "failed to decode JSON: %s", string(r.Body))
	return r
}

// String returns the response body as string
func (r *Response) String() string {
	return string(r.Body)
}

// APIResponse wraps all API responses (data is wrapped in "data" field)
type APIResponse[T any] struct {
	Data T `json:"data"`
	Meta struct {
		RequestID string `json:"requestId"`
		Timestamp string `json:"timestamp"`
	} `json:"meta"`
}

// RegisterRequest represents a user registration request
type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

// LoginRequest represents a user login request
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// AuthResponseData represents authentication response data
type AuthResponseData struct {
	Token string `json:"token"`
	User  struct {
		ID    string `json:"id"`
		Email string `json:"email"`
		Name  string `json:"name"`
	} `json:"user"`
}

// AuthResponse is the full API response for auth
type AuthResponse struct {
	Data AuthResponseData `json:"data"`
	Meta struct {
		RequestID string `json:"requestId"`
		Timestamp string `json:"timestamp"`
	} `json:"meta"`
}


// ProfileResponseData represents user profile data
type ProfileResponseData struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

// ProfileResponse is the full API response for profile
type ProfileResponse struct {
	Data ProfileResponseData `json:"data"`
}

// SettingsResponseData represents user settings data
type SettingsResponseData struct {
	Timezone     string `json:"timezone"`
	Theme        string `json:"theme"`
	IsConfigured bool   `json:"isConfigured"`
}

// SettingsResponse is the full API response for settings
type SettingsResponse struct {
	Data SettingsResponseData `json:"data"`
}

// UpdateSettingsRequest represents settings update request
type UpdateSettingsRequest struct {
	Timezone *string `json:"timezone,omitempty"`
	Theme    *string `json:"theme,omitempty"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}
