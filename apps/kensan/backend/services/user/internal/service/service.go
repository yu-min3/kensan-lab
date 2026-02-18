package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/kensan/backend/services/user/internal"
	"github.com/kensan/backend/services/user/internal/repository"
	"github.com/kensan/backend/shared/auth"
	sharedErrors "github.com/kensan/backend/shared/errors"
	"golang.org/x/crypto/bcrypt"
)

// Service-specific errors
var (
	ErrInvalidCredentials = fmt.Errorf("credentials: %w", sharedErrors.ErrUnauthorized)
	ErrEmailRequired      = sharedErrors.Required("email")
	ErrPasswordRequired   = sharedErrors.Required("password")
	ErrNameRequired       = sharedErrors.Required("name")
	ErrInvalidEmail       = sharedErrors.InvalidFormat("email", "valid email address")
	ErrPasswordTooShort   = fmt.Errorf("password: %w (must be at least 8 characters)", sharedErrors.ErrInvalidInput)
	ErrInvalidTheme       = fmt.Errorf("theme: %w (must be light, dark, or system)", sharedErrors.ErrInvalidInput)
	ErrUserNotFound       = sharedErrors.NotFound("user")
	ErrUserExists         = sharedErrors.AlreadyExists("user")
)

// bcrypt cost for password hashing
const bcryptCost = 12

// email validation regex
var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)

// Service handles user business logic
type Service struct {
	repo       repository.Repository
	jwtManager *auth.JWTManager
}

// NewService creates a new user service
func NewService(repo repository.Repository, jwtManager *auth.JWTManager) *Service {
	return &Service{
		repo:       repo,
		jwtManager: jwtManager,
	}
}

// Register creates a new user account
func (s *Service) Register(ctx context.Context, req *user.RegisterRequest) (*user.AuthResponse, error) {
	// Validate input
	if err := s.validateRegisterRequest(req); err != nil {
		return nil, err
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcryptCost)
	if err != nil {
		return nil, err
	}

	// Create user
	u := &user.User{
		Email:    strings.ToLower(strings.TrimSpace(req.Email)),
		Name:     strings.TrimSpace(req.Name),
		Password: string(hashedPassword),
	}

	if err := s.repo.Create(ctx, u); err != nil {
		if errors.Is(err, repository.ErrUserExists) {
			return nil, ErrUserExists
		}
		return nil, err
	}

	// Generate JWT token
	token, err := s.jwtManager.GenerateToken(u.ID, u.Email)
	if err != nil {
		return nil, err
	}

	return &user.AuthResponse{
		Token: token,
		User:  u,
	}, nil
}

// Login authenticates a user and returns a JWT token
func (s *Service) Login(ctx context.Context, req *user.LoginRequest) (*user.AuthResponse, error) {
	// Validate input
	if req.Email == "" {
		return nil, ErrEmailRequired
	}
	if req.Password == "" {
		return nil, ErrPasswordRequired
	}

	// Find user by email
	u, err := s.repo.GetByEmail(ctx, strings.ToLower(strings.TrimSpace(req.Email)))
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(req.Password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	// Generate JWT token
	token, err := s.jwtManager.GenerateToken(u.ID, u.Email)
	if err != nil {
		return nil, err
	}

	return &user.AuthResponse{
		Token: token,
		User:  u,
	}, nil
}

// GetProfile retrieves a user's profile
func (s *Service) GetProfile(ctx context.Context, userID string) (*user.User, error) {
	u, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return u, nil
}

// UpdateProfile updates a user's profile
func (s *Service) UpdateProfile(ctx context.Context, userID string, req *user.UpdateProfileRequest) (*user.User, error) {
	// Get existing user
	u, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	// Update fields
	if req.Name != "" {
		u.Name = strings.TrimSpace(req.Name)
	}
	if req.Email != "" {
		email := strings.ToLower(strings.TrimSpace(req.Email))
		if !emailRegex.MatchString(email) {
			return nil, ErrInvalidEmail
		}

		// Check if email is already taken by another user
		if email != u.Email {
			existingUser, err := s.repo.GetByEmail(ctx, email)
			if err != nil && !errors.Is(err, repository.ErrUserNotFound) {
				return nil, err
			}
			if existingUser != nil {
				return nil, ErrUserExists
			}
		}
		u.Email = email
	}

	// Save changes
	if err := s.repo.Update(ctx, u); err != nil {
		return nil, err
	}

	return u, nil
}

// GetSettings retrieves a user's settings
func (s *Service) GetSettings(ctx context.Context, userID string) (*user.SettingsResponse, error) {
	settings, err := s.repo.GetSettings(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrSettingsNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	response := &user.SettingsResponse{
		UserID:         settings.UserID,
		Timezone:       settings.Timezone,
		Theme:          settings.Theme,
		IsConfigured:   settings.IsConfigured,
		AIEnabled:      settings.AIEnabled,
		AIConsentGiven: settings.AIConsentGiven,
	}

	return response, nil
}

// UpdateSettings updates a user's settings
func (s *Service) UpdateSettings(ctx context.Context, userID string, req *user.UpdateSettingsRequest) (*user.SettingsResponse, error) {
	// Get existing settings
	settings, err := s.repo.GetSettings(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrSettingsNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	// Update fields
	if req.Timezone != "" {
		settings.Timezone = req.Timezone
	}
	if req.Theme != "" {
		if req.Theme != "light" && req.Theme != "dark" && req.Theme != "system" {
			return nil, ErrInvalidTheme
		}
		settings.Theme = req.Theme
	}
	if req.AIEnabled != nil {
		settings.AIEnabled = *req.AIEnabled
	}

	// IsConfigured is true if timezone is set
	settings.IsConfigured = settings.Timezone != ""

	// Save changes
	if err := s.repo.UpdateSettings(ctx, settings); err != nil {
		return nil, err
	}

	return s.GetSettings(ctx, userID)
}

// GiveAIConsent records the user's AI consent
func (s *Service) GiveAIConsent(ctx context.Context, userID string, consent bool) (*user.SettingsResponse, error) {
	// Get existing settings
	settings, err := s.repo.GetSettings(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrSettingsNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	// Update consent
	settings.AIConsentGiven = consent
	if !consent {
		// If consent is revoked, disable AI
		settings.AIEnabled = false
	}

	// Save changes
	if err := s.repo.UpdateSettings(ctx, settings); err != nil {
		return nil, err
	}

	return s.GetSettings(ctx, userID)
}

// validateRegisterRequest validates the registration request
func (s *Service) validateRegisterRequest(req *user.RegisterRequest) error {
	if req.Email == "" {
		return ErrEmailRequired
	}
	if !emailRegex.MatchString(strings.TrimSpace(req.Email)) {
		return ErrInvalidEmail
	}
	if req.Password == "" {
		return ErrPasswordRequired
	}
	if len(req.Password) < 8 {
		return ErrPasswordTooShort
	}
	if req.Name == "" {
		return ErrNameRequired
	}
	return nil
}

