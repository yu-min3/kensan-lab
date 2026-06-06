package timeblock

import (
	"time"
)

// TimeBlock represents a planned time block for a day
type TimeBlock struct {
	ID            string    `json:"id"`
	UserID        string    `json:"userId"`
	StartDatetime time.Time `json:"startDatetime"` // UTC ISO 8601
	EndDatetime   time.Time `json:"endDatetime"`   // UTC ISO 8601
	TaskID        *string   `json:"taskId,omitempty"`
	TaskName      string    `json:"taskName"`
	MilestoneID   *string   `json:"milestoneId,omitempty"`
	MilestoneName *string   `json:"milestoneName,omitempty"`
	GoalID        *string   `json:"goalId,omitempty"`
	GoalName      *string   `json:"goalName,omitempty"`
	GoalColor     *string   `json:"goalColor,omitempty"`
	TagIDs    []string  `json:"tagIds,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

// TimeEntry represents an actual time record (実績)
type TimeEntry struct {
	ID            string    `json:"id"`
	UserID        string    `json:"userId"`
	StartDatetime time.Time `json:"startDatetime"` // UTC ISO 8601
	EndDatetime   time.Time `json:"endDatetime"`   // UTC ISO 8601
	TaskID        *string   `json:"taskId,omitempty"`
	TaskName      string    `json:"taskName"`
	MilestoneID   *string   `json:"milestoneId,omitempty"`
	MilestoneName *string   `json:"milestoneName,omitempty"`
	GoalID        *string   `json:"goalId,omitempty"`
	GoalName      *string   `json:"goalName,omitempty"`
	GoalColor     *string   `json:"goalColor,omitempty"`
	TagIDs        []string  `json:"tagIds,omitempty"`
	Description   *string   `json:"description,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

// CreateTimeBlockInput represents the input for creating a time block
type CreateTimeBlockInput struct {
	StartDatetime string   `json:"startDatetime"` // ISO 8601 UTC string
	EndDatetime   string   `json:"endDatetime"`   // ISO 8601 UTC string
	TaskID        *string  `json:"taskId,omitempty"`
	TaskName      string   `json:"taskName"`
	MilestoneID   *string  `json:"milestoneId,omitempty"`
	MilestoneName *string  `json:"milestoneName,omitempty"`
	GoalID        *string  `json:"goalId,omitempty"`
	GoalName      *string  `json:"goalName,omitempty"`
	GoalColor     *string  `json:"goalColor,omitempty"`
	TagIDs []string `json:"tagIds,omitempty"`
}

// UpdateTimeBlockInput represents the input for updating a time block
type UpdateTimeBlockInput struct {
	StartDatetime *string  `json:"startDatetime,omitempty"` // ISO 8601 UTC string
	EndDatetime   *string  `json:"endDatetime,omitempty"`   // ISO 8601 UTC string
	TaskID        *string  `json:"taskId,omitempty"`
	TaskName      *string  `json:"taskName,omitempty"`
	MilestoneID   *string  `json:"milestoneId,omitempty"`
	MilestoneName *string  `json:"milestoneName,omitempty"`
	GoalID        *string  `json:"goalId,omitempty"`
	GoalName      *string  `json:"goalName,omitempty"`
	GoalColor     *string  `json:"goalColor,omitempty"`
	TagIDs        []string `json:"tagIds,omitempty"`
}

// CreateTimeEntryInput represents the input for creating a time entry
type CreateTimeEntryInput struct {
	StartDatetime string   `json:"startDatetime"` // ISO 8601 UTC string
	EndDatetime   string   `json:"endDatetime"`   // ISO 8601 UTC string
	TaskID        *string  `json:"taskId,omitempty"`
	TaskName      string   `json:"taskName"`
	MilestoneID   *string  `json:"milestoneId,omitempty"`
	MilestoneName *string  `json:"milestoneName,omitempty"`
	GoalID        *string  `json:"goalId,omitempty"`
	GoalName      *string  `json:"goalName,omitempty"`
	GoalColor     *string  `json:"goalColor,omitempty"`
	TagIDs        []string `json:"tagIds,omitempty"`
	Description   *string  `json:"description,omitempty"`
}

// UpdateTimeEntryInput represents the input for updating a time entry
type UpdateTimeEntryInput struct {
	StartDatetime *string  `json:"startDatetime,omitempty"` // ISO 8601 UTC string
	EndDatetime   *string  `json:"endDatetime,omitempty"`   // ISO 8601 UTC string
	TaskID        *string  `json:"taskId,omitempty"`
	TaskName      *string  `json:"taskName,omitempty"`
	MilestoneID   *string  `json:"milestoneId,omitempty"`
	MilestoneName *string  `json:"milestoneName,omitempty"`
	GoalID        *string  `json:"goalId,omitempty"`
	GoalName      *string  `json:"goalName,omitempty"`
	GoalColor     *string  `json:"goalColor,omitempty"`
	TagIDs        []string `json:"tagIds,omitempty"`
	Description   *string  `json:"description,omitempty"`
}

// TimeBlockFilter represents filters for listing time blocks
type TimeBlockFilter struct {
	StartDatetime *string // UTC timestamp range start (inclusive), ISO 8601
	EndDatetime   *string // UTC timestamp range end (exclusive), ISO 8601
	GoalID        *string // Filter by goal
	MilestoneID   *string // Filter by milestone
}

// TimeEntryFilter represents filters for listing time entries
type TimeEntryFilter struct {
	StartDatetime *string // UTC timestamp range start (inclusive), ISO 8601
	EndDatetime   *string // UTC timestamp range end (exclusive), ISO 8601
	GoalID        *string // Filter by goal
	MilestoneID   *string // Filter by milestone
}

// RunningTimer represents an active timer for time tracking
type RunningTimer struct {
	ID            string    `json:"id"`
	UserID        string    `json:"userId"`
	TaskID        *string   `json:"taskId,omitempty"`
	TaskName      string    `json:"taskName"`
	MilestoneID   *string   `json:"milestoneId,omitempty"`
	MilestoneName *string   `json:"milestoneName,omitempty"`
	GoalID        *string   `json:"goalId,omitempty"`
	GoalName      *string   `json:"goalName,omitempty"`
	GoalColor     *string   `json:"goalColor,omitempty"`
	TagIDs        []string  `json:"tagIds,omitempty"`
	StartedAt     time.Time `json:"startedAt"`
	CreatedAt     time.Time `json:"createdAt"`
}

// StartTimerInput represents the input for starting a timer
type StartTimerInput struct {
	TaskID        *string  `json:"taskId,omitempty"`
	TaskName      string   `json:"taskName"`
	MilestoneID   *string  `json:"milestoneId,omitempty"`
	MilestoneName *string  `json:"milestoneName,omitempty"`
	GoalID        *string  `json:"goalId,omitempty"`
	GoalName      *string  `json:"goalName,omitempty"`
	GoalColor     *string  `json:"goalColor,omitempty"`
	TagIDs        []string `json:"tagIds,omitempty"`
}

// StopTimerResult represents the result of stopping a timer
type StopTimerResult struct {
	TimeEntry *TimeEntry `json:"timeEntry"`
	Duration  int64      `json:"duration"` // Duration in seconds
}
