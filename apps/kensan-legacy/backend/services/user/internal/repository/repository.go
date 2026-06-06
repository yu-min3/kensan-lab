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
	"github.com/kensan/backend/shared/vault"
)

var (
	ErrUserNotFound     = errors.New("user not found")
	ErrUserExists       = errors.New("user already exists")
	ErrSettingsNotFound = errors.New("settings not found")
)

// Ensure PostgresRepository implements Repository interface
var _ Repository = (*PostgresRepository)(nil)

// PostgresRepository handles user data persistence with PostgreSQL.
//
// users.name は Vault Transit で暗号化して name_enc (BYTEA, ciphertext) に保存し、
// 検索性のため HMAC を name_hash (BYTEA) に保存する (Stage 6, Phase 1: 完全一致のみ)。
// Service / Handler 層からは透過 — 既存 User.Name フィールドにアクセスするだけで
// 自動的に encrypt/decrypt される。
type PostgresRepository struct {
	pool      *pgxpool.Pool
	encryptor vault.Encryptor
}

// NewPostgresRepository creates a new PostgreSQL user repository.
//
// encryptor で users.name の encrypt/decrypt/HMAC を行う。Vault が無い環境では
// vault.NoOpEncryptor{} を渡せば passthrough で動く (dev のみ、prod では必ず
// 本物の vault.Client を渡すこと)。
func NewPostgresRepository(pool *pgxpool.Pool, encryptor vault.Encryptor) *PostgresRepository {
	return &PostgresRepository{pool: pool, encryptor: encryptor}
}

// GetByID retrieves a user by ID
func (r *PostgresRepository) GetByID(ctx context.Context, id string) (*user.User, error) {
	query := `
		SELECT id, email, name_enc, password_hash, created_at, updated_at
		FROM users
		WHERE id = $1
	`

	var u user.User
	var nameEnc []byte
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&u.ID,
		&u.Email,
		&nameEnc,
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

	if err := r.decryptName(ctx, nameEnc, &u); err != nil {
		return nil, err
	}
	return &u, nil
}

// GetByEmail retrieves a user by email
func (r *PostgresRepository) GetByEmail(ctx context.Context, email string) (*user.User, error) {
	query := `
		SELECT id, email, name_enc, password_hash, created_at, updated_at
		FROM users
		WHERE email = $1
	`

	var u user.User
	var nameEnc []byte
	err := r.pool.QueryRow(ctx, query, email).Scan(
		&u.ID,
		&u.Email,
		&nameEnc,
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

	if err := r.decryptName(ctx, nameEnc, &u); err != nil {
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

	nameEnc, nameHash, err := r.encryptName(ctx, u.Name)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO users (id, email, name_enc, name_hash, password_hash, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err = r.pool.Exec(ctx, query,
		u.ID,
		u.Email,
		nameEnc,
		nameHash,
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

	nameEnc, nameHash, err := r.encryptName(ctx, u.Name)
	if err != nil {
		return err
	}

	query := `
		UPDATE users
		SET email = $2, name_enc = $3, name_hash = $4, password_hash = $5, updated_at = $6
		WHERE id = $1
	`

	result, err := r.pool.Exec(ctx, query,
		u.ID,
		u.Email,
		nameEnc,
		nameHash,
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

// encryptName encrypts u.Name with Vault Transit and computes its HMAC.
// Returns ciphertext bytes (BYTEA) and HMAC bytes (BYTEA) ready for SQL.
//
// users.name is required (CHECK NOT NULL upstream), so empty plaintext is an
// invariant violation — but to keep this layer defensive we surface the error
// from the encryptor (NoOpEncryptor / *Client both reject empty input).
func (r *PostgresRepository) encryptName(ctx context.Context, name string) (enc, hmac []byte, err error) {
	ct, err := r.encryptor.Encrypt(ctx, []byte(name))
	if err != nil {
		return nil, nil, err
	}
	hm, err := r.encryptor.HMAC(ctx, []byte(name))
	if err != nil {
		return nil, nil, err
	}
	return []byte(ct), []byte(hm), nil
}

// decryptName decrypts the BYTEA column into u.Name. nil/empty bytea is treated
// as empty name (legitimate transitional state during migration backfill).
func (r *PostgresRepository) decryptName(ctx context.Context, enc []byte, u *user.User) error {
	if len(enc) == 0 {
		u.Name = ""
		return nil
	}
	pt, err := r.encryptor.Decrypt(ctx, string(enc))
	if err != nil {
		return err
	}
	u.Name = string(pt)
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
