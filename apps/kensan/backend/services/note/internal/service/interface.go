package service

import (
	"context"

	note "github.com/kensan/backend/services/note/internal"
)

// NoteService defines the interface for note operations
type NoteService interface {
	List(ctx context.Context, userID string, filter *note.NoteFilter) ([]*note.NoteListItem, error)
	GetByID(ctx context.Context, userID, noteID string) (*note.Note, error)
	Create(ctx context.Context, userID string, input *note.CreateNoteInput) (*note.Note, error)
	Update(ctx context.Context, userID, noteID string, input *note.UpdateNoteInput) (*note.Note, error)
	Delete(ctx context.Context, userID, noteID string) error
	Archive(ctx context.Context, userID, noteID string, archived bool) (*note.Note, error)
	Search(ctx context.Context, userID string, query string, filter *note.NoteFilter, limit int) ([]*note.SearchResult, error)
	GetNoteTypes(ctx context.Context) ([]*note.NoteTypeConfig, error)
	LoadNoteTypes(ctx context.Context) error
}

// NoteContentService defines the interface for note content operations
type NoteContentService interface {
	ListContents(ctx context.Context, userID, noteID string) ([]*note.NoteContent, error)
	GetContent(ctx context.Context, userID, noteID, contentID string) (*note.NoteContent, error)
	CreateContent(ctx context.Context, userID, noteID string, input *note.CreateNoteContentInput) (*note.NoteContent, error)
	UpdateContent(ctx context.Context, userID, noteID, contentID string, input *note.UpdateNoteContentInput) (*note.NoteContent, error)
	DeleteContent(ctx context.Context, userID, noteID, contentID string) error
	ReorderContents(ctx context.Context, userID, noteID string, contentIDs []string) error
}

// NoteStorageService defines the interface for note storage operations
type NoteStorageService interface {
	GetUploadURL(ctx context.Context, userID, noteID string, req *note.UploadURLRequest) (*note.UploadURLResponse, error)
	GetDownloadURL(ctx context.Context, userID, noteID, contentID string) (string, error)
}

// FullService is the combined interface that embeds all note service interfaces
type FullService interface {
	NoteService
	NoteContentService
	NoteStorageService
}

// Compile-time check to ensure Service implements FullService
var _ FullService = (*Service)(nil)
