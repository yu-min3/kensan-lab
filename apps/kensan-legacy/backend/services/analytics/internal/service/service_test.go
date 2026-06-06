package service

import (
	"context"
	"errors"
	"testing"

	analytics "github.com/kensan/backend/services/analytics/internal"
	"github.com/kensan/backend/services/analytics/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockRepository is a mock implementation of the analytics repository
type MockRepository struct {
	mock.Mock
}

// Compile-time check that MockRepository implements repository.Repository
var _ repository.Repository = (*MockRepository)(nil)

func (m *MockRepository) GetTotalMinutesByDateRange(ctx context.Context, userID, startDatetime, endDatetime string) (int, error) {
	args := m.Called(ctx, userID, startDatetime, endDatetime)
	return args.Int(0), args.Error(1)
}

func (m *MockRepository) GetCompletedTasksCount(ctx context.Context, userID, startDate, endDate string) (int, error) {
	args := m.Called(ctx, userID, startDate, endDate)
	return args.Int(0), args.Error(1)
}

func (m *MockRepository) GetTimeBlocksAggregated(ctx context.Context, userID, startDatetime, endDatetime string) (int, error) {
	args := m.Called(ctx, userID, startDatetime, endDatetime)
	return args.Int(0), args.Error(1)
}

func (m *MockRepository) GetDailyBreakdown(ctx context.Context, userID, startDatetime, endDatetime, timezone string) ([]analytics.DailyBreakdown, error) {
	args := m.Called(ctx, userID, startDatetime, endDatetime, timezone)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]analytics.DailyBreakdown), args.Error(1)
}

func (m *MockRepository) GetWeeklyBreakdown(ctx context.Context, userID, startDatetime, endDatetime, timezone string) ([]analytics.DailyBreakdown, error) {
	args := m.Called(ctx, userID, startDatetime, endDatetime, timezone)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]analytics.DailyBreakdown), args.Error(1)
}

func (m *MockRepository) GetMinutesByGoal(ctx context.Context, userID, startDatetime, endDatetime string) ([]repository.GoalWithMinutes, error) {
	args := m.Called(ctx, userID, startDatetime, endDatetime)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]repository.GoalWithMinutes), args.Error(1)
}

func (m *MockRepository) GetMinutesByMilestone(ctx context.Context, userID, startDatetime, endDatetime string) ([]repository.MilestoneWithMinutes, error) {
	args := m.Called(ctx, userID, startDatetime, endDatetime)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]repository.MilestoneWithMinutes), args.Error(1)
}

func (m *MockRepository) GetMinutesByTag(ctx context.Context, userID, startDatetime, endDatetime string) ([]repository.TagWithMinutes, error) {
	args := m.Called(ctx, userID, startDatetime, endDatetime)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]repository.TagWithMinutes), args.Error(1)
}

func (m *MockRepository) GetGoals(ctx context.Context, userID string) ([]analytics.GoalSummary, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]analytics.GoalSummary), args.Error(1)
}

// ========== TrendPeriod Tests ==========

func TestTrendPeriod_IsValid(t *testing.T) {
	testCases := []struct {
		period   analytics.TrendPeriod
		expected bool
	}{
		{analytics.TrendPeriodWeek, true},
		{analytics.TrendPeriodMonth, true},
		{analytics.TrendPeriodQuarter, true},
		{analytics.TrendPeriod("invalid"), false},
		{analytics.TrendPeriod(""), false},
	}

	for _, tc := range testCases {
		t.Run(string(tc.period), func(t *testing.T) {
			assert.Equal(t, tc.expected, tc.period.IsValid())
		})
	}
}

// ========== Service.GetWeeklySummary Tests ==========

func TestService_GetWeeklySummary_Success(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)
	ctx := context.Background()
	userID := "user-123"

	filter := analytics.WeeklySummaryFilter{
		WeekStart: "2024-01-15",
		// No timezone = defaults to UTC
	}

	// With UTC timezone, dateToUtcRange("2024-01-15", "2024-01-21", "UTC") gives:
	// start: "2024-01-15T00:00:00Z", end: "2024-01-22T00:00:00Z" (exclusive)
	startDt := "2024-01-15T00:00:00Z"
	endDt := "2024-01-22T00:00:00Z"

	// Setup mock expectations with UTC datetime strings
	mockRepo.On("GetTotalMinutesByDateRange", ctx, userID, startDt, endDt).Return(1500, nil)
	mockRepo.On("GetMinutesByGoal", ctx, userID, startDt, endDt).Return([]repository.GoalWithMinutes{
		{ID: "goal-1", Name: "GK", Color: "#0EA5E9", Minutes: 900},
		{ID: "goal-2", Name: "OSS", Color: "#10B981", Minutes: 600},
	}, nil)
	mockRepo.On("GetMinutesByTag", ctx, userID, startDt, endDt).Return([]repository.TagWithMinutes{}, nil)
	mockRepo.On("GetMinutesByMilestone", ctx, userID, startDt, endDt).Return([]repository.MilestoneWithMinutes{}, nil)
	mockRepo.On("GetCompletedTasksCount", ctx, userID, "2024-01-15", "2024-01-21").Return(15, nil)
	mockRepo.On("GetTimeBlocksAggregated", ctx, userID, startDt, endDt).Return(1800, nil)

	result, err := svc.GetWeeklySummary(ctx, userID, filter)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "2024-01-15", result.WeekStart)
	assert.Equal(t, "2024-01-21", result.WeekEnd)
	assert.Equal(t, 1500, result.TotalMinutes)
	assert.Len(t, result.ByGoal, 2)
	assert.Equal(t, 900, result.ByGoal[0].Minutes)
	assert.Equal(t, 600, result.ByGoal[1].Minutes)
	assert.Equal(t, 15, result.CompletedTasks)
	assert.Equal(t, 1800, result.PlannedVsActual.Planned)
	assert.Equal(t, 1500, result.PlannedVsActual.Actual)
	mockRepo.AssertExpectations(t)
}

func TestService_GetWeeklySummary_InvalidWeekStart(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)
	ctx := context.Background()
	userID := "user-123"

	filter := analytics.WeeklySummaryFilter{
		WeekStart: "invalid-date",
	}

	result, err := svc.GetWeeklySummary(ctx, userID, filter)

	assert.Nil(t, result)
	assert.Equal(t, ErrInvalidWeekStart, err)
}

func TestService_GetWeeklySummary_RepositoryError(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)
	ctx := context.Background()
	userID := "user-123"

	filter := analytics.WeeklySummaryFilter{
		WeekStart: "2024-01-15",
	}

	startDt := "2024-01-15T00:00:00Z"
	endDt := "2024-01-22T00:00:00Z"

	repoErr := errors.New("database connection failed")
	mockRepo.On("GetTotalMinutesByDateRange", ctx, userID, startDt, endDt).Return(0, repoErr)

	result, err := svc.GetWeeklySummary(ctx, userID, filter)

	assert.Nil(t, result)
	assert.Equal(t, repoErr, err)
	mockRepo.AssertExpectations(t)
}

func TestService_GetWeeklySummary_EmptyData(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)
	ctx := context.Background()
	userID := "user-123"

	filter := analytics.WeeklySummaryFilter{
		WeekStart: "2024-01-15",
	}

	startDt := "2024-01-15T00:00:00Z"
	endDt := "2024-01-22T00:00:00Z"

	// Return empty data
	mockRepo.On("GetTotalMinutesByDateRange", ctx, userID, startDt, endDt).Return(0, nil)
	mockRepo.On("GetMinutesByGoal", ctx, userID, startDt, endDt).Return([]repository.GoalWithMinutes{}, nil)
	mockRepo.On("GetMinutesByTag", ctx, userID, startDt, endDt).Return([]repository.TagWithMinutes{}, nil)
	mockRepo.On("GetMinutesByMilestone", ctx, userID, startDt, endDt).Return([]repository.MilestoneWithMinutes{}, nil)
	mockRepo.On("GetCompletedTasksCount", ctx, userID, "2024-01-15", "2024-01-21").Return(0, nil)
	mockRepo.On("GetTimeBlocksAggregated", ctx, userID, startDt, endDt).Return(0, nil)

	result, err := svc.GetWeeklySummary(ctx, userID, filter)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotNil(t, result.ByGoal)
	assert.NotNil(t, result.ByTag)
	assert.NotNil(t, result.ByMilestone)
	mockRepo.AssertExpectations(t)
}

// ========== Service.GetMonthlySummary Tests ==========

func TestService_GetMonthlySummary_Success(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)
	ctx := context.Background()
	userID := "user-123"

	filter := analytics.MonthlySummaryFilter{
		Year:  2024,
		Month: 1,
		// No timezone = defaults to UTC
	}

	// With UTC timezone, dateToUtcRange("2024-01-01", "2024-01-31", "UTC") gives:
	// start: "2024-01-01T00:00:00Z", end: "2024-02-01T00:00:00Z" (exclusive)
	startDt := "2024-01-01T00:00:00Z"
	endDt := "2024-02-01T00:00:00Z"

	// Setup mock expectations for January 2024
	mockRepo.On("GetTotalMinutesByDateRange", ctx, userID, startDt, endDt).Return(6000, nil)
	mockRepo.On("GetMinutesByGoal", ctx, userID, startDt, endDt).Return([]repository.GoalWithMinutes{
		{ID: "goal-1", Name: "GK", Color: "#0EA5E9", Minutes: 3000},
		{ID: "goal-2", Name: "OSS", Color: "#10B981", Minutes: 2000},
	}, nil)
	mockRepo.On("GetMinutesByTag", ctx, userID, startDt, endDt).Return([]repository.TagWithMinutes{}, nil)
	mockRepo.On("GetMinutesByMilestone", ctx, userID, startDt, endDt).Return([]repository.MilestoneWithMinutes{}, nil)
	mockRepo.On("GetCompletedTasksCount", ctx, userID, "2024-01-01", "2024-01-31").Return(50, nil)
	mockRepo.On("GetTimeBlocksAggregated", ctx, userID, startDt, endDt).Return(7000, nil)
	mockRepo.On("GetWeeklyBreakdown", ctx, userID, startDt, endDt, "UTC").Return([]analytics.DailyBreakdown{
		{Date: "Week 1", Minutes: 1500},
		{Date: "Week 2", Minutes: 1500},
	}, nil)

	result, err := svc.GetMonthlySummary(ctx, userID, filter)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 2024, result.Year)
	assert.Equal(t, 1, result.Month)
	assert.Equal(t, 6000, result.TotalMinutes)
	assert.Equal(t, 50, result.CompletedTasks)
	assert.Equal(t, 7000, result.PlannedVsActual.Planned)
	mockRepo.AssertExpectations(t)
}

func TestService_GetMonthlySummary_InvalidMonth(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)
	ctx := context.Background()
	userID := "user-123"

	testCases := []struct {
		name   string
		filter analytics.MonthlySummaryFilter
	}{
		{
			name:   "month 0",
			filter: analytics.MonthlySummaryFilter{Year: 2024, Month: 0},
		},
		{
			name:   "month 13",
			filter: analytics.MonthlySummaryFilter{Year: 2024, Month: 13},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Note: Month 0 defaults to current month, so only Month 13 should fail
			if tc.filter.Month == 13 {
				result, err := svc.GetMonthlySummary(ctx, userID, tc.filter)
				assert.Nil(t, result)
				assert.Equal(t, ErrInvalidMonth, err)
			}
		})
	}
}

func TestService_GetMonthlySummary_InvalidYear(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)
	ctx := context.Background()
	userID := "user-123"

	testCases := []struct {
		name   string
		filter analytics.MonthlySummaryFilter
	}{
		{
			name:   "year 1999",
			filter: analytics.MonthlySummaryFilter{Year: 1999, Month: 6},
		},
		{
			name:   "year 2101",
			filter: analytics.MonthlySummaryFilter{Year: 2101, Month: 6},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := svc.GetMonthlySummary(ctx, userID, tc.filter)
			assert.Nil(t, result)
			assert.Equal(t, ErrInvalidYear, err)
		})
	}
}

// ========== Service.GetTrends Tests ==========

func TestService_GetTrends_Success(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)
	ctx := context.Background()
	userID := "user-123"

	filter := analytics.TrendFilter{
		Period: analytics.TrendPeriodWeek,
		Count:  2,
	}

	// Mock will be called for each period with UTC datetime strings
	mockRepo.On("GetTotalMinutesByDateRange", ctx, userID, mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(1500, nil)

	result, err := svc.GetTrends(ctx, userID, filter)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result, 2)
	mockRepo.AssertExpectations(t)
}

func TestService_GetTrends_InvalidPeriod(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)
	ctx := context.Background()
	userID := "user-123"

	filter := analytics.TrendFilter{
		Period: analytics.TrendPeriod("invalid"),
		Count:  4,
	}

	result, err := svc.GetTrends(ctx, userID, filter)

	assert.Nil(t, result)
	assert.Equal(t, ErrInvalidPeriod, err)
}

func TestService_GetTrends_DefaultsApplied(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)
	ctx := context.Background()
	userID := "user-123"

	// Empty filter should use defaults
	filter := analytics.TrendFilter{}

	mockRepo.On("GetTotalMinutesByDateRange", ctx, userID, mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(1000, nil)

	result, err := svc.GetTrends(ctx, userID, filter)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	// Default count is 4
	assert.Len(t, result, 4)
	mockRepo.AssertExpectations(t)
}

func TestService_GetTrends_CountLimitedTo52(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)
	ctx := context.Background()
	userID := "user-123"

	filter := analytics.TrendFilter{
		Period: analytics.TrendPeriodWeek,
		Count:  100, // Should be capped at 52
	}

	mockRepo.On("GetTotalMinutesByDateRange", ctx, userID, mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(500, nil)

	result, err := svc.GetTrends(ctx, userID, filter)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result, 52)
	mockRepo.AssertExpectations(t)
}
