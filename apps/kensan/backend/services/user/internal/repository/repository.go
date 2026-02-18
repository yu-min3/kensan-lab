package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kensan/backend/services/user/internal"
	sharedErrors "github.com/kensan/backend/shared/errors"
)

var (
	ErrUserNotFound     = errors.New("user not found")
	ErrUserExists       = errors.New("user already exists")
	ErrSettingsNotFound = errors.New("settings not found")
)

// Ensure PostgresRepository implements Repository interface
var _ Repository = (*PostgresRepository)(nil)

// PostgresRepository handles user data persistence with PostgreSQL
type PostgresRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresRepository creates a new PostgreSQL user repository
func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

// GetByID retrieves a user by ID
func (r *PostgresRepository) GetByID(ctx context.Context, id string) (*user.User, error) {
	query := `
		SELECT id, email, name, password_hash, created_at, updated_at
		FROM users
		WHERE id = $1
	`

	var u user.User
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&u.ID,
		&u.Email,
		&u.Name,
		&u.Password,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	return &u, nil
}

// GetByEmail retrieves a user by email
func (r *PostgresRepository) GetByEmail(ctx context.Context, email string) (*user.User, error) {
	query := `
		SELECT id, email, name, password_hash, created_at, updated_at
		FROM users
		WHERE email = $1
	`

	var u user.User
	err := r.pool.QueryRow(ctx, query, email).Scan(
		&u.ID,
		&u.Email,
		&u.Name,
		&u.Password,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	return &u, nil
}

// Create creates a new user
func (r *PostgresRepository) Create(ctx context.Context, u *user.User) error {
	// Generate UUID if not set
	if u.ID == "" {
		u.ID = uuid.New().String()
	}

	now := time.Now()
	u.CreatedAt = now
	u.UpdatedAt = now

	query := `
		INSERT INTO users (id, email, name, password_hash, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`

	_, err := r.pool.Exec(ctx, query,
		u.ID,
		u.Email,
		u.Name,
		u.Password,
		u.CreatedAt,
		u.UpdatedAt,
	)
	if err != nil {
		// Check for unique constraint violation (email already exists)
		if sharedErrors.IsUniqueViolation(err) {
			return ErrUserExists
		}
		return err
	}

	// Create default settings for the user
	return r.createDefaultSettings(ctx, u.ID)
}

// Update updates an existing user
func (r *PostgresRepository) Update(ctx context.Context, u *user.User) error {
	u.UpdatedAt = time.Now()

	query := `
		UPDATE users
		SET email = $2, name = $3, password_hash = $4, updated_at = $5
		WHERE id = $1
	`

	result, err := r.pool.Exec(ctx, query,
		u.ID,
		u.Email,
		u.Name,
		u.Password,
		u.UpdatedAt,
	)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrUserNotFound
	}

	return nil
}

// GetSettings retrieves user settings
func (r *PostgresRepository) GetSettings(ctx context.Context, userID string) (*user.UserSettings, error) {
	query := `
		SELECT user_id, timezone, theme, is_configured, ai_enabled, ai_consent_given
		FROM user_settings
		WHERE user_id = $1
	`

	var s user.UserSettings

	err := r.pool.QueryRow(ctx, query, userID).Scan(
		&s.UserID,
		&s.Timezone,
		&s.Theme,
		&s.IsConfigured,
		&s.AIEnabled,
		&s.AIConsentGiven,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrSettingsNotFound
		}
		return nil, err
	}

	return &s, nil
}

// UpdateSettings updates user settings
func (r *PostgresRepository) UpdateSettings(ctx context.Context, s *user.UserSettings) error {
	query := `
		UPDATE user_settings
		SET timezone = $2, theme = $3, is_configured = $4, ai_enabled = $5, ai_consent_given = $6
		WHERE user_id = $1
	`

	result, err := r.pool.Exec(ctx, query,
		s.UserID,
		s.Timezone,
		s.Theme,
		s.IsConfigured,
		s.AIEnabled,
		s.AIConsentGiven,
	)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrSettingsNotFound
	}

	return nil
}

// createDefaultSettings creates default settings for a new user
func (r *PostgresRepository) createDefaultSettings(ctx context.Context, userID string) error {
	query := `
		INSERT INTO user_settings (user_id, timezone, theme, is_configured, ai_enabled, ai_consent_given)
		VALUES ($1, $2, $3, $4, $5, $6)
	`

	_, err := r.pool.Exec(ctx, query,
		userID,
		"Asia/Tokyo", // Default timezone
		"system",     // Default theme
		false,        // Not configured
		false,        // AI disabled by default
		false,        // AI consent not given
	)
	return err
}

// Delete deletes a user and their settings
func (r *PostgresRepository) Delete(ctx context.Context, userID string) error {
	// Delete settings first (due to foreign key constraint)
	_, err := r.pool.Exec(ctx, "DELETE FROM user_settings WHERE user_id = $1", userID)
	if err != nil {
		return err
	}

	// Delete user
	result, err := r.pool.Exec(ctx, "DELETE FROM users WHERE id = $1", userID)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrUserNotFound
	}

	return nil
}
