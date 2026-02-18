package memo

import (
	"time"
)

// Memo represents a quick memo/note
type Memo struct {
	ID        string    `json:"id"`
	UserID    string    `json:"userId"`
	Content   string    `json:"content"`
	Archived  bool      `json:"archived"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// CreateMemoInput represents the input for creating a memo
type CreateMemoInput struct {
	Content string `json:"content"`
}

// UpdateMemoInput represents the input for updating a memo
type UpdateMemoInput struct {
	Content  *string `json:"content,omitempty"`
	Archived *bool   `json:"archived,omitempty"`
}

// MemoFilter represents filters for listing memos
type MemoFilter struct {
	Archived   *bool   // Filter by archived status
	Date       *string // Filter by date (YYYY-MM-DD)
	Limit      int     // Max number of results
	IncludeAll bool    // Include archived memos too
}
