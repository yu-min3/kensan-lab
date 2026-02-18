package service

import (
	"context"

	memo "github.com/kensan/backend/services/memo/internal"
	"github.com/kensan/backend/services/memo/internal/repository"
	"github.com/kensan/backend/shared/errors"
)

// Service-specific errors
var (
	ErrMemoNotFound = errors.NotFound("memo")
	ErrInvalidInput = errors.ErrInvalidInput
)

// Service handles business logic for memos
type Service struct {
	repo repository.Repository
}

// NewService creates a new memo service
func NewService(repo repository.Repository) *Service {
	return &Service{repo: repo}
}

// List returns memos for a user with optional filters
func (s *Service) List(ctx context.Context, userID string, filter memo.MemoFilter) ([]memo.Memo, error) {
	memos, err := s.repo.List(ctx, userID, filter)
	if err != nil {
		return nil, err
	}

	if memos == nil {
		return []memo.Memo{}, nil
	}

	return memos, nil
}

// GetByID returns a memo by ID
func (s *Service) GetByID(ctx context.Context, userID, memoID string) (*memo.Memo, error) {
	m, err := s.repo.GetByID(ctx, userID, memoID)
	if err != nil {
		return nil, err
	}

	if m == nil {
		return nil, ErrMemoNotFound
	}

	return m, nil
}

// Create creates a new memo
func (s *Service) Create(ctx context.Context, userID string, input memo.CreateMemoInput) (*memo.Memo, error) {
	if input.Content == "" {
		return nil, ErrInvalidInput
	}

	return s.repo.Create(ctx, userID, input)
}

// Update updates an existing memo
func (s *Service) Update(ctx context.Context, userID, memoID string, input memo.UpdateMemoInput) (*memo.Memo, error) {
	// Check if memo exists
	existing, err := s.repo.GetByID(ctx, userID, memoID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, ErrMemoNotFound
	}

	m, err := s.repo.Update(ctx, userID, memoID, input)
	if err != nil {
		return nil, err
	}

	if m == nil {
		return nil, ErrMemoNotFound
	}

	return m, nil
}

// Archive archives a memo (convenience method)
func (s *Service) Archive(ctx context.Context, userID, memoID string) (*memo.Memo, error) {
	archived := true
	return s.Update(ctx, userID, memoID, memo.UpdateMemoInput{Archived: &archived})
}

// Delete deletes a memo
func (s *Service) Delete(ctx context.Context, userID, memoID string) error {
	// Check if memo exists
	existing, err := s.repo.GetByID(ctx, userID, memoID)
	if err != nil {
		return err
	}
	if existing == nil {
		return ErrMemoNotFound
	}

	return s.repo.Delete(ctx, userID, memoID)
}
