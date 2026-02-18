package service

import (
	"context"

	analytics "github.com/kensan/backend/services/analytics/internal"
)

// AnalyticsService defines the interface for analytics-related operations
type AnalyticsService interface {
	GetWeeklySummary(ctx context.Context, userID string, filter analytics.WeeklySummaryFilter) (*analytics.WeeklySummary, error)
	GetMonthlySummary(ctx context.Context, userID string, filter analytics.MonthlySummaryFilter) (*analytics.MonthlySummary, error)
	GetTrends(ctx context.Context, userID string, filter analytics.TrendFilter) ([]analytics.TrendDataPoint, error)
	GetSummaryByDateRange(ctx context.Context, userID string, filter analytics.SummaryFilter) (*analytics.WeeklySummary, error)
	GetDailyStudyHours(ctx context.Context, userID string, filter analytics.DailyStudyHoursFilter) ([]analytics.DailyStudyHour, error)
}

// Compile-time check to ensure Service implements AnalyticsService
var _ AnalyticsService = (*Service)(nil)
