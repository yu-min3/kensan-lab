package repository

import (
	"context"

	memo "github.com/kensan/backend/services/memo/internal"
)

// Repository defines the interface for memo data operations
type Repository interface {
	// List returns memos for a user with optional filters
	List(ctx context.Context, userID string, filter memo.MemoFilter) ([]memo.Memo, error)

	// GetByID returns a memo by ID
	GetByID(ctx context.Context, userID, memoID string) (*memo.Memo, error)

	// Create creates a new memo
	Create(ctx context.Context, userID string, input memo.CreateMemoInput) (*memo.Memo, error)

	// Update updates an existing memo
	Update(ctx context.Context, userID, memoID string, input memo.UpdateMemoInput) (*memo.Memo, error)

	// Delete deletes a memo
	Delete(ctx context.Context, userID, memoID string) error
}
