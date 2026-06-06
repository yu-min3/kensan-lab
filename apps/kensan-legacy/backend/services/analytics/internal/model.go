package analytics

// DailyBreakdown represents time spent on a single day
type DailyBreakdown struct {
	Date    string `json:"date"`
	Minutes int    `json:"minutes"`
}

// PlannedVsActual represents the comparison between planned and actual time
type PlannedVsActual struct {
	Planned int `json:"planned"`
	Actual  int `json:"actual"`
}

// GoalSummary represents aggregated data for a goal (matching frontend format)
type GoalSummary struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Color   string `json:"color"`
	Minutes int    `json:"minutes"`
}

// TagSummary represents aggregated data for a tag (matching frontend format)
type TagSummary struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Color   string `json:"color"`
	Minutes int    `json:"minutes"`
}

// MilestoneSummary represents aggregated data for a milestone (matching frontend format)
type MilestoneSummary struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	GoalID  string `json:"goalId"`
	Minutes int    `json:"minutes"`
}

// WeeklySummary represents a weekly analytics summary
type WeeklySummary struct {
	WeekStart       string             `json:"weekStart"`
	WeekEnd         string             `json:"weekEnd"`
	TotalMinutes    int                `json:"totalMinutes"`
	ByGoal          []GoalSummary      `json:"byGoal"`
	ByTag           []TagSummary       `json:"byTag"`
	ByMilestone     []MilestoneSummary `json:"byMilestone"`
	CompletedTasks  int                `json:"completedTasks"`
	PlannedVsActual PlannedVsActual    `json:"plannedVsActual"`
}

// MonthlySummary represents a monthly analytics summary
type MonthlySummary struct {
	Year            int                `json:"year"`
	Month           int                `json:"month"`
	TotalMinutes    int                `json:"totalMinutes"`
	ByGoal          []GoalSummary      `json:"byGoal"`
	ByTag           []TagSummary       `json:"byTag"`
	ByMilestone     []MilestoneSummary `json:"byMilestone"`
	CompletedTasks  int                `json:"completedTasks"`
	PlannedVsActual PlannedVsActual    `json:"plannedVsActual"`
	WeeklyBreakdown []DailyBreakdown   `json:"weeklyBreakdown"`
}

// TrendDataPoint represents a single data point in trend analysis
type TrendDataPoint struct {
	StartDate    string `json:"startDate"`
	EndDate      string `json:"endDate"`
	TotalMinutes int    `json:"totalMinutes"`
}

// TrendPeriod represents the period for trend analysis
type TrendPeriod string

const (
	TrendPeriodWeek    TrendPeriod = "week"
	TrendPeriodMonth   TrendPeriod = "month"
	TrendPeriodQuarter TrendPeriod = "quarter"
)

// IsValid checks if the trend period is valid
func (p TrendPeriod) IsValid() bool {
	switch p {
	case TrendPeriodWeek, TrendPeriodMonth, TrendPeriodQuarter:
		return true
	}
	return false
}

// WeeklySummaryFilter represents filters for getting weekly summary
type WeeklySummaryFilter struct {
	WeekStart string // YYYY-MM-DD format, should be a Monday
	Timezone  string // IANA timezone (e.g., "Asia/Tokyo") for daily/weekly grouping
}

// MonthlySummaryFilter represents filters for getting monthly summary
type MonthlySummaryFilter struct {
	Year     int
	Month    int
	Timezone string // IANA timezone (e.g., "Asia/Tokyo") for weekly grouping
}

// TrendFilter represents filters for trend analysis
type TrendFilter struct {
	Period TrendPeriod
	Count  int
}

// DailyStudyHour represents study hours for a single day (for chart display)
type DailyStudyHour struct {
	Date  string  `json:"date"`
	Hours float64 `json:"hours"`
	Day   string  `json:"day"`
}

// DailyStudyHoursFilter represents filters for daily study hours
type DailyStudyHoursFilter struct {
	Days      int
	StartDate string // YYYY-MM-DD format (optional, takes precedence over Days)
	EndDate   string // YYYY-MM-DD format (optional, takes precedence over Days)
	Timezone  string // IANA timezone (e.g., "Asia/Tokyo") for daily grouping
}

// SummaryFilter represents filters for getting summary by date range
type SummaryFilter struct {
	StartDate string // YYYY-MM-DD format (required)
	EndDate   string // YYYY-MM-DD format (required)
	Timezone  string // IANA timezone (e.g., "Asia/Tokyo") for grouping
}
