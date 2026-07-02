package repository

import (
	"context"

	"github.com/kensan/backend/services/user/internal"
)

// Repository defines the interface for user data persistence
type Repository interface {
	// GetByID retrieves a user by ID
	GetByID(ctx context.Context, id string) (*user.User, error)

	// GetByEmail retrieves a user by email
	GetByEmail(ctx context.Context, email string) (*user.User, error)

	// Create creates a new user
	Create(ctx context.Context, u *user.User) error

	// Update updates an existing user
	Update(ctx context.Context, u *user.User) error

	// Delete deletes a user and their settings
	Delete(ctx context.Context, userID string) error

	// GetSettings retrieves user settings
	GetSettings(ctx context.Context, userID string) (*user.UserSettings, error)

	// UpdateSettings updates user settings
	UpdateSettings(ctx context.Context, s *user.UserSettings) error
}
