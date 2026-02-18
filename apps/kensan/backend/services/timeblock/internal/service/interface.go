package service

import (
	"context"

	timeblock "github.com/kensan/backend/services/timeblock/internal"
)

// TimeBlockService defines the interface for time block operations
type TimeBlockService interface {
	ListTimeBlocks(ctx context.Context, userID string, filter timeblock.TimeBlockFilter) ([]timeblock.TimeBlock, error)
	GetTimeBlock(ctx context.Context, userID, timeBlockID string) (*timeblock.TimeBlock, error)
	CreateTimeBlock(ctx context.Context, userID string, input timeblock.CreateTimeBlockInput) (*timeblock.TimeBlock, error)
	UpdateTimeBlock(ctx context.Context, userID, timeBlockID string, input timeblock.UpdateTimeBlockInput) (*timeblock.TimeBlock, error)
	DeleteTimeBlock(ctx context.Context, userID, timeBlockID string) error
}

// TimeEntryService defines the interface for time entry operations
type TimeEntryService interface {
	ListTimeEntries(ctx context.Context, userID string, filter timeblock.TimeEntryFilter) ([]timeblock.TimeEntry, error)
	GetTimeEntry(ctx context.Context, userID, timeEntryID string) (*timeblock.TimeEntry, error)
	CreateTimeEntry(ctx context.Context, userID string, input timeblock.CreateTimeEntryInput) (*timeblock.TimeEntry, error)
	UpdateTimeEntry(ctx context.Context, userID, timeEntryID string, input timeblock.UpdateTimeEntryInput) (*timeblock.TimeEntry, error)
	DeleteTimeEntry(ctx context.Context, userID, timeEntryID string) error
}

// TimerService defines the interface for timer operations
type TimerService interface {
	GetRunningTimer(ctx context.Context, userID string) (*timeblock.RunningTimer, error)
	StartTimer(ctx context.Context, userID string, input timeblock.StartTimerInput) (*timeblock.RunningTimer, error)
	StopTimer(ctx context.Context, userID string) (*timeblock.StopTimerResult, error)
}

// FullService is the combined interface that embeds all timeblock service interfaces
type FullService interface {
	TimeBlockService
	TimeEntryService
	TimerService
}

// Compile-time check to ensure Service implements FullService
var _ FullService = (*Service)(nil)
