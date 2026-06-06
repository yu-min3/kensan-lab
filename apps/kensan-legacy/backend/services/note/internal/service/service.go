package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	"log/slog"

	"github.com/google/uuid"
	"github.com/kensan/backend/services/note/internal"
	"github.com/kensan/backend/services/note/internal/repository"
	"github.com/kensan/backend/services/note/internal/storage"
	sharedErrors "github.com/kensan/backend/shared/errors"
)

// Service-specific errors
var (
	ErrNoteNotFound       = sharedErrors.NotFound("note")
	ErrNoteAlreadyExists  = repository.ErrNoteAlreadyExists
	ErrTypeRequired       = sharedErrors.Required("type")
	ErrInvalidType        = errors.New("invalid note type")
	ErrTitleRequired      = sharedErrors.Required("title")
	ErrContentRequired    = sharedErrors.Required("content")
	ErrFormatRequired     = sharedErrors.Required("format")
	ErrInvalidFormat      = errors.New("format must be markdown or drawio")
	ErrDateRequired       = errors.New("date is required for this note type")
	ErrQueryRequired      = sharedErrors.Required("query")
	ErrUnauthorized       = sharedErrors.ErrUnauthorized
	ErrStorageUnavailable = errors.New("storage is not configured")
	ErrMetadataValidation = errors.New("metadata validation error")
)

// StorageClient interface for storage operations
type StorageClient interface {
	Upload(ctx context.Context, key string, reader io.Reader, size int64, contentType string) error
	GetPresignedUploadURL(ctx context.Context, key string, expiry time.Duration) (string, error)
	GetPresignedDownloadURL(ctx context.Context, key string, expiry time.Duration) (string, error)
	Delete(ctx context.Context, key string) error
}

const (
	// storageThreshold is the content size above which content is stored externally
	storageThreshold = 100 * 1024 // 100KB
)

// contentTypesAlwaysExternal are content types that always use external storage
var contentTypesAlwaysExternal = map[note.ContentType]bool{
	note.ContentTypeImage: true,
	note.ContentTypePDF:   true,
}

// Service handles note business logic
type Service struct {
	repo      repository.Repository
	storage   StorageClient
	typeCache map[string]*note.NoteTypeConfig
	typeMu    sync.RWMutex
}

// NewService creates a new note service
func NewService(repo repository.Repository, storageClient *storage.Client) *Service {
	var sc StorageClient
	if storageClient != nil {
		sc = storageClient
	}
	return &Service{
		repo:      repo,
		storage:   sc,
		typeCache: make(map[string]*note.NoteTypeConfig),
	}
}

// LoadNoteTypes loads note type configurations from the database into cache.
// Should be called during service initialization.
func (s *Service) LoadNoteTypes(ctx context.Context) error {
	types, err := s.repo.ListNoteTypes(ctx, false)
	if err != nil {
		return fmt.Errorf("failed to load note types: %w", err)
	}

	s.typeMu.Lock()
	defer s.typeMu.Unlock()
	s.typeCache = make(map[string]*note.NoteTypeConfig, len(types))
	for _, t := range types {
		s.typeCache[t.Slug] = t
	}

	return nil
}

// GetNoteTypes returns all active note type configurations
func (s *Service) GetNoteTypes(ctx context.Context) ([]*note.NoteTypeConfig, error) {
	s.typeMu.RLock()
	defer s.typeMu.RUnlock()

	var result []*note.NoteTypeConfig
	for _, t := range s.typeCache {
		if t.IsActive {
			result = append(result, t)
		}
	}
	return result, nil
}

// getNoteTypeConfig returns the config for a given type slug
func (s *Service) getNoteTypeConfig(slug string) *note.NoteTypeConfig {
	s.typeMu.RLock()
	defer s.typeMu.RUnlock()
	return s.typeCache[slug]
}

// IsValidNoteType checks if a note type slug is valid and active
func (s *Service) IsValidNoteType(slug string) bool {
	cfg := s.getNoteTypeConfig(slug)
	return cfg != nil && cfg.IsActive
}

// List retrieves notes for a user with optional filters
func (s *Service) List(ctx context.Context, userID string, filter *note.NoteFilter) ([]*note.NoteListItem, error) {
	return s.repo.List(ctx, userID, filter)
}

// GetByID retrieves a note by ID (with content)
func (s *Service) GetByID(ctx context.Context, userID, noteID string) (*note.Note, error) {
	n, err := s.repo.GetByIDAndUserID(ctx, noteID, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNoteNotFound) {
			return nil, ErrNoteNotFound
		}
		return nil, err
	}
	return n, nil
}

// Create creates a new note
func (s *Service) Create(ctx context.Context, userID string, input *note.CreateNoteInput) (*note.Note, error) {
	// Validate input
	if err := s.validateCreateInput(input); err != nil {
		return nil, err
	}

	// Convert metadata input to metadata items
	var metadataItems []note.NoteMetadataItem
	if len(input.Metadata) > 0 {
		metadataItems = make([]note.NoteMetadataItem, len(input.Metadata))
		for i, m := range input.Metadata {
			metadataItems[i] = note.NoteMetadataItem{Key: m.Key, Value: m.Value}
		}
	}

	// Create note
	n := &note.Note{
		UserID:              userID,
		Type:                input.Type,
		Title:               input.Title,
		Content:             input.Content,
		Format:              input.Format,
		Date:                input.Date,
		TaskID:              input.TaskID,
		MilestoneID:         input.MilestoneID,
		MilestoneName:       input.MilestoneName,
		GoalID:              input.GoalID,
		GoalName:            input.GoalName,
		GoalColor:           input.GoalColor,
		TagIDs:              input.TagIDs,
		Metadata:            metadataItems,
		RelatedTimeEntryIDs: input.RelatedTimeEntryIDs,
		FileURL:             input.FileURL,
		Archived:            false,
	}

	// Trim title if present
	if n.Title != nil {
		trimmed := strings.TrimSpace(*n.Title)
		n.Title = &trimmed
	}

	if err := s.repo.Create(ctx, n); err != nil {
		return nil, err
	}

	return n, nil
}

// Update updates an existing note
func (s *Service) Update(ctx context.Context, userID, noteID string, input *note.UpdateNoteInput) (*note.Note, error) {
	// Get existing note
	n, err := s.repo.GetByIDAndUserID(ctx, noteID, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNoteNotFound) {
			return nil, ErrNoteNotFound
		}
		return nil, err
	}

	// Update fields
	if input.Title != nil {
		title := strings.TrimSpace(*input.Title)
		// Title is always required for notes
		if title == "" {
			return nil, ErrTitleRequired
		}
		n.Title = &title
	}
	if input.Content != nil {
		n.Content = *input.Content
	}
	if input.Format != nil {
		if !input.Format.IsValid() {
			return nil, ErrInvalidFormat
		}
		n.Format = *input.Format
	}
	if input.Date != nil {
		n.Date = *input.Date
	}
	if input.TaskID != nil {
		n.TaskID = input.TaskID
	}
	if input.MilestoneID != nil {
		n.MilestoneID = input.MilestoneID
	}
	if input.MilestoneName != nil {
		n.MilestoneName = input.MilestoneName
	}
	if input.GoalID != nil {
		n.GoalID = input.GoalID
	}
	if input.GoalName != nil {
		n.GoalName = input.GoalName
	}
	if input.GoalColor != nil {
		n.GoalColor = input.GoalColor
	}
	if input.TagIDs != nil {
		n.TagIDs = input.TagIDs
	}
	if input.Metadata != nil {
		metadataItems := make([]note.NoteMetadataItem, len(input.Metadata))
		for i, m := range input.Metadata {
			metadataItems[i] = note.NoteMetadataItem{Key: m.Key, Value: m.Value}
		}
		n.Metadata = metadataItems
	}
	if input.RelatedTimeEntryIDs != nil {
		n.RelatedTimeEntryIDs = input.RelatedTimeEntryIDs
	}
	if input.FileURL != nil {
		n.FileURL = input.FileURL
	}
	if input.Archived != nil {
		n.Archived = *input.Archived
	}

	// Save changes
	if err := s.repo.Update(ctx, n); err != nil {
		return nil, err
	}

	return n, nil
}

// Delete deletes a note
func (s *Service) Delete(ctx context.Context, userID, noteID string) error {
	err := s.repo.DeleteByIDAndUserID(ctx, noteID, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNoteNotFound) {
			return ErrNoteNotFound
		}
		return err
	}
	return nil
}

// Archive archives or unarchives a note
func (s *Service) Archive(ctx context.Context, userID, noteID string, archived bool) (*note.Note, error) {
	return s.Update(ctx, userID, noteID, &note.UpdateNoteInput{
		Archived: &archived,
	})
}

// Search performs a full-text search on notes
func (s *Service) Search(ctx context.Context, userID string, query string, filter *note.NoteFilter, limit int) ([]*note.SearchResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, ErrQueryRequired
	}

	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	return s.repo.Search(ctx, userID, query, filter, limit)
}

// validateCreateInput validates the create input using data-driven type configuration
func (s *Service) validateCreateInput(input *note.CreateNoteInput) error {
	// Validate type
	if input.Type == "" {
		return ErrTypeRequired
	}

	cfg := s.getNoteTypeConfig(string(input.Type))
	if cfg == nil || !cfg.IsActive {
		return ErrInvalidType
	}

	// Validate title based on constraints
	if cfg.Constraints.TitleRequired {
		if input.Title == nil || strings.TrimSpace(*input.Title) == "" {
			return ErrTitleRequired
		}
	}

	// Content and date are not enforced on creation to support draft workflows
	// (e.g., creating a note to upload images before filling in content).
	// The frontend enforces these constraints via its save button validation.

	// Validate format
	if input.Format == "" {
		return ErrFormatRequired
	}
	if !input.Format.IsValid() {
		return ErrInvalidFormat
	}

	// Validate metadata against schema
	if len(cfg.MetadataSchema) > 0 {
		if err := s.validateMetadata(input.Metadata, cfg.MetadataSchema); err != nil {
			return err
		}
	}

	return nil
}

// validateMetadata validates metadata against the type's schema
func (s *Service) validateMetadata(metadata []note.SetNoteMetadataInput, schema []note.FieldSchema) error {
	// Build lookup map from provided metadata
	metaMap := make(map[string]string, len(metadata))
	for _, m := range metadata {
		if m.Value != nil {
			metaMap[m.Key] = *m.Value
		}
	}

	for _, field := range schema {
		value, exists := metaMap[field.Key]

		// Check required fields
		if field.Required && (!exists || value == "") {
			return fmt.Errorf("%w: %s is required", ErrMetadataValidation, field.Label)
		}

		if !exists || value == "" {
			continue
		}

		// Type-based validation
		switch field.Type {
		case "integer":
			n, err := strconv.Atoi(value)
			if err != nil {
				return fmt.Errorf("%w: %s must be an integer", ErrMetadataValidation, field.Label)
			}
			if minVal, ok := field.Constraints["min"]; ok {
				if min, ok := toInt(minVal); ok && n < min {
					return fmt.Errorf("%w: %s must be at least %d", ErrMetadataValidation, field.Label, min)
				}
			}
			if maxVal, ok := field.Constraints["max"]; ok {
				if max, ok := toInt(maxVal); ok && n > max {
					return fmt.Errorf("%w: %s must be at most %d", ErrMetadataValidation, field.Label, max)
				}
			}
		case "enum":
			if valuesRaw, ok := field.Constraints["values"]; ok {
				if values, ok := toStringSlice(valuesRaw); ok {
					found := false
					for _, v := range values {
						if v == value {
							found = true
							break
						}
					}
					if !found {
						return fmt.Errorf("%w: %s must be one of %v", ErrMetadataValidation, field.Label, values)
					}
				}
			}
		}
	}

	return nil
}

// toInt converts an any value to int
func toInt(v any) (int, bool) {
	switch n := v.(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	case int64:
		return int(n), true
	}
	return 0, false
}

// toStringSlice converts an any value to []string
func toStringSlice(v any) ([]string, bool) {
	if arr, ok := v.([]any); ok {
		result := make([]string, 0, len(arr))
		for _, item := range arr {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result, true
	}
	return nil, false
}

// ========== NoteContent Operations ==========

var (
	ErrContentNotFound     = errors.New("content not found")
	ErrContentTypeRequired = errors.New("content type is required")
	ErrInvalidContentType  = errors.New("invalid content type")
)

// ListContents retrieves all contents for a note
func (s *Service) ListContents(ctx context.Context, userID, noteID string) ([]*note.NoteContent, error) {
	// Verify note belongs to user
	_, err := s.repo.GetByIDAndUserID(ctx, noteID, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNoteNotFound) {
			return nil, ErrNoteNotFound
		}
		return nil, err
	}

	return s.repo.ListContents(ctx, noteID)
}

// GetContent retrieves a content by ID
func (s *Service) GetContent(ctx context.Context, userID, noteID, contentID string) (*note.NoteContent, error) {
	// Verify note belongs to user
	_, err := s.repo.GetByIDAndUserID(ctx, noteID, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNoteNotFound) {
			return nil, ErrNoteNotFound
		}
		return nil, err
	}

	content, err := s.repo.GetContent(ctx, contentID)
	if err != nil {
		return nil, err
	}
	if content == nil || content.NoteID != noteID {
		return nil, ErrContentNotFound
	}

	return content, nil
}

// shouldUseExternalStorage determines if content should be stored in external storage.
// Returns true if content type is binary (image/pdf) or content exceeds the size threshold.
func (s *Service) shouldUseExternalStorage(input *note.CreateNoteContentInput) bool {
	if s.storage == nil {
		return false
	}
	// Binary types always go to external storage
	if contentTypesAlwaysExternal[input.ContentType] {
		return true
	}
	// Text content exceeding threshold goes to external storage
	if input.Content != nil && len(*input.Content) > storageThreshold {
		return true
	}
	// Explicit file size exceeding threshold
	if input.FileSizeBytes != nil && *input.FileSizeBytes > storageThreshold {
		return true
	}
	return false
}

// CreateContent creates a new note content.
// If storage is configured, content exceeding 100KB or binary types (image, pdf)
// are automatically uploaded to external storage.
func (s *Service) CreateContent(ctx context.Context, userID, noteID string, input *note.CreateNoteContentInput) (*note.NoteContent, error) {
	// Verify note belongs to user
	_, err := s.repo.GetByIDAndUserID(ctx, noteID, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNoteNotFound) {
			return nil, ErrNoteNotFound
		}
		return nil, err
	}

	// Validate input
	if err := s.validateContentInput(input); err != nil {
		return nil, err
	}

	// Determine sort order
	sortOrder := 0
	if input.SortOrder != nil {
		sortOrder = *input.SortOrder
	} else {
		// Get current max sort order
		contents, err := s.repo.ListContents(ctx, noteID)
		if err == nil && len(contents) > 0 {
			sortOrder = contents[len(contents)-1].SortOrder + 1
		}
	}

	// Auto-upload to external storage if threshold exceeded or binary type
	storageProvider := input.StorageProvider
	storageKey := input.StorageKey
	inlineContent := input.Content

	if input.StorageKey == nil && s.shouldUseExternalStorage(input) {
		contentID := uuid.New().String()
		ext := storage.GetExtensionForContentType(string(input.ContentType))
		key := storage.GenerateKey(noteID, contentID, ext)

		mimeType := "application/octet-stream"
		if input.MimeType != nil {
			mimeType = *input.MimeType
		} else {
			switch input.ContentType {
			case note.ContentTypeMarkdown:
				mimeType = "text/markdown"
			case note.ContentTypeDrawio:
				mimeType = "application/xml"
			}
		}

		if input.Content != nil {
			data := []byte(*input.Content)
			if err := s.storage.Upload(ctx, key, bytes.NewReader(data), int64(len(data)), mimeType); err != nil {
				return nil, fmt.Errorf("failed to upload content to storage: %w", err)
			}

			provider := note.StorageProviderMinIO
			storageProvider = &provider
			storageKey = &key
			inlineContent = nil // clear inline content since it's in storage
		}
	}

	content := &note.NoteContent{
		NoteID:          noteID,
		ContentType:     input.ContentType,
		Content:         inlineContent,
		StorageProvider: storageProvider,
		StorageKey:      storageKey,
		FileName:        input.FileName,
		MimeType:        input.MimeType,
		FileSizeBytes:   input.FileSizeBytes,
		Checksum:        input.Checksum,
		ThumbnailBase64: input.ThumbnailBase64,
		SortOrder:       sortOrder,
		Metadata:        input.Metadata,
	}

	if err := s.repo.CreateContent(ctx, content); err != nil {
		return nil, err
	}

	// Mark note as needing reindexing
	_ = s.repo.UpdateIndexStatus(ctx, noteID, note.IndexStatusPending)

	return content, nil
}

// UpdateContent updates an existing note content.
// If content was previously inline and the new content exceeds the threshold,
// it will be uploaded to external storage.
func (s *Service) UpdateContent(ctx context.Context, userID, noteID, contentID string, input *note.UpdateNoteContentInput) (*note.NoteContent, error) {
	// Verify note belongs to user
	_, err := s.repo.GetByIDAndUserID(ctx, noteID, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNoteNotFound) {
			return nil, ErrNoteNotFound
		}
		return nil, err
	}

	// Get existing content
	content, err := s.repo.GetContent(ctx, contentID)
	if err != nil {
		return nil, err
	}
	if content == nil || content.NoteID != noteID {
		return nil, ErrContentNotFound
	}

	// Update fields
	if input.Content != nil {
		// Check if new content should be moved to external storage
		if s.storage != nil && content.StorageKey == nil && len(*input.Content) > storageThreshold {
			ext := storage.GetExtensionForContentType(string(content.ContentType))
			key := storage.GenerateKey(noteID, contentID, ext)

			mimeType := "text/plain"
			if content.MimeType != nil {
				mimeType = *content.MimeType
			}

			data := []byte(*input.Content)
			if err := s.storage.Upload(ctx, key, bytes.NewReader(data), int64(len(data)), mimeType); err != nil {
				return nil, fmt.Errorf("failed to upload content to storage: %w", err)
			}

			provider := note.StorageProviderMinIO
			content.StorageProvider = &provider
			content.StorageKey = &key
			content.Content = nil
		} else if content.StorageKey != nil && s.storage != nil {
			// Content is already in storage - update the stored object
			mimeType := "text/plain"
			if content.MimeType != nil {
				mimeType = *content.MimeType
			}
			data := []byte(*input.Content)
			if err := s.storage.Upload(ctx, *content.StorageKey, bytes.NewReader(data), int64(len(data)), mimeType); err != nil {
				return nil, fmt.Errorf("failed to update content in storage: %w", err)
			}
			content.Content = nil
		} else {
			content.Content = input.Content
		}
	}
	if input.SortOrder != nil {
		content.SortOrder = *input.SortOrder
	}
	if input.Metadata != nil {
		content.Metadata = input.Metadata
	}
	if input.ThumbnailBase64 != nil {
		content.ThumbnailBase64 = input.ThumbnailBase64
	}

	if err := s.repo.UpdateContent(ctx, content); err != nil {
		return nil, err
	}

	// Mark note as needing reindexing
	_ = s.repo.UpdateIndexStatus(ctx, noteID, note.IndexStatusPending)

	return content, nil
}

// DeleteContent deletes a note content and cleans up external storage if applicable
func (s *Service) DeleteContent(ctx context.Context, userID, noteID, contentID string) error {
	// Verify note belongs to user
	_, err := s.repo.GetByIDAndUserID(ctx, noteID, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNoteNotFound) {
			return ErrNoteNotFound
		}
		return err
	}

	// Verify content belongs to note
	content, err := s.repo.GetContent(ctx, contentID)
	if err != nil {
		return err
	}
	if content == nil || content.NoteID != noteID {
		return ErrContentNotFound
	}

	// Clean up external storage
	if content.StorageKey != nil && s.storage != nil {
		if err := s.storage.Delete(ctx, *content.StorageKey); err != nil {
			// Log but don't fail - orphaned objects are less harmful than failed deletes
			slog.Warn("Failed to delete storage object", "error", err, "key", *content.StorageKey)
		}
	}

	if err := s.repo.DeleteContent(ctx, contentID); err != nil {
		return err
	}

	// Mark note as needing reindexing
	_ = s.repo.UpdateIndexStatus(ctx, noteID, note.IndexStatusPending)

	return nil
}

// ReorderContents updates the sort order of contents
func (s *Service) ReorderContents(ctx context.Context, userID, noteID string, contentIDs []string) error {
	// Verify note belongs to user
	_, err := s.repo.GetByIDAndUserID(ctx, noteID, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNoteNotFound) {
			return ErrNoteNotFound
		}
		return err
	}

	return s.repo.ReorderContents(ctx, noteID, contentIDs)
}

// validateContentInput validates content input
func (s *Service) validateContentInput(input *note.CreateNoteContentInput) error {
	if input.ContentType == "" {
		return ErrContentTypeRequired
	}
	if !input.ContentType.IsValid() {
		return ErrInvalidContentType
	}
	return nil
}

// GetUploadURL generates a presigned URL for uploading content to storage
func (s *Service) GetUploadURL(ctx context.Context, userID, noteID string, req *note.UploadURLRequest) (*note.UploadURLResponse, error) {
	// Verify note belongs to user
	_, err := s.repo.GetByIDAndUserID(ctx, noteID, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNoteNotFound) {
			return nil, ErrNoteNotFound
		}
		return nil, err
	}

	// Check storage is available
	if s.storage == nil {
		return nil, ErrStorageUnavailable
	}

	// Generate content ID and storage key
	contentID := uuid.New().String()
	storageKey := storage.GenerateKey(noteID, contentID, getExtensionFromMimeType(req.MimeType))

	// Generate presigned URL (valid for 15 minutes)
	uploadURL, err := s.storage.GetPresignedUploadURL(ctx, storageKey, 15*time.Minute)
	if err != nil {
		return nil, err
	}

	return &note.UploadURLResponse{
		UploadURL:  uploadURL,
		ContentID:  contentID,
		StorageKey: storageKey,
	}, nil
}

// GetDownloadURL generates a presigned URL for downloading content from storage
func (s *Service) GetDownloadURL(ctx context.Context, userID, noteID, contentID string) (string, error) {
	// Verify note belongs to user
	_, err := s.repo.GetByIDAndUserID(ctx, noteID, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNoteNotFound) {
			return "", ErrNoteNotFound
		}
		return "", err
	}

	// Get content
	content, err := s.repo.GetContent(ctx, contentID)
	if err != nil {
		return "", err
	}
	if content == nil || content.NoteID != noteID {
		return "", ErrContentNotFound
	}

	// Check storage key exists
	if content.StorageKey == nil || *content.StorageKey == "" {
		return "", errors.New("content is not stored in external storage")
	}

	// Check storage is available
	if s.storage == nil {
		return "", ErrStorageUnavailable
	}

	// Generate presigned URL (valid for 1 hour)
	return s.storage.GetPresignedDownloadURL(ctx, *content.StorageKey, time.Hour)
}

// getExtensionFromMimeType returns file extension from MIME type
func getExtensionFromMimeType(mimeType string) string {
	switch mimeType {
	case "text/markdown", "text/x-markdown":
		return ".md"
	case "application/xml", "text/xml":
		return ".drawio"
	case "image/png":
		return ".png"
	case "image/jpeg":
		return ".jpg"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "application/pdf":
		return ".pdf"
	default:
		return ""
	}
}
