package service

import (
	"context"

	user "github.com/kensan/backend/services/user/internal"
)

// AuthService defines the interface for authentication operations
type AuthService interface {
	Register(ctx context.Context, req *user.RegisterRequest) (*user.AuthResponse, error)
	Login(ctx context.Context, req *user.LoginRequest) (*user.AuthResponse, error)
}

// ProfileService defines the interface for user profile operations
type ProfileService interface {
	GetProfile(ctx context.Context, userID string) (*user.User, error)
	UpdateProfile(ctx context.Context, userID string, req *user.UpdateProfileRequest) (*user.User, error)
}

// SettingsService defines the interface for user settings operations
type SettingsService interface {
	GetSettings(ctx context.Context, userID string) (*user.SettingsResponse, error)
	UpdateSettings(ctx context.Context, userID string, req *user.UpdateSettingsRequest) (*user.SettingsResponse, error)
	GiveAIConsent(ctx context.Context, userID string, consent bool) (*user.SettingsResponse, error)
}

// FullService is the combined interface that embeds all user service interfaces
type FullService interface {
	AuthService
	ProfileService
	SettingsService
}

// Compile-time check to ensure Service implements FullService
var _ FullService = (*Service)(nil)
