package service

import (
	"context"
	"errors"
	"testing"
	"time"

	timeblock "github.com/kensan/backend/services/timeblock/internal"
	"github.com/kensan/backend/services/timeblock/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockRepository is a mock implementation of the timeblock repository
type MockRepository struct {
	mock.Mock
}

// Compile-time check: MockRepository must implement repository.Repository
var _ repository.Repository = (*MockRepository)(nil)

// TimeBlock methods
func (m *MockRepository) ListTimeBlocks(ctx context.Context, userID string, filter timeblock.TimeBlockFilter) ([]timeblock.TimeBlock, error) {
	args := m.Called(ctx, userID, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]timeblock.TimeBlock), args.Error(1)
}

func (m *MockRepository) GetTimeBlockByID(ctx context.Context, userID, timeBlockID string) (*timeblock.TimeBlock, error) {
	args := m.Called(ctx, userID, timeBlockID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*timeblock.TimeBlock), args.Error(1)
}

func (m *MockRepository) CreateTimeBlock(ctx context.Context, userID string, input timeblock.CreateTimeBlockInput) (*timeblock.TimeBlock, error) {
	args := m.Called(ctx, userID, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*timeblock.TimeBlock), args.Error(1)
}

func (m *MockRepository) UpdateTimeBlock(ctx context.Context, userID, timeBlockID string, input timeblock.UpdateTimeBlockInput) (*timeblock.TimeBlock, error) {
	args := m.Called(ctx, userID, timeBlockID, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*timeblock.TimeBlock), args.Error(1)
}

func (m *MockRepository) DeleteTimeBlock(ctx context.Context, userID, timeBlockID string) error {
	args := m.Called(ctx, userID, timeBlockID)
	return args.Error(0)
}

func (m *MockRepository) CreateTimeBlockBatch(ctx context.Context, userID string, inputs []timeblock.CreateTimeBlockInput) ([]timeblock.TimeBlock, error) {
	args := m.Called(ctx, userID, inputs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]timeblock.TimeBlock), args.Error(1)
}

// TimeEntry methods
func (m *MockRepository) ListTimeEntries(ctx context.Context, userID string, filter timeblock.TimeEntryFilter) ([]timeblock.TimeEntry, error) {
	args := m.Called(ctx, userID, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]timeblock.TimeEntry), args.Error(1)
}

func (m *MockRepository) GetTimeEntryByID(ctx context.Context, userID, timeEntryID string) (*timeblock.TimeEntry, error) {
	args := m.Called(ctx, userID, timeEntryID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*timeblock.TimeEntry), args.Error(1)
}

func (m *MockRepository) CreateTimeEntry(ctx context.Context, userID string, input timeblock.CreateTimeEntryInput) (*timeblock.TimeEntry, error) {
	args := m.Called(ctx, userID, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*timeblock.TimeEntry), args.Error(1)
}

func (m *MockRepository) UpdateTimeEntry(ctx context.Context, userID, timeEntryID string, input timeblock.UpdateTimeEntryInput) (*timeblock.TimeEntry, error) {
	args := m.Called(ctx, userID, timeEntryID, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*timeblock.TimeEntry), args.Error(1)
}

func (m *MockRepository) DeleteTimeEntry(ctx context.Context, userID, timeEntryID string) error {
	args := m.Called(ctx, userID, timeEntryID)
	return args.Error(0)
}

// Timer methods
func (m *MockRepository) GetRunningTimer(ctx context.Context, userID string) (*timeblock.RunningTimer, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*timeblock.RunningTimer), args.Error(1)
}

func (m *MockRepository) StartTimer(ctx context.Context, userID string, input timeblock.StartTimerInput) (*timeblock.RunningTimer, error) {
	args := m.Called(ctx, userID, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*timeblock.RunningTimer), args.Error(1)
}

func (m *MockRepository) StopTimer(ctx context.Context, userID string) (*timeblock.TimeEntry, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*timeblock.TimeEntry), args.Error(1)
}

// ========== Datetime Validation Tests ==========

func TestValidateDatetime(t *testing.T) {
	testCases := []struct {
		name      string
		datetime  string
		wantError bool
	}{
		{"valid UTC", "2024-01-15T09:00:00Z", false},
		{"valid with timezone offset", "2024-01-15T18:00:00+09:00", false},
		{"valid with milliseconds", "2024-01-15T09:00:00.000Z", false},
		{"invalid format - date only", "2024-01-15", true},
		{"invalid format - time only", "09:00", true},
		{"invalid format - no timezone", "2024-01-15T09:00:00", true},
		{"invalid format - slash date", "2024/01/15T09:00:00Z", true},
		{"invalid format - empty", "", true},
		{"invalid format - random string", "invalid", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := validateDatetime(tc.datetime)
			if tc.wantError {
				assert.Error(t, err)
				assert.ErrorIs(t, err, ErrInvalidDatetime)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// ========== TimeBlock List Tests ==========

func TestService_ListTimeBlocks_Success(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)
	ctx := context.Background()
	userID := "user-123"
	filter := timeblock.TimeBlockFilter{}

	expectedBlocks := []timeblock.TimeBlock{
		{ID: "tb1", TaskName: "Morning Study", StartDatetime: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC), EndDatetime: time.Date(2024, 1, 15, 1, 0, 0, 0, time.UTC)},
		{ID: "tb2", TaskName: "Afternoon Work", StartDatetime: time.Date(2024, 1, 15, 5, 0, 0, 0, time.UTC), EndDatetime: time.Date(2024, 1, 15, 7, 0, 0, 0, time.UTC)},
	}

	mockRepo.On("ListTimeBlocks", ctx, userID, filter).Return(expectedBlocks, nil)

	result, err := svc.ListTimeBlocks(ctx, userID, filter)

	assert.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "Morning Study", result[0].TaskName)
	mockRepo.AssertExpectations(t)
}

func TestService_ListTimeBlocks_WithDatetimeFilter(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)
	ctx := context.Background()
	userID := "user-123"
	startDt := "2024-01-14T15:00:00Z"
	endDt := "2024-01-15T15:00:00Z"
	filter := timeblock.TimeBlockFilter{
		StartDatetime: &startDt,
		EndDatetime:   &endDt,
	}

	expectedBlocks := []timeblock.TimeBlock{
		{ID: "tb1", TaskName: "Study", StartDatetime: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC), EndDatetime: time.Date(2024, 1, 15, 1, 0, 0, 0, time.UTC)},
	}

	mockRepo.On("ListTimeBlocks", ctx, userID, filter).Return(expectedBlocks, nil)

	result, err := svc.ListTimeBlocks(ctx, userID, filter)

	assert.NoError(t, err)
	assert.Len(t, result, 1)
	mockRepo.AssertExpectations(t)
}

func TestService_ListTimeBlocks_ReturnsEmptySliceForNil(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)
	ctx := context.Background()
	userID := "user-123"
	filter := timeblock.TimeBlockFilter{}

	mockRepo.On("ListTimeBlocks", ctx, userID, filter).Return(nil, nil)

	result, err := svc.ListTimeBlocks(ctx, userID, filter)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result, 0)
	mockRepo.AssertExpectations(t)
}

// ========== TimeBlock GetByID Tests ==========

func TestService_GetTimeBlock_Success(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)
	ctx := context.Background()
	userID := "user-123"
	timeBlockID := "tb-123"

	expectedBlock := &timeblock.TimeBlock{
		ID:            timeBlockID,
		UserID:        userID,
		TaskName:      "Test Block",
		StartDatetime: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		EndDatetime:   time.Date(2024, 1, 15, 1, 0, 0, 0, time.UTC),
	}

	mockRepo.On("GetTimeBlockByID", ctx, userID, timeBlockID).Return(expectedBlock, nil)

	result, err := svc.GetTimeBlock(ctx, userID, timeBlockID)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "Test Block", result.TaskName)
	mockRepo.AssertExpectations(t)
}

func TestService_GetTimeBlock_NotFound(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)
	ctx := context.Background()
	userID := "user-123"
	timeBlockID := "nonexistent"

	mockRepo.On("GetTimeBlockByID", ctx, userID, timeBlockID).Return(nil, nil)

	result, err := svc.GetTimeBlock(ctx, userID, timeBlockID)

	assert.ErrorIs(t, err, ErrTimeBlockNotFound)
	assert.Nil(t, result)
	mockRepo.AssertExpectations(t)
}

func TestService_GetTimeBlock_RepoError(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)
	ctx := context.Background()
	userID := "user-123"
	timeBlockID := "tb-123"
	repoErr := errors.New("database error")

	mockRepo.On("GetTimeBlockByID", ctx, userID, timeBlockID).Return(nil, repoErr)

	result, err := svc.GetTimeBlock(ctx, userID, timeBlockID)

	assert.ErrorIs(t, err, repoErr)
	assert.Nil(t, result)
	mockRepo.AssertExpectations(t)
}

// ========== TimeBlock Create Tests ==========

func TestService_CreateTimeBlock_Success(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)
	ctx := context.Background()
	userID := "user-123"

	input := timeblock.CreateTimeBlockInput{
		StartDatetime: "2024-01-15T09:00:00Z",
		EndDatetime:   "2024-01-15T10:00:00Z",
		TaskName:      "New Block",
	}

	expectedBlock := &timeblock.TimeBlock{
		ID:            "tb-new",
		UserID:        userID,
		TaskName:      "New Block",
		StartDatetime: time.Date(2024, 1, 15, 9, 0, 0, 0, time.UTC),
		EndDatetime:   time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
	}

	mockRepo.On("CreateTimeBlock", ctx, userID, input).Return(expectedBlock, nil)

	result, err := svc.CreateTimeBlock(ctx, userID, input)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "New Block", result.TaskName)
	mockRepo.AssertExpectations(t)
}

func TestService_CreateTimeBlock_EmptyTaskName(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)
	ctx := context.Background()
	userID := "user-123"

	input := timeblock.CreateTimeBlockInput{
		StartDatetime: "2024-01-15T09:00:00Z",
		EndDatetime:   "2024-01-15T10:00:00Z",
		TaskName:      "",
	}

	result, err := svc.CreateTimeBlock(ctx, userID, input)

	assert.ErrorIs(t, err, ErrInvalidInput)
	assert.Nil(t, result)
	mockRepo.AssertNotCalled(t, "CreateTimeBlock")
}

func TestService_CreateTimeBlock_InvalidStartDatetime(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)
	ctx := context.Background()
	userID := "user-123"

	input := timeblock.CreateTimeBlockInput{
		StartDatetime: "not-a-datetime",
		EndDatetime:   "2024-01-15T10:00:00Z",
		TaskName:      "Task",
	}

	result, err := svc.CreateTimeBlock(ctx, userID, input)

	assert.ErrorIs(t, err, ErrInvalidDatetime)
	assert.Nil(t, result)
	mockRepo.AssertNotCalled(t, "CreateTimeBlock")
}

func TestService_CreateTimeBlock_InvalidEndDatetime(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)
	ctx := context.Background()
	userID := "user-123"

	input := timeblock.CreateTimeBlockInput{
		StartDatetime: "2024-01-15T09:00:00Z",
		EndDatetime:   "not-a-datetime",
		TaskName:      "Task",
	}

	result, err := svc.CreateTimeBlock(ctx, userID, input)

	assert.ErrorIs(t, err, ErrInvalidDatetime)
	assert.Nil(t, result)
	mockRepo.AssertNotCalled(t, "CreateTimeBlock")
}

func TestService_CreateTimeBlock_EndBeforeStart(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)
	ctx := context.Background()
	userID := "user-123"

	input := timeblock.CreateTimeBlockInput{
		StartDatetime: "2024-01-15T10:00:00Z",
		EndDatetime:   "2024-01-15T09:00:00Z",
		TaskName:      "Task",
	}

	result, err := svc.CreateTimeBlock(ctx, userID, input)

	assert.ErrorIs(t, err, ErrInvalidDatetime)
	assert.Nil(t, result)
	mockRepo.AssertNotCalled(t, "CreateTimeBlock")
}

func TestService_CreateTimeBlock_EndEqualsStart(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)
	ctx := context.Background()
	userID := "user-123"

	input := timeblock.CreateTimeBlockInput{
		StartDatetime: "2024-01-15T09:00:00Z",
		EndDatetime:   "2024-01-15T09:00:00Z",
		TaskName:      "Task",
	}

	result, err := svc.CreateTimeBlock(ctx, userID, input)

	assert.ErrorIs(t, err, ErrInvalidDatetime)
	assert.Nil(t, result)
	mockRepo.AssertNotCalled(t, "CreateTimeBlock")
}

// ========== TimeBlock Update Tests ==========

func TestService_UpdateTimeBlock_Success(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)
	ctx := context.Background()
	userID := "user-123"
	timeBlockID := "tb-123"
	newTaskName := "Updated Block"

	existingBlock := &timeblock.TimeBlock{
		ID:            timeBlockID,
		UserID:        userID,
		TaskName:      "Old Block",
		StartDatetime: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		EndDatetime:   time.Date(2024, 1, 15, 1, 0, 0, 0, time.UTC),
	}

	input := timeblock.UpdateTimeBlockInput{
		TaskName: &newTaskName,
	}

	updatedBlock := &timeblock.TimeBlock{
		ID:            timeBlockID,
		UserID:        userID,
		TaskName:      newTaskName,
		StartDatetime: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		EndDatetime:   time.Date(2024, 1, 15, 1, 0, 0, 0, time.UTC),
	}

	mockRepo.On("GetTimeBlockByID", ctx, userID, timeBlockID).Return(existingBlock, nil)
	mockRepo.On("UpdateTimeBlock", ctx, userID, timeBlockID, input).Return(updatedBlock, nil)

	result, err := svc.UpdateTimeBlock(ctx, userID, timeBlockID, input)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, newTaskName, result.TaskName)
	mockRepo.AssertExpectations(t)
}

func TestService_UpdateTimeBlock_NotFound(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)
	ctx := context.Background()
	userID := "user-123"
	timeBlockID := "nonexistent"
	newTaskName := "Updated Block"

	input := timeblock.UpdateTimeBlockInput{
		TaskName: &newTaskName,
	}

	mockRepo.On("GetTimeBlockByID", ctx, userID, timeBlockID).Return(nil, nil)

	result, err := svc.UpdateTimeBlock(ctx, userID, timeBlockID, input)

	assert.ErrorIs(t, err, ErrTimeBlockNotFound)
	assert.Nil(t, result)
	mockRepo.AssertNotCalled(t, "UpdateTimeBlock")
}

func TestService_UpdateTimeBlock_InvalidDatetime(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)
	ctx := context.Background()
	userID := "user-123"
	timeBlockID := "tb-123"
	invalidDt := "not-a-datetime"

	existingBlock := &timeblock.TimeBlock{
		ID:            timeBlockID,
		UserID:        userID,
		TaskName:      "Block",
		StartDatetime: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		EndDatetime:   time.Date(2024, 1, 15, 1, 0, 0, 0, time.UTC),
	}

	input := timeblock.UpdateTimeBlockInput{
		StartDatetime: &invalidDt,
	}

	mockRepo.On("GetTimeBlockByID", ctx, userID, timeBlockID).Return(existingBlock, nil)

	result, err := svc.UpdateTimeBlock(ctx, userID, timeBlockID, input)

	assert.ErrorIs(t, err, ErrInvalidDatetime)
	assert.Nil(t, result)
	mockRepo.AssertNotCalled(t, "UpdateTimeBlock")
}

// ========== TimeBlock Delete Tests ==========

func TestService_DeleteTimeBlock_Success(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)
	ctx := context.Background()
	userID := "user-123"
	timeBlockID := "tb-123"

	existingBlock := &timeblock.TimeBlock{
		ID:            timeBlockID,
		UserID:        userID,
		TaskName:      "Block to delete",
		StartDatetime: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		EndDatetime:   time.Date(2024, 1, 15, 1, 0, 0, 0, time.UTC),
	}

	mockRepo.On("GetTimeBlockByID", ctx, userID, timeBlockID).Return(existingBlock, nil)
	mockRepo.On("DeleteTimeBlock", ctx, userID, timeBlockID).Return(nil)

	err := svc.DeleteTimeBlock(ctx, userID, timeBlockID)

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestService_DeleteTimeBlock_NotFound(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)
	ctx := context.Background()
	userID := "user-123"
	timeBlockID := "nonexistent"

	mockRepo.On("GetTimeBlockByID", ctx, userID, timeBlockID).Return(nil, nil)

	err := svc.DeleteTimeBlock(ctx, userID, timeBlockID)

	assert.ErrorIs(t, err, ErrTimeBlockNotFound)
	mockRepo.AssertNotCalled(t, "DeleteTimeBlock")
}

// ========== TimeEntry List Tests ==========

func TestService_ListTimeEntries_Success(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)
	ctx := context.Background()
	userID := "user-123"
	filter := timeblock.TimeEntryFilter{}

	expectedEntries := []timeblock.TimeEntry{
		{ID: "te1", TaskName: "Work Task", StartDatetime: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC), EndDatetime: time.Date(2024, 1, 15, 1, 30, 0, 0, time.UTC)},
		{ID: "te2", TaskName: "Another Task", StartDatetime: time.Date(2024, 1, 15, 2, 0, 0, 0, time.UTC), EndDatetime: time.Date(2024, 1, 15, 3, 0, 0, 0, time.UTC)},
	}

	mockRepo.On("ListTimeEntries", ctx, userID, filter).Return(expectedEntries, nil)

	result, err := svc.ListTimeEntries(ctx, userID, filter)

	assert.NoError(t, err)
	assert.Len(t, result, 2)
	mockRepo.AssertExpectations(t)
}

func TestService_ListTimeEntries_ReturnsEmptySliceForNil(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)
	ctx := context.Background()
	userID := "user-123"
	filter := timeblock.TimeEntryFilter{}

	mockRepo.On("ListTimeEntries", ctx, userID, filter).Return(nil, nil)

	result, err := svc.ListTimeEntries(ctx, userID, filter)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result, 0)
	mockRepo.AssertExpectations(t)
}

// ========== TimeEntry GetByID Tests ==========

func TestService_GetTimeEntry_Success(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)
	ctx := context.Background()
	userID := "user-123"
	timeEntryID := "te-123"

	expectedEntry := &timeblock.TimeEntry{
		ID:            timeEntryID,
		UserID:        userID,
		TaskName:      "Test Entry",
		StartDatetime: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		EndDatetime:   time.Date(2024, 1, 15, 1, 0, 0, 0, time.UTC),
	}

	mockRepo.On("GetTimeEntryByID", ctx, userID, timeEntryID).Return(expectedEntry, nil)

	result, err := svc.GetTimeEntry(ctx, userID, timeEntryID)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "Test Entry", result.TaskName)
	mockRepo.AssertExpectations(t)
}

func TestService_GetTimeEntry_NotFound(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)
	ctx := context.Background()
	userID := "user-123"
	timeEntryID := "nonexistent"

	mockRepo.On("GetTimeEntryByID", ctx, userID, timeEntryID).Return(nil, nil)

	result, err := svc.GetTimeEntry(ctx, userID, timeEntryID)

	assert.ErrorIs(t, err, ErrTimeEntryNotFound)
	assert.Nil(t, result)
	mockRepo.AssertExpectations(t)
}

// ========== TimeEntry Create Tests ==========

func TestService_CreateTimeEntry_Success(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)
	ctx := context.Background()
	userID := "user-123"

	input := timeblock.CreateTimeEntryInput{
		StartDatetime: "2024-01-15T09:00:00Z",
		EndDatetime:   "2024-01-15T10:30:00Z",
		TaskName:      "New Entry",
		Description:   strPtr("Working on feature"),
	}

	expectedEntry := &timeblock.TimeEntry{
		ID:            "te-new",
		UserID:        userID,
		TaskName:      "New Entry",
		StartDatetime: time.Date(2024, 1, 15, 9, 0, 0, 0, time.UTC),
		EndDatetime:   time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		Description:   strPtr("Working on feature"),
	}

	mockRepo.On("CreateTimeEntry", ctx, userID, input).Return(expectedEntry, nil)

	result, err := svc.CreateTimeEntry(ctx, userID, input)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "New Entry", result.TaskName)
	mockRepo.AssertExpectations(t)
}

func TestService_CreateTimeEntry_EmptyTaskName(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)
	ctx := context.Background()
	userID := "user-123"

	input := timeblock.CreateTimeEntryInput{
		StartDatetime: "2024-01-15T09:00:00Z",
		EndDatetime:   "2024-01-15T10:30:00Z",
		TaskName:      "",
	}

	result, err := svc.CreateTimeEntry(ctx, userID, input)

	assert.ErrorIs(t, err, ErrInvalidInput)
	assert.Nil(t, result)
	mockRepo.AssertNotCalled(t, "CreateTimeEntry")
}

func TestService_CreateTimeEntry_InvalidDatetime(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)
	ctx := context.Background()
	userID := "user-123"

	input := timeblock.CreateTimeEntryInput{
		StartDatetime: "not-a-datetime",
		EndDatetime:   "2024-01-15T10:30:00Z",
		TaskName:      "Task",
	}

	result, err := svc.CreateTimeEntry(ctx, userID, input)

	assert.ErrorIs(t, err, ErrInvalidDatetime)
	assert.Nil(t, result)
	mockRepo.AssertNotCalled(t, "CreateTimeEntry")
}

func TestService_CreateTimeEntry_EndBeforeStart(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)
	ctx := context.Background()
	userID := "user-123"

	input := timeblock.CreateTimeEntryInput{
		StartDatetime: "2024-01-15T10:30:00Z",
		EndDatetime:   "2024-01-15T09:00:00Z",
		TaskName:      "Task",
	}

	result, err := svc.CreateTimeEntry(ctx, userID, input)

	assert.ErrorIs(t, err, ErrInvalidDatetime)
	assert.Nil(t, result)
	mockRepo.AssertNotCalled(t, "CreateTimeEntry")
}

// ========== TimeEntry Update Tests ==========

func TestService_UpdateTimeEntry_Success(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)
	ctx := context.Background()
	userID := "user-123"
	timeEntryID := "te-123"
	newTaskName := "Updated Entry"

	existingEntry := &timeblock.TimeEntry{
		ID:            timeEntryID,
		UserID:        userID,
		TaskName:      "Old Entry",
		StartDatetime: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		EndDatetime:   time.Date(2024, 1, 15, 1, 0, 0, 0, time.UTC),
	}

	input := timeblock.UpdateTimeEntryInput{
		TaskName: &newTaskName,
	}

	updatedEntry := &timeblock.TimeEntry{
		ID:            timeEntryID,
		UserID:        userID,
		TaskName:      newTaskName,
		StartDatetime: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		EndDatetime:   time.Date(2024, 1, 15, 1, 0, 0, 0, time.UTC),
	}

	mockRepo.On("GetTimeEntryByID", ctx, userID, timeEntryID).Return(existingEntry, nil)
	mockRepo.On("UpdateTimeEntry", ctx, userID, timeEntryID, input).Return(updatedEntry, nil)

	result, err := svc.UpdateTimeEntry(ctx, userID, timeEntryID, input)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, newTaskName, result.TaskName)
	mockRepo.AssertExpectations(t)
}

func TestService_UpdateTimeEntry_NotFound(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)
	ctx := context.Background()
	userID := "user-123"
	timeEntryID := "nonexistent"
	newTaskName := "Updated Entry"

	input := timeblock.UpdateTimeEntryInput{
		TaskName: &newTaskName,
	}

	mockRepo.On("GetTimeEntryByID", ctx, userID, timeEntryID).Return(nil, nil)

	result, err := svc.UpdateTimeEntry(ctx, userID, timeEntryID, input)

	assert.ErrorIs(t, err, ErrTimeEntryNotFound)
	assert.Nil(t, result)
	mockRepo.AssertNotCalled(t, "UpdateTimeEntry")
}

func TestService_UpdateTimeEntry_InvalidDatetime(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)
	ctx := context.Background()
	userID := "user-123"
	timeEntryID := "te-123"
	invalidDt := "not-a-datetime"

	existingEntry := &timeblock.TimeEntry{
		ID:            timeEntryID,
		UserID:        userID,
		TaskName:      "Entry",
		StartDatetime: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		EndDatetime:   time.Date(2024, 1, 15, 1, 0, 0, 0, time.UTC),
	}

	input := timeblock.UpdateTimeEntryInput{
		StartDatetime: &invalidDt,
	}

	mockRepo.On("GetTimeEntryByID", ctx, userID, timeEntryID).Return(existingEntry, nil)

	result, err := svc.UpdateTimeEntry(ctx, userID, timeEntryID, input)

	assert.ErrorIs(t, err, ErrInvalidDatetime)
	assert.Nil(t, result)
	mockRepo.AssertNotCalled(t, "UpdateTimeEntry")
}

// ========== TimeEntry Delete Tests ==========

func TestService_DeleteTimeEntry_Success(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)
	ctx := context.Background()
	userID := "user-123"
	timeEntryID := "te-123"

	existingEntry := &timeblock.TimeEntry{
		ID:            timeEntryID,
		UserID:        userID,
		TaskName:      "Entry to delete",
		StartDatetime: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		EndDatetime:   time.Date(2024, 1, 15, 1, 0, 0, 0, time.UTC),
	}

	mockRepo.On("GetTimeEntryByID", ctx, userID, timeEntryID).Return(existingEntry, nil)
	mockRepo.On("DeleteTimeEntry", ctx, userID, timeEntryID).Return(nil)

	err := svc.DeleteTimeEntry(ctx, userID, timeEntryID)

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestService_DeleteTimeEntry_NotFound(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)
	ctx := context.Background()
	userID := "user-123"
	timeEntryID := "nonexistent"

	mockRepo.On("GetTimeEntryByID", ctx, userID, timeEntryID).Return(nil, nil)

	err := svc.DeleteTimeEntry(ctx, userID, timeEntryID)

	assert.ErrorIs(t, err, ErrTimeEntryNotFound)
	mockRepo.AssertNotCalled(t, "DeleteTimeEntry")
}

// Helper function
func strPtr(s string) *string {
	return &s
}
