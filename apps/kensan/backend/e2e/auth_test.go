package e2e

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAuthFlow tests the complete authentication flow:
// 1. Register a new user
// 2. Login with the new user
// 3. Get user profile with token
// 4. Update user profile
func TestAuthFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	// Setup test environment
	env := SetupTestEnv(t)
	defer env.Cleanup(t)

	// Start user service
	env.StartService(t, "user", 18081)
	userURL := env.ServiceURL("user")

	client := NewHTTPClient(t)

	// Test 1: Register a new user
	t.Run("Register", func(t *testing.T) {
		var authResp AuthResponse

		client.Post(userURL+"/api/v1/auth/register", RegisterRequest{
			Email:    "e2e-test@example.com",
			Password: "password123",
			Name:     "E2E Test User",
		}).AssertStatus(http.StatusCreated).JSON(&authResp)

		require.NotEmpty(t, authResp.Data.Token)
		assert.Equal(t, "e2e-test@example.com", authResp.Data.User.Email)
		assert.Equal(t, "E2E Test User", authResp.Data.User.Name)
		assert.NotEmpty(t, authResp.Data.User.ID)

		// Save token for subsequent requests
		client.SetToken(authResp.Data.Token)
	})

	// Test 2: Login with the registered user
	t.Run("Login", func(t *testing.T) {
		// Create new client without token
		loginClient := NewHTTPClient(t)

		var authResp AuthResponse
		loginClient.Post(userURL+"/api/v1/auth/login", LoginRequest{
			Email:    "e2e-test@example.com",
			Password: "password123",
		}).AssertStatus(http.StatusOK).JSON(&authResp)

		require.NotEmpty(t, authResp.Data.Token)
		assert.Equal(t, "e2e-test@example.com", authResp.Data.User.Email)
	})

	// Test 3: Get user profile
	t.Run("GetProfile", func(t *testing.T) {
		var profile ProfileResponse

		client.Get(userURL + "/api/v1/users/me").
			AssertStatus(http.StatusOK).
			JSON(&profile)

		assert.Equal(t, "e2e-test@example.com", profile.Data.Email)
		assert.Equal(t, "E2E Test User", profile.Data.Name)
	})

	// Test 4: Get user settings
	t.Run("GetSettings", func(t *testing.T) {
		var settings SettingsResponse

		client.Get(userURL + "/api/v1/users/me/settings").
			AssertStatus(http.StatusOK).
			JSON(&settings)

		// Default settings
		assert.Equal(t, "Asia/Tokyo", settings.Data.Timezone)
		assert.Equal(t, "system", settings.Data.Theme)
		assert.False(t, settings.Data.IsConfigured)
	})

	// Test 5: Update user settings
	t.Run("UpdateSettings", func(t *testing.T) {
		theme := "dark"
		var settings SettingsResponse

		client.Put(userURL+"/api/v1/users/me/settings", UpdateSettingsRequest{
			Theme: &theme,
		}).AssertStatus(http.StatusOK).JSON(&settings)

		assert.Equal(t, "dark", settings.Data.Theme)
	})

	// Test 6: Unauthorized access without token
	t.Run("UnauthorizedAccess", func(t *testing.T) {
		noAuthClient := NewHTTPClient(t)

		noAuthClient.Get(userURL + "/api/v1/users/me").
			AssertStatus(http.StatusUnauthorized)
	})
}

// TestAuthValidation tests authentication validation
func TestAuthValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	env := SetupTestEnv(t)
	defer env.Cleanup(t)

	env.StartService(t, "user", 18082)
	userURL := env.ServiceURL("user")

	client := NewHTTPClient(t)

	t.Run("RegisterWithInvalidEmail", func(t *testing.T) {
		client.Post(userURL+"/api/v1/auth/register", RegisterRequest{
			Email:    "invalid-email",
			Password: "password123",
			Name:     "Test User",
		}).AssertStatus(http.StatusBadRequest)
	})

	t.Run("RegisterWithShortPassword", func(t *testing.T) {
		client.Post(userURL+"/api/v1/auth/register", RegisterRequest{
			Email:    "test@example.com",
			Password: "short",
			Name:     "Test User",
		}).AssertStatus(http.StatusBadRequest)
	})

	t.Run("RegisterWithEmptyName", func(t *testing.T) {
		client.Post(userURL+"/api/v1/auth/register", RegisterRequest{
			Email:    "test@example.com",
			Password: "password123",
			Name:     "",
		}).AssertStatus(http.StatusBadRequest)
	})

	t.Run("LoginWithWrongPassword", func(t *testing.T) {
		// First register
		client.Post(userURL+"/api/v1/auth/register", RegisterRequest{
			Email:    "login-test@example.com",
			Password: "correctpassword",
			Name:     "Test User",
		}).AssertStatus(http.StatusCreated)

		// Then try to login with wrong password
		client.Post(userURL+"/api/v1/auth/login", LoginRequest{
			Email:    "login-test@example.com",
			Password: "wrongpassword",
		}).AssertStatus(http.StatusUnauthorized)
	})

	t.Run("LoginWithNonexistentUser", func(t *testing.T) {
		client.Post(userURL+"/api/v1/auth/login", LoginRequest{
			Email:    "nonexistent@example.com",
			Password: "password123",
		}).AssertStatus(http.StatusUnauthorized)
	})
}

// TestDuplicateRegistration tests that duplicate registration is rejected
func TestDuplicateRegistration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	env := SetupTestEnv(t)
	defer env.Cleanup(t)

	env.StartService(t, "user", 18083)
	userURL := env.ServiceURL("user")

	client := NewHTTPClient(t)

	// Register first user
	client.Post(userURL+"/api/v1/auth/register", RegisterRequest{
		Email:    "duplicate@example.com",
		Password: "password123",
		Name:     "First User",
	}).AssertStatus(http.StatusCreated)

	// Try to register with same email
	client.Post(userURL+"/api/v1/auth/register", RegisterRequest{
		Email:    "duplicate@example.com",
		Password: "password456",
		Name:     "Second User",
	}).AssertStatus(http.StatusConflict)
}
