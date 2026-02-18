package user

import (
	"time"
)

// User represents a user in the system
type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Password  string    `json:"-"` // Never expose password hash in JSON
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// UserSettings represents user-specific settings
type UserSettings struct {
	UserID         string `json:"userId"`
	Timezone       string `json:"timezone"`
	Theme          string `json:"theme"` // "light", "dark", "system"
	IsConfigured   bool   `json:"isConfigured"`
	AIEnabled      bool   `json:"aiEnabled"`
	AIConsentGiven bool   `json:"aiConsentGiven"`
}

// RegisterRequest represents the registration request payload
type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

// LoginRequest represents the login request payload
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// AuthResponse represents the authentication response
type AuthResponse struct {
	Token string `json:"token"`
	User  *User  `json:"user"`
}

// UpdateProfileRequest represents the profile update request payload
type UpdateProfileRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// UpdateSettingsRequest represents the settings update request payload
type UpdateSettingsRequest struct {
	Timezone  string `json:"timezone,omitempty"`
	Theme     string `json:"theme,omitempty"`
	AIEnabled *bool  `json:"aiEnabled,omitempty"`
}

// SettingsResponse represents the settings response
type SettingsResponse struct {
	UserID         string `json:"userId"`
	Timezone       string `json:"timezone"`
	Theme          string `json:"theme"`
	IsConfigured   bool   `json:"isConfigured"`
	AIEnabled      bool   `json:"aiEnabled"`
	AIConsentGiven bool   `json:"aiConsentGiven"`
}

// AIConsentRequest represents the AI consent request payload
type AIConsentRequest struct {
	Consent bool `json:"consent"`
}
