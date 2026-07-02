package service

import (
	"context"
	"fmt"
	"time"

	"github.com/kensan/backend/services/timeblock/internal"
	"github.com/kensan/backend/services/timeblock/internal/repository"
	"github.com/kensan/backend/shared/errors"
)

// Service-specific errors
var (
	ErrTimeBlockNotFound    = errors.NotFound("time block")
	ErrTimeEntryNotFound    = errors.NotFound("time entry")
	ErrRunningTimerNotFound = errors.NotFound("timer")
	ErrTimerAlreadyRunning  = repository.ErrTimerAlreadyRunning
	ErrInvalidInput    = errors.ErrInvalidInput
	ErrInvalidDatetime = fmt.Errorf("invalid datetime: %w", errors.ErrInvalidInput)
)

// Service handles business logic for time blocks and time entries
type Service struct {
	repo repository.Repository
}

// NewService creates a new timeblock service
func NewService(repo repository.Repository) *Service {
	return &Service{repo: repo}
}

// validateDatetime validates that a datetime string is in RFC3339 format and returns the parsed time
func validateDatetime(datetime string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, datetime)
	if err != nil {
		return time.Time{}, ErrInvalidDatetime
	}
	return t, nil
}

// ========== TimeBlock Operations ==========

// ListTimeBlocks returns all time blocks for a user with optional filters
func (s *Service) ListTimeBlocks(ctx context.Context, userID string, filter timeblock.TimeBlockFilter) ([]timeblock.TimeBlock, error) {
	blocks, err := s.repo.ListTimeBlocks(ctx, userID, filter)
	if err != nil {
		return nil, err
	}

	if blocks == nil {
		return []timeblock.TimeBlock{}, nil
	}

	return blocks, nil
}

// GetTimeBlock returns a time block by ID
func (s *Service) GetTimeBlock(ctx context.Context, userID, timeBlockID string) (*timeblock.TimeBlock, error) {
	tb, err := s.repo.GetTimeBlockByID(ctx, userID, timeBlockID)
	if err != nil {
		return nil, err
	}

	if tb == nil {
		return nil, ErrTimeBlockNotFound
	}

	return tb, nil
}

// CreateTimeBlock creates a new time block
func (s *Service) CreateTimeBlock(ctx context.Context, userID string, input timeblock.CreateTimeBlockInput) (*timeblock.TimeBlock, error) {
	// Validate required fields
	if input.TaskName == "" {
		return nil, ErrInvalidInput
	}

	startDt, err := validateDatetime(input.StartDatetime)
	if err != nil {
		return nil, err
	}

	endDt, err := validateDatetime(input.EndDatetime)
	if err != nil {
		return nil, err
	}

	// end must be after start
	if !endDt.After(startDt) {
		return nil, ErrInvalidDatetime
	}

	return s.repo.CreateTimeBlock(ctx, userID, input)
}

// UpdateTimeBlock updates an existing time block
func (s *Service) UpdateTimeBlock(ctx context.Context, userID, timeBlockID string, input timeblock.UpdateTimeBlockInput) (*timeblock.TimeBlock, error) {
	// Check if time block exists and belongs to user
	existing, err := s.repo.GetTimeBlockByID(ctx, userID, timeBlockID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, ErrTimeBlockNotFound
	}

	// Validate optional datetime fields if provided
	if input.StartDatetime != nil {
		if _, err := validateDatetime(*input.StartDatetime); err != nil {
			return nil, err
		}
	}

	if input.EndDatetime != nil {
		if _, err := validateDatetime(*input.EndDatetime); err != nil {
			return nil, err
		}
	}

	tb, err := s.repo.UpdateTimeBlock(ctx, userID, timeBlockID, input)
	if err != nil {
		return nil, err
	}

	if tb == nil {
		return nil, ErrTimeBlockNotFound
	}

	return tb, nil
}

// DeleteTimeBlock deletes a time block
func (s *Service) DeleteTimeBlock(ctx context.Context, userID, timeBlockID string) error {
	// Check if time block exists and belongs to user
	existing, err := s.repo.GetTimeBlockByID(ctx, userID, timeBlockID)
	if err != nil {
		return err
	}
	if existing == nil {
		return ErrTimeBlockNotFound
	}

	return s.repo.DeleteTimeBlock(ctx, userID, timeBlockID)
}

// ========== TimeEntry Operations ==========

// ListTimeEntries returns all time entries for a user with optional filters
func (s *Service) ListTimeEntries(ctx context.Context, userID string, filter timeblock.TimeEntryFilter) ([]timeblock.TimeEntry, error) {
	entries, err := s.repo.ListTimeEntries(ctx, userID, filter)
	if err != nil {
		return nil, err
	}

	if entries == nil {
		return []timeblock.TimeEntry{}, nil
	}

	return entries, nil
}

// GetTimeEntry returns a time entry by ID
func (s *Service) GetTimeEntry(ctx context.Context, userID, timeEntryID string) (*timeblock.TimeEntry, error) {
	te, err := s.repo.GetTimeEntryByID(ctx, userID, timeEntryID)
	if err != nil {
		return nil, err
	}

	if te == nil {
		return nil, ErrTimeEntryNotFound
	}

	return te, nil
}

// CreateTimeEntry creates a new time entry
func (s *Service) CreateTimeEntry(ctx context.Context, userID string, input timeblock.CreateTimeEntryInput) (*timeblock.TimeEntry, error) {
	// Validate required fields
	if input.TaskName == "" {
		return nil, ErrInvalidInput
	}

	startDt, err := validateDatetime(input.StartDatetime)
	if err != nil {
		return nil, err
	}

	endDt, err := validateDatetime(input.EndDatetime)
	if err != nil {
		return nil, err
	}

	// end must be after start
	if !endDt.After(startDt) {
		return nil, ErrInvalidDatetime
	}

	return s.repo.CreateTimeEntry(ctx, userID, input)
}

// UpdateTimeEntry updates an existing time entry
func (s *Service) UpdateTimeEntry(ctx context.Context, userID, timeEntryID string, input timeblock.UpdateTimeEntryInput) (*timeblock.TimeEntry, error) {
	// Check if time entry exists and belongs to user
	existing, err := s.repo.GetTimeEntryByID(ctx, userID, timeEntryID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, ErrTimeEntryNotFound
	}

	// Validate optional datetime fields if provided
	if input.StartDatetime != nil {
		if _, err := validateDatetime(*input.StartDatetime); err != nil {
			return nil, err
		}
	}

	if input.EndDatetime != nil {
		if _, err := validateDatetime(*input.EndDatetime); err != nil {
			return nil, err
		}
	}

	te, err := s.repo.UpdateTimeEntry(ctx, userID, timeEntryID, input)
	if err != nil {
		return nil, err
	}

	if te == nil {
		return nil, ErrTimeEntryNotFound
	}

	return te, nil
}

// DeleteTimeEntry deletes a time entry
func (s *Service) DeleteTimeEntry(ctx context.Context, userID, timeEntryID string) error {
	// Check if time entry exists and belongs to user
	existing, err := s.repo.GetTimeEntryByID(ctx, userID, timeEntryID)
	if err != nil {
		return err
	}
	if existing == nil {
		return ErrTimeEntryNotFound
	}

	return s.repo.DeleteTimeEntry(ctx, userID, timeEntryID)
}

// ========== Timer Operations ==========

// GetRunningTimer returns the current running timer for a user
func (s *Service) GetRunningTimer(ctx context.Context, userID string) (*timeblock.RunningTimer, error) {
	return s.repo.GetRunningTimer(ctx, userID)
}

// StartTimer starts a new timer for a user
func (s *Service) StartTimer(ctx context.Context, userID string, input timeblock.StartTimerInput) (*timeblock.RunningTimer, error) {
	// Validate required fields
	if input.TaskName == "" {
		return nil, ErrInvalidInput
	}

	// Check if a timer is already running
	existing, err := s.repo.GetRunningTimer(ctx, userID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, ErrTimerAlreadyRunning
	}

	return s.repo.StartTimer(ctx, userID, input)
}

// StopTimer stops the current timer and creates a time entry
func (s *Service) StopTimer(ctx context.Context, userID string) (*timeblock.StopTimerResult, error) {
	// Check if a timer is running
	timer, err := s.repo.GetRunningTimer(ctx, userID)
	if err != nil {
		return nil, err
	}
	if timer == nil {
		return nil, ErrRunningTimerNotFound
	}

	// Stop the timer and create a time entry
	entry, err := s.repo.StopTimer(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Calculate duration
	duration := time.Since(timer.StartedAt).Seconds()

	return &timeblock.StopTimerResult{
		TimeEntry: entry,
		Duration:  int64(duration),
	}, nil
}
