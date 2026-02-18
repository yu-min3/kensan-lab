package repository

import (
	"context"

	"github.com/kensan/backend/services/timeblock/internal"
)

// TimeBlockRepository defines the interface for time block data access
type TimeBlockRepository interface {
	ListTimeBlocks(ctx context.Context, userID string, filter timeblock.TimeBlockFilter) ([]timeblock.TimeBlock, error)
	GetTimeBlockByID(ctx context.Context, userID, timeBlockID string) (*timeblock.TimeBlock, error)
	CreateTimeBlock(ctx context.Context, userID string, input timeblock.CreateTimeBlockInput) (*timeblock.TimeBlock, error)
	UpdateTimeBlock(ctx context.Context, userID, timeBlockID string, input timeblock.UpdateTimeBlockInput) (*timeblock.TimeBlock, error)
	DeleteTimeBlock(ctx context.Context, userID, timeBlockID string) error
	CreateTimeBlockBatch(ctx context.Context, userID string, inputs []timeblock.CreateTimeBlockInput) ([]timeblock.TimeBlock, error)
}

// TimeEntryRepository defines the interface for time entry data access
type TimeEntryRepository interface {
	ListTimeEntries(ctx context.Context, userID string, filter timeblock.TimeEntryFilter) ([]timeblock.TimeEntry, error)
	GetTimeEntryByID(ctx context.Context, userID, timeEntryID string) (*timeblock.TimeEntry, error)
	CreateTimeEntry(ctx context.Context, userID string, input timeblock.CreateTimeEntryInput) (*timeblock.TimeEntry, error)
	UpdateTimeEntry(ctx context.Context, userID, timeEntryID string, input timeblock.UpdateTimeEntryInput) (*timeblock.TimeEntry, error)
	DeleteTimeEntry(ctx context.Context, userID, timeEntryID string) error
}

// TimerRepository defines the interface for timer data access
type TimerRepository interface {
	GetRunningTimer(ctx context.Context, userID string) (*timeblock.RunningTimer, error)
	StartTimer(ctx context.Context, userID string, input timeblock.StartTimerInput) (*timeblock.RunningTimer, error)
	StopTimer(ctx context.Context, userID string) (*timeblock.TimeEntry, error)
}

// Repository is the combined interface that embeds all repository interfaces.
// This provides backward compatibility while allowing services to use specific interfaces
// following the Interface Segregation Principle (ISP).
type Repository interface {
	TimeBlockRepository
	TimeEntryRepository
	TimerRepository
}
