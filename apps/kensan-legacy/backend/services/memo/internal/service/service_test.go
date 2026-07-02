package service

import (
	"context"
	"errors"
	"testing"
	"time"

	memo "github.com/kensan/backend/services/memo/internal"
	"github.com/kensan/backend/services/memo/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Compile-time check that MockRepository implements repository.Repository
var _ repository.Repository = (*MockRepository)(nil)

// MockRepository is a mock implementation of the repository.Repository interface
type MockRepository struct {
	mock.Mock
}

func (m *MockRepository) List(ctx context.Context, userID string, filter memo.MemoFilter) ([]memo.Memo, error) {
	args := m.Called(ctx, userID, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]memo.Memo), args.Error(1)
}

func (m *MockRepository) GetByID(ctx context.Context, userID, memoID string) (*memo.Memo, error) {
	args := m.Called(ctx, userID, memoID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*memo.Memo), args.Error(1)
}

func (m *MockRepository) Create(ctx context.Context, userID string, input memo.CreateMemoInput) (*memo.Memo, error) {
	args := m.Called(ctx, userID, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*memo.Memo), args.Error(1)
}

func (m *MockRepository) Update(ctx context.Context, userID, memoID string, input memo.UpdateMemoInput) (*memo.Memo, error) {
	args := m.Called(ctx, userID, memoID, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*memo.Memo), args.Error(1)
}

func (m *MockRepository) Delete(ctx context.Context, userID, memoID string) error {
	args := m.Called(ctx, userID, memoID)
	return args.Error(0)
}

// ========== List Tests ==========

func TestList_Success(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	filter := memo.MemoFilter{}

	now := time.Now()
	expectedMemos := []memo.Memo{
		{ID: "memo-1", UserID: userID, Content: "First memo", Archived: false, CreatedAt: now, UpdatedAt: now},
		{ID: "memo-2", UserID: userID, Content: "Second memo", Archived: false, CreatedAt: now, UpdatedAt: now},
	}

	mockRepo.On("List", ctx, userID, filter).Return(expectedMemos, nil)

	memos, err := svc.List(ctx, userID, filter)

	assert.NoError(t, err)
	assert.Len(t, memos, 2)
	assert.Equal(t, "First memo", memos[0].Content)
	assert.Equal(t, "Second memo", memos[1].Content)
	mockRepo.AssertExpectations(t)
}

func TestList_Empty(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	filter := memo.MemoFilter{}

	// When repository returns nil, service should return empty slice
	mockRepo.On("List", ctx, userID, filter).Return(nil, nil)

	memos, err := svc.List(ctx, userID, filter)

	assert.NoError(t, err)
	assert.NotNil(t, memos)
	assert.Len(t, memos, 0)
	mockRepo.AssertExpectations(t)
}

func TestList_WithArchivedFilter(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	archived := true
	filter := memo.MemoFilter{Archived: &archived}

	now := time.Now()
	expectedMemos := []memo.Memo{
		{ID: "memo-1", UserID: userID, Content: "Archived memo", Archived: true, CreatedAt: now, UpdatedAt: now},
	}

	mockRepo.On("List", ctx, userID, filter).Return(expectedMemos, nil)

	memos, err := svc.List(ctx, userID, filter)

	assert.NoError(t, err)
	assert.Len(t, memos, 1)
	assert.True(t, memos[0].Archived)
	mockRepo.AssertExpectations(t)
}

func TestList_WithDateFilter(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	date := "2026-01-20"
	filter := memo.MemoFilter{Date: &date}

	now := time.Now()
	expectedMemos := []memo.Memo{
		{ID: "memo-1", UserID: userID, Content: "Today's memo", CreatedAt: now, UpdatedAt: now},
	}

	mockRepo.On("List", ctx, userID, filter).Return(expectedMemos, nil)

	memos, err := svc.List(ctx, userID, filter)

	assert.NoError(t, err)
	assert.Len(t, memos, 1)
	mockRepo.AssertExpectations(t)
}

func TestList_RepositoryError(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	filter := memo.MemoFilter{}

	mockRepo.On("List", ctx, userID, filter).Return(nil, errors.New("database error"))

	memos, err := svc.List(ctx, userID, filter)

	assert.Error(t, err)
	assert.Nil(t, memos)
	mockRepo.AssertExpectations(t)
}

// ========== GetByID Tests ==========

func TestGetByID_Success(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	memoID := "memo-123"

	now := time.Now()
	expectedMemo := &memo.Memo{
		ID:        memoID,
		UserID:    userID,
		Content:   "Test memo content",
		Archived:  false,
		CreatedAt: now,
		UpdatedAt: now,
	}

	mockRepo.On("GetByID", ctx, userID, memoID).Return(expectedMemo, nil)

	result, err := svc.GetByID(ctx, userID, memoID)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, memoID, result.ID)
	assert.Equal(t, userID, result.UserID)
	assert.Equal(t, "Test memo content", result.Content)
	assert.False(t, result.Archived)
	mockRepo.AssertExpectations(t)
}

func TestGetByID_NotFound(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	memoID := "nonexistent"

	mockRepo.On("GetByID", ctx, userID, memoID).Return(nil, nil)

	result, err := svc.GetByID(ctx, userID, memoID)

	assert.Error(t, err)
	assert.Equal(t, ErrMemoNotFound, err)
	assert.Nil(t, result)
	mockRepo.AssertExpectations(t)
}

func TestGetByID_RepositoryError(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	memoID := "memo-123"

	mockRepo.On("GetByID", ctx, userID, memoID).Return(nil, errors.New("database error"))

	result, err := svc.GetByID(ctx, userID, memoID)

	assert.Error(t, err)
	assert.Nil(t, result)
	mockRepo.AssertExpectations(t)
}

// ========== Create Tests ==========

func TestCreate_Success(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	input := memo.CreateMemoInput{
		Content: "New memo content",
	}

	now := time.Now()
	expectedMemo := &memo.Memo{
		ID:        "memo-new",
		UserID:    userID,
		Content:   "New memo content",
		Archived:  false,
		CreatedAt: now,
		UpdatedAt: now,
	}

	mockRepo.On("Create", ctx, userID, input).Return(expectedMemo, nil)

	result, err := svc.Create(ctx, userID, input)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "New memo content", result.Content)
	assert.Equal(t, userID, result.UserID)
	assert.False(t, result.Archived)
	mockRepo.AssertExpectations(t)
}

func TestCreate_EmptyContent(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	input := memo.CreateMemoInput{
		Content: "",
	}

	result, err := svc.Create(ctx, userID, input)

	assert.Error(t, err)
	assert.Equal(t, ErrInvalidInput, err)
	assert.Nil(t, result)
	mockRepo.AssertExpectations(t)
}

func TestCreate_RepositoryError(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	input := memo.CreateMemoInput{
		Content: "New memo content",
	}

	mockRepo.On("Create", ctx, userID, input).Return(nil, errors.New("database error"))

	result, err := svc.Create(ctx, userID, input)

	assert.Error(t, err)
	assert.Nil(t, result)
	mockRepo.AssertExpectations(t)
}

// ========== Update Tests ==========

func TestUpdate_Success(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	memoID := "memo-123"
	newContent := "Updated content"
	input := memo.UpdateMemoInput{
		Content: &newContent,
	}

	now := time.Now()
	existingMemo := &memo.Memo{ID: memoID, UserID: userID, Content: "Old content", CreatedAt: now, UpdatedAt: now}
	updatedMemo := &memo.Memo{
		ID:        memoID,
		UserID:    userID,
		Content:   "Updated content",
		Archived:  false,
		CreatedAt: now,
		UpdatedAt: now,
	}

	mockRepo.On("GetByID", ctx, userID, memoID).Return(existingMemo, nil)
	mockRepo.On("Update", ctx, userID, memoID, input).Return(updatedMemo, nil)

	result, err := svc.Update(ctx, userID, memoID, input)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "Updated content", result.Content)
	mockRepo.AssertExpectations(t)
}

func TestUpdate_NotFound(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	memoID := "nonexistent"
	newContent := "Updated content"
	input := memo.UpdateMemoInput{
		Content: &newContent,
	}

	mockRepo.On("GetByID", ctx, userID, memoID).Return(nil, nil)

	result, err := svc.Update(ctx, userID, memoID, input)

	assert.Error(t, err)
	assert.Equal(t, ErrMemoNotFound, err)
	assert.Nil(t, result)
	mockRepo.AssertExpectations(t)
}

func TestUpdate_ArchiveField(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	memoID := "memo-123"
	archived := true
	input := memo.UpdateMemoInput{
		Archived: &archived,
	}

	now := time.Now()
	existingMemo := &memo.Memo{ID: memoID, UserID: userID, Content: "Memo content", Archived: false, CreatedAt: now, UpdatedAt: now}
	updatedMemo := &memo.Memo{
		ID:        memoID,
		UserID:    userID,
		Content:   "Memo content",
		Archived:  true,
		CreatedAt: now,
		UpdatedAt: now,
	}

	mockRepo.On("GetByID", ctx, userID, memoID).Return(existingMemo, nil)
	mockRepo.On("Update", ctx, userID, memoID, input).Return(updatedMemo, nil)

	result, err := svc.Update(ctx, userID, memoID, input)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Archived)
	mockRepo.AssertExpectations(t)
}

func TestUpdate_GetByIDRepositoryError(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	memoID := "memo-123"
	newContent := "Updated content"
	input := memo.UpdateMemoInput{
		Content: &newContent,
	}

	mockRepo.On("GetByID", ctx, userID, memoID).Return(nil, errors.New("database error"))

	result, err := svc.Update(ctx, userID, memoID, input)

	assert.Error(t, err)
	assert.Nil(t, result)
	mockRepo.AssertExpectations(t)
}

func TestUpdate_UpdateRepositoryReturnsNil(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	memoID := "memo-123"
	newContent := "Updated content"
	input := memo.UpdateMemoInput{
		Content: &newContent,
	}

	now := time.Now()
	existingMemo := &memo.Memo{ID: memoID, UserID: userID, Content: "Old content", CreatedAt: now, UpdatedAt: now}

	mockRepo.On("GetByID", ctx, userID, memoID).Return(existingMemo, nil)
	mockRepo.On("Update", ctx, userID, memoID, input).Return(nil, nil)

	result, err := svc.Update(ctx, userID, memoID, input)

	assert.Error(t, err)
	assert.Equal(t, ErrMemoNotFound, err)
	assert.Nil(t, result)
	mockRepo.AssertExpectations(t)
}

func TestUpdate_UpdateRepositoryError(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	memoID := "memo-123"
	newContent := "Updated content"
	input := memo.UpdateMemoInput{
		Content: &newContent,
	}

	now := time.Now()
	existingMemo := &memo.Memo{ID: memoID, UserID: userID, Content: "Old content", CreatedAt: now, UpdatedAt: now}

	mockRepo.On("GetByID", ctx, userID, memoID).Return(existingMemo, nil)
	mockRepo.On("Update", ctx, userID, memoID, input).Return(nil, errors.New("database error"))

	result, err := svc.Update(ctx, userID, memoID, input)

	assert.Error(t, err)
	assert.Nil(t, result)
	mockRepo.AssertExpectations(t)
}

// ========== Archive Tests ==========

func TestArchive_Success(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	memoID := "memo-123"

	now := time.Now()
	existingMemo := &memo.Memo{ID: memoID, UserID: userID, Content: "Memo content", Archived: false, CreatedAt: now, UpdatedAt: now}

	archived := true
	expectedInput := memo.UpdateMemoInput{Archived: &archived}

	updatedMemo := &memo.Memo{
		ID:        memoID,
		UserID:    userID,
		Content:   "Memo content",
		Archived:  true,
		CreatedAt: now,
		UpdatedAt: now,
	}

	mockRepo.On("GetByID", ctx, userID, memoID).Return(existingMemo, nil)
	mockRepo.On("Update", ctx, userID, memoID, expectedInput).Return(updatedMemo, nil)

	result, err := svc.Archive(ctx, userID, memoID)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Archived)
	assert.Equal(t, "Memo content", result.Content)
	mockRepo.AssertExpectations(t)
}

func TestArchive_NotFound(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	memoID := "nonexistent"

	mockRepo.On("GetByID", ctx, userID, memoID).Return(nil, nil)

	result, err := svc.Archive(ctx, userID, memoID)

	assert.Error(t, err)
	assert.Equal(t, ErrMemoNotFound, err)
	assert.Nil(t, result)
	mockRepo.AssertExpectations(t)
}

// ========== Delete Tests ==========

func TestDelete_Success(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	memoID := "memo-123"

	now := time.Now()
	existingMemo := &memo.Memo{ID: memoID, UserID: userID, Content: "Memo to delete", CreatedAt: now, UpdatedAt: now}
	mockRepo.On("GetByID", ctx, userID, memoID).Return(existingMemo, nil)
	mockRepo.On("Delete", ctx, userID, memoID).Return(nil)

	err := svc.Delete(ctx, userID, memoID)

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestDelete_NotFound(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	memoID := "nonexistent"

	mockRepo.On("GetByID", ctx, userID, memoID).Return(nil, nil)

	err := svc.Delete(ctx, userID, memoID)

	assert.Error(t, err)
	assert.Equal(t, ErrMemoNotFound, err)
	mockRepo.AssertExpectations(t)
}

func TestDelete_GetByIDRepositoryError(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	memoID := "memo-123"

	mockRepo.On("GetByID", ctx, userID, memoID).Return(nil, errors.New("database error"))

	err := svc.Delete(ctx, userID, memoID)

	assert.Error(t, err)
	mockRepo.AssertExpectations(t)
}

func TestDelete_DeleteRepositoryError(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	memoID := "memo-123"

	now := time.Now()
	existingMemo := &memo.Memo{ID: memoID, UserID: userID, Content: "Memo to delete", CreatedAt: now, UpdatedAt: now}
	mockRepo.On("GetByID", ctx, userID, memoID).Return(existingMemo, nil)
	mockRepo.On("Delete", ctx, userID, memoID).Return(errors.New("database error"))

	err := svc.Delete(ctx, userID, memoID)

	assert.Error(t, err)
	mockRepo.AssertExpectations(t)
}

// ========== Multi-tenancy Tests ==========

func TestGetByID_DifferentUser(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)

	ctx := context.Background()
	userID := "user-456"
	memoID := "memo-123"

	// A different user should not find a memo that belongs to another user
	mockRepo.On("GetByID", ctx, userID, memoID).Return(nil, nil)

	result, err := svc.GetByID(ctx, userID, memoID)

	assert.Error(t, err)
	assert.Equal(t, ErrMemoNotFound, err)
	assert.Nil(t, result)
	mockRepo.AssertExpectations(t)
}

func TestList_DifferentUser(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)

	ctx := context.Background()
	userID := "user-456"
	filter := memo.MemoFilter{}

	// Different user should get their own (possibly empty) list
	mockRepo.On("List", ctx, userID, filter).Return(nil, nil)

	memos, err := svc.List(ctx, userID, filter)

	assert.NoError(t, err)
	assert.NotNil(t, memos)
	assert.Len(t, memos, 0)
	mockRepo.AssertExpectations(t)
}

// ========== Table-Driven Tests ==========

func TestCreate_TableDriven(t *testing.T) {
	tests := []struct {
		name    string
		input   memo.CreateMemoInput
		wantErr bool
		errVal  error
	}{
		{
			name:    "valid content",
			input:   memo.CreateMemoInput{Content: "A valid memo"},
			wantErr: false,
		},
		{
			name:    "empty content",
			input:   memo.CreateMemoInput{Content: ""},
			wantErr: true,
			errVal:  ErrInvalidInput,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := new(MockRepository)
			svc := NewService(mockRepo)

			ctx := context.Background()
			userID := "user-123"

			if !tt.wantErr {
				now := time.Now()
				expectedMemo := &memo.Memo{
					ID:        "memo-new",
					UserID:    userID,
					Content:   tt.input.Content,
					Archived:  false,
					CreatedAt: now,
					UpdatedAt: now,
				}
				mockRepo.On("Create", ctx, userID, tt.input).Return(expectedMemo, nil)
			}

			result, err := svc.Create(ctx, userID, tt.input)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errVal != nil {
					assert.Equal(t, tt.errVal, err)
				}
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.input.Content, result.Content)
			}
			mockRepo.AssertExpectations(t)
		})
	}
}

func TestGetByID_TableDriven(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name       string
		memoID     string
		repoReturn *memo.Memo
		repoErr    error
		wantErr    bool
		errVal     error
	}{
		{
			name:       "found",
			memoID:     "memo-123",
			repoReturn: &memo.Memo{ID: "memo-123", UserID: "user-123", Content: "Test", CreatedAt: now, UpdatedAt: now},
			repoErr:    nil,
			wantErr:    false,
		},
		{
			name:       "not found - nil return",
			memoID:     "nonexistent",
			repoReturn: nil,
			repoErr:    nil,
			wantErr:    true,
			errVal:     ErrMemoNotFound,
		},
		{
			name:       "repository error",
			memoID:     "memo-123",
			repoReturn: nil,
			repoErr:    errors.New("database error"),
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := new(MockRepository)
			svc := NewService(mockRepo)

			ctx := context.Background()
			userID := "user-123"

			if tt.repoReturn != nil {
				mockRepo.On("GetByID", ctx, userID, tt.memoID).Return(tt.repoReturn, tt.repoErr)
			} else {
				mockRepo.On("GetByID", ctx, userID, tt.memoID).Return(nil, tt.repoErr)
			}

			result, err := svc.GetByID(ctx, userID, tt.memoID)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errVal != nil {
					assert.Equal(t, tt.errVal, err)
				}
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.memoID, result.ID)
			}
			mockRepo.AssertExpectations(t)
		})
	}
}
