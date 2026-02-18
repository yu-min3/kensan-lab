package repository

import (
	"context"

	"github.com/kensan/backend/services/note/internal"
)

// Repository defines the interface for note data persistence
type Repository interface {
	// GetByIDAndUserID retrieves a note by ID and user ID (with content)
	GetByIDAndUserID(ctx context.Context, id, userID string) (*note.Note, error)

	// List retrieves notes for a user with optional filters (without content)
	List(ctx context.Context, userID string, filter *note.NoteFilter) ([]*note.NoteListItem, error)

	// Create creates a new note
	Create(ctx context.Context, n *note.Note) error

	// Update updates an existing note
	Update(ctx context.Context, n *note.Note) error

	// DeleteByIDAndUserID deletes a note by ID and user ID
	DeleteByIDAndUserID(ctx context.Context, id, userID string) error

	// Search performs a full-text search on notes
	Search(ctx context.Context, userID, query string, filter *note.NoteFilter, limit int) ([]*note.SearchResult, error)

	// ========== NoteType Operations ==========

	// ListNoteTypes retrieves note type configurations
	ListNoteTypes(ctx context.Context, activeOnly bool) ([]*note.NoteTypeConfig, error)

	// GetNoteTypeBySlug retrieves a note type configuration by slug
	GetNoteTypeBySlug(ctx context.Context, slug string) (*note.NoteTypeConfig, error)

	// ========== NoteContent Operations ==========

	// ListContents retrieves all contents for a note
	ListContents(ctx context.Context, noteID string) ([]*note.NoteContent, error)

	// GetContent retrieves a content by ID
	GetContent(ctx context.Context, contentID string) (*note.NoteContent, error)

	// CreateContent creates a new note content
	CreateContent(ctx context.Context, content *note.NoteContent) error

	// UpdateContent updates an existing note content
	UpdateContent(ctx context.Context, content *note.NoteContent) error

	// DeleteContent deletes a note content
	DeleteContent(ctx context.Context, contentID string) error

	// ReorderContents updates the sort order of contents
	ReorderContents(ctx context.Context, noteID string, contentIDs []string) error

	// UpdateIndexStatus updates the index status of a note
	UpdateIndexStatus(ctx context.Context, noteID string, status note.IndexStatus) error

	// ========== NoteMetadata Operations ==========

	// ListMetadata retrieves all metadata for a note
	ListMetadata(ctx context.Context, noteID string) ([]*note.NoteMetadataItem, error)

	// SetMetadata sets a metadata key-value pair (upsert)
	SetMetadata(ctx context.Context, noteID, key string, value *string) error

	// DeleteMetadata deletes a metadata key
	DeleteMetadata(ctx context.Context, noteID, key string) error

	// BulkSetMetadata replaces all metadata for a note
	BulkSetMetadata(ctx context.Context, noteID string, metadata []note.SetNoteMetadataInput) error
}
