package service

import (
	"context"
	"errors"
	"testing"

	user "github.com/kensan/backend/services/user/internal"
	"github.com/kensan/backend/services/user/internal/repository"
	"github.com/kensan/backend/shared/auth"
	"github.com/kensan/backend/shared/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/crypto/bcrypt"
)

// MockRepository is a mock implementation of repository.Repository
type MockRepository struct {
	mock.Mock
}

// Compile-time check that MockRepository implements repository.Repository
var _ repository.Repository = (*MockRepository)(nil)

func (m *MockRepository) Create(ctx context.Context, u *user.User) error {
	args := m.Called(ctx, u)
	if u != nil && u.ID == "" {
		u.ID = "user-new"
	}
	return args.Error(0)
}

func (m *MockRepository) GetByID(ctx context.Context, id string) (*user.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*user.User), args.Error(1)
}

func (m *MockRepository) GetByEmail(ctx context.Context, email string) (*user.User, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*user.User), args.Error(1)
}

func (m *MockRepository) Update(ctx context.Context, u *user.User) error {
	args := m.Called(ctx, u)
	return args.Error(0)
}

func (m *MockRepository) Delete(ctx context.Context, userID string) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *MockRepository) GetSettings(ctx context.Context, userID string) (*user.UserSettings, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*user.UserSettings), args.Error(1)
}

func (m *MockRepository) UpdateSettings(ctx context.Context, settings *user.UserSettings) error {
	args := m.Called(ctx, settings)
	return args.Error(0)
}

// Helper to create a Service with mock repository
func newTestService(repo *MockRepository) *Service {
	jwtManager := auth.NewJWTManager(config.JWTConfig{
		Secret:     "test-secret-key-for-testing-12345",
		Issuer:     "kensan-test",
		ExpireHour: 24,
	})
	return NewService(repo, jwtManager)
}

// ========== Unit Tests for Helper Functions ==========

func TestEmailValidation(t *testing.T) {
	testCases := []struct {
		email    string
		expected bool
	}{
		{"user@example.com", true},
		{"user.name@example.com", true},
		{"user+tag@example.com", true},
		{"user@sub.domain.com", true},
		{"invalid", false},
		{"invalid@", false},
		{"@invalid.com", false},
		{"invalid@.com", false},
		{"", false},
	}

	for _, tc := range testCases {
		t.Run(tc.email, func(t *testing.T) {
			result := emailRegex.MatchString(tc.email)
			assert.Equal(t, tc.expected, result)
		})
	}
}


// ========== Register Tests ==========

func TestRegister_Success(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)
	ctx := context.Background()

	req := &user.RegisterRequest{
		Email:    "newuser@example.com",
		Password: "password123",
		Name:     "New User",
	}

	mockRepo.On("Create", ctx, mock.AnythingOfType("*user.User")).Return(nil)

	resp, err := svc.Register(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotEmpty(t, resp.Token)
	assert.Equal(t, "newuser@example.com", resp.User.Email)
	assert.Equal(t, "New User", resp.User.Name)
	mockRepo.AssertExpectations(t)
}

func TestRegister_EmptyEmail(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)
	ctx := context.Background()

	req := &user.RegisterRequest{
		Email:    "",
		Password: "password123",
		Name:     "Test User",
	}

	resp, err := svc.Register(ctx, req)

	assert.Nil(t, resp)
	assert.ErrorIs(t, err, ErrEmailRequired)
	mockRepo.AssertNotCalled(t, "Create")
}

func TestRegister_InvalidEmail(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)
	ctx := context.Background()

	req := &user.RegisterRequest{
		Email:    "invalid-email",
		Password: "password123",
		Name:     "Test User",
	}

	resp, err := svc.Register(ctx, req)

	assert.Nil(t, resp)
	assert.ErrorIs(t, err, ErrInvalidEmail)
	mockRepo.AssertNotCalled(t, "Create")
}

func TestRegister_ShortPassword(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)
	ctx := context.Background()

	req := &user.RegisterRequest{
		Email:    "user@example.com",
		Password: "short",
		Name:     "Test User",
	}

	resp, err := svc.Register(ctx, req)

	assert.Nil(t, resp)
	assert.ErrorIs(t, err, ErrPasswordTooShort)
	mockRepo.AssertNotCalled(t, "Create")
}

func TestRegister_EmptyPassword(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)
	ctx := context.Background()

	req := &user.RegisterRequest{
		Email:    "user@example.com",
		Password: "",
		Name:     "Test User",
	}

	resp, err := svc.Register(ctx, req)

	assert.Nil(t, resp)
	assert.ErrorIs(t, err, ErrPasswordRequired)
	mockRepo.AssertNotCalled(t, "Create")
}

func TestRegister_EmptyName(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)
	ctx := context.Background()

	req := &user.RegisterRequest{
		Email:    "user@example.com",
		Password: "password123",
		Name:     "",
	}

	resp, err := svc.Register(ctx, req)

	assert.Nil(t, resp)
	assert.ErrorIs(t, err, ErrNameRequired)
	mockRepo.AssertNotCalled(t, "Create")
}

func TestRegister_UserAlreadyExists(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)
	ctx := context.Background()

	req := &user.RegisterRequest{
		Email:    "existing@example.com",
		Password: "password123",
		Name:     "Existing User",
	}

	mockRepo.On("Create", ctx, mock.AnythingOfType("*user.User")).Return(repository.ErrUserExists)

	resp, err := svc.Register(ctx, req)

	assert.Nil(t, resp)
	assert.ErrorIs(t, err, ErrUserExists)
	mockRepo.AssertExpectations(t)
}

// ========== Login Tests ==========

func TestLogin_Success(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)
	ctx := context.Background()

	// Generate bcrypt hash for "password123" at test time
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.MinCost)
	assert.NoError(t, err)

	existingUser := &user.User{
		ID:       "user-123",
		Email:    "test@example.com",
		Name:     "Test User",
		Password: string(hashedPassword),
	}

	req := &user.LoginRequest{
		Email:    "test@example.com",
		Password: "password123",
	}

	mockRepo.On("GetByEmail", ctx, "test@example.com").Return(existingUser, nil)

	resp, err := svc.Login(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotEmpty(t, resp.Token)
	assert.Equal(t, "user-123", resp.User.ID)
	mockRepo.AssertExpectations(t)
}

func TestLogin_EmptyEmail(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)
	ctx := context.Background()

	req := &user.LoginRequest{
		Email:    "",
		Password: "password123",
	}

	resp, err := svc.Login(ctx, req)

	assert.Nil(t, resp)
	assert.ErrorIs(t, err, ErrEmailRequired)
	mockRepo.AssertNotCalled(t, "GetByEmail")
}

func TestLogin_EmptyPassword(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)
	ctx := context.Background()

	req := &user.LoginRequest{
		Email:    "user@example.com",
		Password: "",
	}

	resp, err := svc.Login(ctx, req)

	assert.Nil(t, resp)
	assert.ErrorIs(t, err, ErrPasswordRequired)
	mockRepo.AssertNotCalled(t, "GetByEmail")
}

func TestLogin_UserNotFound(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)
	ctx := context.Background()

	req := &user.LoginRequest{
		Email:    "nonexistent@example.com",
		Password: "password123",
	}

	mockRepo.On("GetByEmail", ctx, "nonexistent@example.com").Return(nil, repository.ErrUserNotFound)

	resp, err := svc.Login(ctx, req)

	assert.Nil(t, resp)
	assert.ErrorIs(t, err, ErrInvalidCredentials)
	mockRepo.AssertExpectations(t)
}

func TestLogin_WrongPassword(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)
	ctx := context.Background()

	// Generate bcrypt hash for "password123" at test time
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.MinCost)
	assert.NoError(t, err)

	existingUser := &user.User{
		ID:       "user-123",
		Email:    "test@example.com",
		Name:     "Test User",
		Password: string(hashedPassword),
	}

	req := &user.LoginRequest{
		Email:    "test@example.com",
		Password: "wrongpassword",
	}

	mockRepo.On("GetByEmail", ctx, "test@example.com").Return(existingUser, nil)

	resp, err := svc.Login(ctx, req)

	assert.Nil(t, resp)
	assert.ErrorIs(t, err, ErrInvalidCredentials)
	mockRepo.AssertExpectations(t)
}

// ========== GetProfile Tests ==========

func TestGetProfile_Success(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)
	ctx := context.Background()
	userID := "user-123"

	expectedUser := &user.User{
		ID:    userID,
		Email: "test@example.com",
		Name:  "Test User",
	}

	mockRepo.On("GetByID", ctx, userID).Return(expectedUser, nil)

	result, err := svc.GetProfile(ctx, userID)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "Test User", result.Name)
	assert.Equal(t, "test@example.com", result.Email)
	mockRepo.AssertExpectations(t)
}

func TestGetProfile_NotFound(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)
	ctx := context.Background()
	userID := "nonexistent-user"

	mockRepo.On("GetByID", ctx, userID).Return(nil, repository.ErrUserNotFound)

	result, err := svc.GetProfile(ctx, userID)

	assert.Nil(t, result)
	assert.ErrorIs(t, err, ErrUserNotFound)
	mockRepo.AssertExpectations(t)
}

// ========== UpdateProfile Tests ==========

func TestUpdateProfile_Success(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)
	ctx := context.Background()
	userID := "user-123"

	existingUser := &user.User{
		ID:    userID,
		Email: "old@example.com",
		Name:  "Old Name",
	}

	req := &user.UpdateProfileRequest{
		Name:  "New Name",
		Email: "new@example.com",
	}

	mockRepo.On("GetByID", ctx, userID).Return(existingUser, nil)
	mockRepo.On("GetByEmail", ctx, "new@example.com").Return(nil, repository.ErrUserNotFound)
	mockRepo.On("Update", ctx, mock.AnythingOfType("*user.User")).Return(nil)

	result, err := svc.UpdateProfile(ctx, userID, req)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "New Name", result.Name)
	assert.Equal(t, "new@example.com", result.Email)
	mockRepo.AssertExpectations(t)
}

func TestUpdateProfile_UserNotFound(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)
	ctx := context.Background()
	userID := "nonexistent-user"

	req := &user.UpdateProfileRequest{
		Name: "New Name",
	}

	mockRepo.On("GetByID", ctx, userID).Return(nil, repository.ErrUserNotFound)

	result, err := svc.UpdateProfile(ctx, userID, req)

	assert.Nil(t, result)
	assert.ErrorIs(t, err, ErrUserNotFound)
	mockRepo.AssertExpectations(t)
}

func TestUpdateProfile_InvalidEmail(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)
	ctx := context.Background()
	userID := "user-123"

	existingUser := &user.User{
		ID:    userID,
		Email: "old@example.com",
		Name:  "Old Name",
	}

	req := &user.UpdateProfileRequest{
		Email: "invalid-email",
	}

	mockRepo.On("GetByID", ctx, userID).Return(existingUser, nil)

	result, err := svc.UpdateProfile(ctx, userID, req)

	assert.Nil(t, result)
	assert.ErrorIs(t, err, ErrInvalidEmail)
	mockRepo.AssertExpectations(t)
}

func TestUpdateProfile_EmailAlreadyTaken(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)
	ctx := context.Background()
	userID := "user-123"

	existingUser := &user.User{
		ID:    userID,
		Email: "old@example.com",
		Name:  "Old Name",
	}

	anotherUser := &user.User{
		ID:    "user-456",
		Email: "taken@example.com",
		Name:  "Another User",
	}

	req := &user.UpdateProfileRequest{
		Email: "taken@example.com",
	}

	mockRepo.On("GetByID", ctx, userID).Return(existingUser, nil)
	mockRepo.On("GetByEmail", ctx, "taken@example.com").Return(anotherUser, nil)

	result, err := svc.UpdateProfile(ctx, userID, req)

	assert.Nil(t, result)
	assert.ErrorIs(t, err, ErrUserExists)
	mockRepo.AssertExpectations(t)
}

// ========== GetSettings Tests ==========

func TestGetSettings_Success(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)
	ctx := context.Background()
	userID := "user-123"

	expectedSettings := &user.UserSettings{
		UserID:         userID,
		Theme:          "dark",
		Timezone:       "Asia/Tokyo",
		IsConfigured:   true,
		AIEnabled:      true,
		AIConsentGiven: true,
	}

	mockRepo.On("GetSettings", ctx, userID).Return(expectedSettings, nil)

	result, err := svc.GetSettings(ctx, userID)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "dark", result.Theme)
	assert.Equal(t, "Asia/Tokyo", result.Timezone)
	assert.True(t, result.AIEnabled)
	assert.True(t, result.IsConfigured)
	mockRepo.AssertExpectations(t)
}

func TestGetSettings_NotFound(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)
	ctx := context.Background()
	userID := "nonexistent-user"

	mockRepo.On("GetSettings", ctx, userID).Return(nil, repository.ErrSettingsNotFound)

	result, err := svc.GetSettings(ctx, userID)

	assert.Nil(t, result)
	assert.ErrorIs(t, err, ErrUserNotFound)
	mockRepo.AssertExpectations(t)
}

// ========== UpdateSettings Tests ==========

func TestUpdateSettings_Success(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)
	ctx := context.Background()
	userID := "user-123"

	existingSettings := &user.UserSettings{
		UserID:   userID,
		Theme:    "light",
		Timezone: "UTC",
	}

	req := &user.UpdateSettingsRequest{
		Theme:    "dark",
		Timezone: "Asia/Tokyo",
	}

	// First call for UpdateSettings
	mockRepo.On("GetSettings", ctx, userID).Return(existingSettings, nil).Once()
	mockRepo.On("UpdateSettings", ctx, mock.AnythingOfType("*user.UserSettings")).Return(nil)
	// Second call for GetSettings (to return the response)
	updatedSettings := &user.UserSettings{
		UserID:   userID,
		Theme:    "dark",
		Timezone: "Asia/Tokyo",
	}
	mockRepo.On("GetSettings", ctx, userID).Return(updatedSettings, nil).Once()

	result, err := svc.UpdateSettings(ctx, userID, req)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "dark", result.Theme)
	assert.Equal(t, "Asia/Tokyo", result.Timezone)
	mockRepo.AssertExpectations(t)
}

func TestUpdateSettings_InvalidTheme(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)
	ctx := context.Background()
	userID := "user-123"

	existingSettings := &user.UserSettings{
		UserID: userID,
		Theme:  "light",
	}

	req := &user.UpdateSettingsRequest{
		Theme: "invalid-theme",
	}

	mockRepo.On("GetSettings", ctx, userID).Return(existingSettings, nil)

	result, err := svc.UpdateSettings(ctx, userID, req)

	assert.Nil(t, result)
	assert.ErrorIs(t, err, ErrInvalidTheme)
	mockRepo.AssertExpectations(t)
}

func TestUpdateSettings_NotFound(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)
	ctx := context.Background()
	userID := "nonexistent-user"

	req := &user.UpdateSettingsRequest{
		Theme: "dark",
	}

	mockRepo.On("GetSettings", ctx, userID).Return(nil, repository.ErrSettingsNotFound)

	result, err := svc.UpdateSettings(ctx, userID, req)

	assert.Nil(t, result)
	assert.ErrorIs(t, err, ErrUserNotFound)
	mockRepo.AssertExpectations(t)
}

func TestUpdateSettings_SetsIsConfigured(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)
	ctx := context.Background()
	userID := "user-123"

	existingSettings := &user.UserSettings{
		UserID:       userID,
		IsConfigured: false,
	}

	req := &user.UpdateSettingsRequest{
		Timezone: "Asia/Tokyo",
	}

	mockRepo.On("GetSettings", ctx, userID).Return(existingSettings, nil).Once()
	mockRepo.On("UpdateSettings", ctx, mock.MatchedBy(func(s *user.UserSettings) bool {
		return s.IsConfigured == true && s.Timezone == "Asia/Tokyo"
	})).Return(nil)

	updatedSettings := &user.UserSettings{
		UserID:       userID,
		Timezone:     "Asia/Tokyo",
		IsConfigured: true,
	}
	mockRepo.On("GetSettings", ctx, userID).Return(updatedSettings, nil).Once()

	result, err := svc.UpdateSettings(ctx, userID, req)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.IsConfigured)
	mockRepo.AssertExpectations(t)
}

// ========== GiveAIConsent Tests ==========

func TestGiveAIConsent_GrantConsent(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)
	ctx := context.Background()
	userID := "user-123"

	existingSettings := &user.UserSettings{
		UserID:         userID,
		AIConsentGiven: false,
		AIEnabled:      false,
	}

	mockRepo.On("GetSettings", ctx, userID).Return(existingSettings, nil).Once()
	mockRepo.On("UpdateSettings", ctx, mock.MatchedBy(func(s *user.UserSettings) bool {
		return s.AIConsentGiven == true
	})).Return(nil)

	updatedSettings := &user.UserSettings{
		UserID:         userID,
		AIConsentGiven: true,
		AIEnabled:      false, // AI enabled is not automatically set to true on consent grant
	}
	mockRepo.On("GetSettings", ctx, userID).Return(updatedSettings, nil).Once()

	result, err := svc.GiveAIConsent(ctx, userID, true)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.AIConsentGiven)
	mockRepo.AssertExpectations(t)
}

func TestGiveAIConsent_RevokeDisablesAI(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)
	ctx := context.Background()
	userID := "user-123"

	existingSettings := &user.UserSettings{
		UserID:         userID,
		AIConsentGiven: true,
		AIEnabled:      true,
	}

	mockRepo.On("GetSettings", ctx, userID).Return(existingSettings, nil).Once()
	mockRepo.On("UpdateSettings", ctx, mock.MatchedBy(func(s *user.UserSettings) bool {
		// When consent is revoked, AIEnabled should be set to false
		return s.AIConsentGiven == false && s.AIEnabled == false
	})).Return(nil)

	updatedSettings := &user.UserSettings{
		UserID:         userID,
		AIConsentGiven: false,
		AIEnabled:      false,
	}
	mockRepo.On("GetSettings", ctx, userID).Return(updatedSettings, nil).Once()

	result, err := svc.GiveAIConsent(ctx, userID, false)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.AIConsentGiven)
	assert.False(t, result.AIEnabled)
	mockRepo.AssertExpectations(t)
}

func TestGiveAIConsent_NotFound(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)
	ctx := context.Background()
	userID := "nonexistent-user"

	mockRepo.On("GetSettings", ctx, userID).Return(nil, repository.ErrSettingsNotFound)

	result, err := svc.GiveAIConsent(ctx, userID, true)

	assert.Nil(t, result)
	assert.ErrorIs(t, err, ErrUserNotFound)
	mockRepo.AssertExpectations(t)
}

// ========== Repository Error Propagation Tests ==========

func TestRegister_RepositoryError(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)
	ctx := context.Background()

	req := &user.RegisterRequest{
		Email:    "user@example.com",
		Password: "password123",
		Name:     "Test User",
	}

	dbErr := errors.New("database connection failed")
	mockRepo.On("Create", ctx, mock.AnythingOfType("*user.User")).Return(dbErr)

	resp, err := svc.Register(ctx, req)

	assert.Nil(t, resp)
	assert.ErrorIs(t, err, dbErr)
	mockRepo.AssertExpectations(t)
}

func TestLogin_RepositoryError(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)
	ctx := context.Background()

	req := &user.LoginRequest{
		Email:    "user@example.com",
		Password: "password123",
	}

	dbErr := errors.New("database connection failed")
	mockRepo.On("GetByEmail", ctx, "user@example.com").Return(nil, dbErr)

	resp, err := svc.Login(ctx, req)

	assert.Nil(t, resp)
	assert.ErrorIs(t, err, dbErr)
	mockRepo.AssertExpectations(t)
}
