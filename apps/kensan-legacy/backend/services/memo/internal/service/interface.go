package service

import (
	"context"

	memo "github.com/kensan/backend/services/memo/internal"
)

// MemoService defines the interface for memo-related operations
type MemoService interface {
	List(ctx context.Context, userID string, filter memo.MemoFilter) ([]memo.Memo, error)
	GetByID(ctx context.Context, userID, memoID string) (*memo.Memo, error)
	Create(ctx context.Context, userID string, input memo.CreateMemoInput) (*memo.Memo, error)
	Update(ctx context.Context, userID, memoID string, input memo.UpdateMemoInput) (*memo.Memo, error)
	Archive(ctx context.Context, userID, memoID string) (*memo.Memo, error)
	Delete(ctx context.Context, userID, memoID string) error
}

// Compile-time check to ensure Service implements MemoService
var _ MemoService = (*Service)(nil)
