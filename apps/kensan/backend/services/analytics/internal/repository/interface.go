package repository

import (
	"context"

	analytics "github.com/kensan/backend/services/analytics/internal"
)

// Repository defines the interface for analytics data access
type Repository interface {
	// GetTimeBlocksAggregated returns aggregated time blocks (planned time) for a datetime range
	GetTimeBlocksAggregated(ctx context.Context, userID, startDatetime, endDatetime string) (int, error)

	// GetCompletedTasksCount returns the count of tasks completed within a date range
	GetCompletedTasksCount(ctx context.Context, userID, startDate, endDate string) (int, error)

	// GetTotalMinutesByDateRange returns total minutes for a datetime range
	GetTotalMinutesByDateRange(ctx context.Context, userID, startDatetime, endDatetime string) (int, error)

	// GetDailyBreakdown returns daily minutes for a datetime range, grouped by local date using timezone
	GetDailyBreakdown(ctx context.Context, userID, startDatetime, endDatetime, timezone string) ([]analytics.DailyBreakdown, error)

	// GetWeeklyBreakdown returns weekly minutes for a datetime range (for monthly summary), using timezone for grouping
	GetWeeklyBreakdown(ctx context.Context, userID, startDatetime, endDatetime, timezone string) ([]analytics.DailyBreakdown, error)

	// GetMinutesByGoal returns minutes aggregated by goal for a datetime range
	GetMinutesByGoal(ctx context.Context, userID, startDatetime, endDatetime string) ([]GoalWithMinutes, error)

	// GetMinutesByMilestone returns minutes aggregated by milestone for a datetime range
	GetMinutesByMilestone(ctx context.Context, userID, startDatetime, endDatetime string) ([]MilestoneWithMinutes, error)

	// GetMinutesByTag returns minutes aggregated by tag for a datetime range
	GetMinutesByTag(ctx context.Context, userID, startDatetime, endDatetime string) ([]TagWithMinutes, error)

	// GetGoals returns all goals for a user
	GetGoals(ctx context.Context, userID string) ([]analytics.GoalSummary, error)
}
