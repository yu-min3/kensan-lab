package note

import (
	"time"

	"github.com/kensan/backend/shared/types"
)

// NoteType represents the type of note
type NoteType string

// Well-known note type constants (for backward compatibility)
const (
	NoteTypeDiary    NoteType = "diary"
	NoteTypeLearning NoteType = "learning"
)

// IsValid is deprecated: use Service.IsValidNoteType() for dynamic validation.
// Kept for backward compatibility with existing code that doesn't have service access.
func (t NoteType) IsValid() bool {
	return t != ""
}

// ============================================
// NoteTypeConfig - データ駆動型ノートタイプ定義
// ============================================

// NoteTypeConfig represents a note type configuration from the database
type NoteTypeConfig struct {
	ID             string          `json:"id"`
	Slug           string          `json:"slug"`
	DisplayName    string          `json:"displayName"`
	DisplayNameEn  *string         `json:"displayNameEn,omitempty"`
	Description    *string         `json:"description,omitempty"`
	Icon           string          `json:"icon"`
	Color          string          `json:"color"`
	Constraints    TypeConstraints `json:"constraints"`
	MetadataSchema []FieldSchema   `json:"metadataSchema"`
	SortOrder      int             `json:"sortOrder"`
	IsSystem       bool            `json:"isSystem"`
	IsActive       bool            `json:"isActive"`
}

// TypeConstraints defines the constraints for a note type
type TypeConstraints struct {
	DateRequired    bool `json:"dateRequired"`
	TitleRequired   bool `json:"titleRequired"`
	ContentRequired bool `json:"contentRequired"`
	DailyUnique     bool `json:"dailyUnique"`
}

// FieldSchema defines a metadata field schema
type FieldSchema struct {
	Key         string            `json:"key"`
	Label       string            `json:"label"`
	LabelEn     *string           `json:"labelEn,omitempty"`
	Type        string            `json:"type"` // string, integer, float, boolean, enum, date, url
	Required    bool              `json:"required"`
	Constraints map[string]any    `json:"constraints,omitempty"`
}

// NoteFormat represents the format of note content
type NoteFormat string

const (
	NoteFormatMarkdown NoteFormat = "markdown"
	NoteFormatDrawio   NoteFormat = "drawio"
)

// IsValid checks if the note format is valid
func (f NoteFormat) IsValid() bool {
	switch f {
	case NoteFormatMarkdown, NoteFormatDrawio:
		return true
	}
	return false
}

// Note represents a unified note entity (diary, learning record)
type Note struct {
	ID                  string             `json:"id"`
	UserID              string             `json:"userId"`
	Type                NoteType           `json:"type"`
	Title               *string            `json:"title,omitempty"`
	Content             string             `json:"content"`
	Format              NoteFormat         `json:"format"`
	Date                types.DateOnly     `json:"date,omitempty"`
	TaskID              *string            `json:"taskId,omitempty"`
	MilestoneID         *string            `json:"milestoneId,omitempty"`
	MilestoneName       *string            `json:"milestoneName,omitempty"`
	GoalID              *string            `json:"goalId,omitempty"`
	GoalName            *string            `json:"goalName,omitempty"`
	GoalColor           *string            `json:"goalColor,omitempty"`
	TagIDs              []string           `json:"tagIds,omitempty"`
	Metadata            []NoteMetadataItem `json:"metadata,omitempty"`
	RelatedTimeEntryIDs []string           `json:"relatedTimeEntryIds,omitempty"`
	FileURL             *string            `json:"fileUrl,omitempty"`
	Archived            bool               `json:"archived"`
	CreatedAt           time.Time          `json:"createdAt"`
	UpdatedAt           time.Time          `json:"updatedAt"`
}

// NoteListItem represents a note without content (for list response)
type NoteListItem struct {
	ID                  string         `json:"id"`
	UserID              string         `json:"userId"`
	Type                NoteType       `json:"type"`
	Title               *string        `json:"title,omitempty"`
	Format              NoteFormat     `json:"format"`
	Date                types.DateOnly `json:"date,omitempty"`
	TaskID              *string        `json:"taskId,omitempty"`
	MilestoneID         *string        `json:"milestoneId,omitempty"`
	MilestoneName       *string        `json:"milestoneName,omitempty"`
	GoalID              *string        `json:"goalId,omitempty"`
	GoalName            *string        `json:"goalName,omitempty"`
	GoalColor           *string        `json:"goalColor,omitempty"`
	TagIDs              []string       `json:"tagIds,omitempty"`
	RelatedTimeEntryIDs []string       `json:"relatedTimeEntryIds,omitempty"`
	FileURL             *string        `json:"fileUrl,omitempty"`
	Archived            bool           `json:"archived"`
	CreatedAt           time.Time      `json:"createdAt"`
	UpdatedAt           time.Time      `json:"updatedAt"`
}

// ToListItem converts a Note to NoteListItem (without content)
func (n *Note) ToListItem() *NoteListItem {
	return &NoteListItem{
		ID:                  n.ID,
		UserID:              n.UserID,
		Type:                n.Type,
		Title:               n.Title,
		Format:              n.Format,
		Date:                n.Date,
		TaskID:              n.TaskID,
		MilestoneID:         n.MilestoneID,
		MilestoneName:       n.MilestoneName,
		GoalID:              n.GoalID,
		GoalName:            n.GoalName,
		GoalColor:           n.GoalColor,
		TagIDs:              n.TagIDs,
		RelatedTimeEntryIDs: n.RelatedTimeEntryIDs,
		FileURL:             n.FileURL,
		Archived:            n.Archived,
		CreatedAt:           n.CreatedAt,
		UpdatedAt:           n.UpdatedAt,
	}
}

// CreateNoteInput represents the input for creating a note
type CreateNoteInput struct {
	Type                NoteType               `json:"type"`
	Title               *string                `json:"title,omitempty"`
	Content             string                 `json:"content"`
	Format              NoteFormat             `json:"format"`
	Date                types.DateOnly         `json:"date,omitempty"`
	TaskID              *string                `json:"taskId,omitempty"`
	MilestoneID         *string                `json:"milestoneId,omitempty"`
	MilestoneName       *string                `json:"milestoneName,omitempty"`
	GoalID              *string                `json:"goalId,omitempty"`
	GoalName            *string                `json:"goalName,omitempty"`
	GoalColor           *string                `json:"goalColor,omitempty"`
	TagIDs              []string               `json:"tagIds,omitempty"`
	Metadata            []SetNoteMetadataInput `json:"metadata,omitempty"`
	RelatedTimeEntryIDs []string               `json:"relatedTimeEntryIds,omitempty"`
	FileURL             *string                `json:"fileUrl,omitempty"`
}

// UpdateNoteInput represents the input for updating a note
type UpdateNoteInput struct {
	Title               *string                `json:"title,omitempty"`
	Content             *string                `json:"content,omitempty"`
	Format              *NoteFormat            `json:"format,omitempty"`
	Date                *types.DateOnly        `json:"date,omitempty"`
	TaskID              *string                `json:"taskId,omitempty"`
	MilestoneID         *string                `json:"milestoneId,omitempty"`
	MilestoneName       *string                `json:"milestoneName,omitempty"`
	GoalID              *string                `json:"goalId,omitempty"`
	GoalName            *string                `json:"goalName,omitempty"`
	GoalColor           *string                `json:"goalColor,omitempty"`
	TagIDs              []string               `json:"tagIds,omitempty"`
	Metadata            []SetNoteMetadataInput `json:"metadata,omitempty"`
	RelatedTimeEntryIDs []string               `json:"relatedTimeEntryIds,omitempty"`
	FileURL             *string                `json:"fileUrl,omitempty"`
	Archived            *bool                  `json:"archived,omitempty"`
}

// NoteFilter represents filters for listing notes
type NoteFilter struct {
	Types       []NoteType  // Filter by types
	GoalID      *string     // Filter by goal
	MilestoneID *string     // Filter by milestone
	TaskID      *string     // Filter by task
	TagIDs      []string    // Filter by tags (AND condition)
	DateFrom    *string     // Filter by date range start (YYYY-MM-DD)
	DateTo      *string     // Filter by date range end (YYYY-MM-DD)
	Archived    *bool       // Filter by archived status
	Format      *NoteFormat // Filter by format
	Query       *string     // For full-text search on title and content
}

// SearchResult represents a result from search
type SearchResult struct {
	Note  *NoteListItem `json:"note"`
	Score float64       `json:"score"` // Relevance score
}

// ============================================
// NoteContent - 複数コンテンツ対応
// ============================================

// ContentType represents the type of content
type ContentType string

const (
	ContentTypeMarkdown ContentType = "markdown"
	ContentTypeDrawio   ContentType = "drawio"
	ContentTypeImage    ContentType = "image"
	ContentTypePDF      ContentType = "pdf"
	ContentTypeCode     ContentType = "code"
	ContentTypeMindmap  ContentType = "mindmap"
)

// IsValid checks if the content type is valid
func (t ContentType) IsValid() bool {
	switch t {
	case ContentTypeMarkdown, ContentTypeDrawio, ContentTypeImage, ContentTypePDF, ContentTypeCode, ContentTypeMindmap:
		return true
	}
	return false
}

// StorageProvider represents the storage backend
type StorageProvider string

const (
	StorageProviderMinIO StorageProvider = "minio"
	StorageProviderR2    StorageProvider = "r2"
	StorageProviderS3    StorageProvider = "s3"
	StorageProviderLocal StorageProvider = "local"
)

// IndexStatus represents the indexing status of a note
type IndexStatus string

const (
	IndexStatusPending    IndexStatus = "pending"
	IndexStatusProcessing IndexStatus = "processing"
	IndexStatusIndexed    IndexStatus = "indexed"
	IndexStatusFailed     IndexStatus = "failed"
)

// NoteContent represents a content item within a note
type NoteContent struct {
	ID              string           `json:"id"`
	NoteID          string           `json:"noteId"`
	ContentType     ContentType      `json:"contentType"`
	Content         *string          `json:"content,omitempty"`         // インラインコンテンツ
	StorageProvider *StorageProvider `json:"storageProvider,omitempty"` // ストレージ種別
	StorageKey      *string          `json:"storageKey,omitempty"`      // ストレージ内のキー
	FileName        *string          `json:"fileName,omitempty"`
	MimeType        *string          `json:"mimeType,omitempty"`
	FileSizeBytes   *int64           `json:"fileSizeBytes,omitempty"`
	Checksum        *string          `json:"checksum,omitempty"`
	ThumbnailBase64 *string          `json:"thumbnailBase64,omitempty"`
	SortOrder       int              `json:"sortOrder"`
	Metadata        map[string]any   `json:"metadata,omitempty"`
	CreatedAt       time.Time        `json:"createdAt"`
	UpdatedAt       time.Time        `json:"updatedAt"`
}

// GetContentURL returns the public URL for accessing the content
func (c *NoteContent) GetContentURL(publicBaseURL string) string {
	if c.StorageKey != nil && *c.StorageKey != "" {
		return publicBaseURL + "/" + *c.StorageKey
	}
	return ""
}

// CreateNoteContentInput represents input for creating a note content
type CreateNoteContentInput struct {
	ContentType     ContentType      `json:"contentType"`
	Content         *string          `json:"content,omitempty"`
	StorageProvider *StorageProvider `json:"storageProvider,omitempty"`
	StorageKey      *string          `json:"storageKey,omitempty"`
	FileName        *string          `json:"fileName,omitempty"`
	MimeType        *string          `json:"mimeType,omitempty"`
	FileSizeBytes   *int64           `json:"fileSizeBytes,omitempty"`
	Checksum        *string          `json:"checksum,omitempty"`
	ThumbnailBase64 *string          `json:"thumbnailBase64,omitempty"`
	SortOrder       *int             `json:"sortOrder,omitempty"`
	Metadata        map[string]any   `json:"metadata,omitempty"`
}

// UpdateNoteContentInput represents input for updating a note content
type UpdateNoteContentInput struct {
	Content         *string        `json:"content,omitempty"`
	SortOrder       *int           `json:"sortOrder,omitempty"`
	Metadata        map[string]any `json:"metadata,omitempty"`
	ThumbnailBase64 *string        `json:"thumbnailBase64,omitempty"`
}

// UploadURLRequest represents request for getting upload URL
type UploadURLRequest struct {
	FileName  string `json:"fileName"`
	MimeType  string `json:"mimeType"`
	FileSize  int64  `json:"fileSize"`
}

// UploadURLResponse represents response with presigned upload URL
type UploadURLResponse struct {
	UploadURL  string `json:"uploadUrl"`
	ContentID  string `json:"contentId"`
	StorageKey string `json:"storageKey"`
}

// NoteContentChunk represents a chunk for AI/search indexing
type NoteContentChunk struct {
	ID             string     `json:"id"`
	NoteID         string     `json:"noteId"`
	NoteContentID  string     `json:"noteContentId"`
	ChunkIndex     int        `json:"chunkIndex"`
	ChunkText      string     `json:"chunkText"`
	TokenCount     *int       `json:"tokenCount,omitempty"`
	EmbeddingModel *string    `json:"embeddingModel,omitempty"`
	ProcessedAt    *time.Time `json:"processedAt,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
}

// ChunkSearchResult represents a search result from vector/text search
type ChunkSearchResult struct {
	NoteID        string  `json:"noteId"`
	NoteTitle     string  `json:"noteTitle"`
	ChunkText     string  `json:"chunkText"`
	Score         float64 `json:"score"`
	HighlightText string  `json:"highlightText,omitempty"`
}

// ============================================
// NoteMetadata - メタデータ
// ============================================

// NoteMetadataItem represents a single metadata key-value pair
type NoteMetadataItem struct {
	ID        string    `json:"id"`
	NoteID    string    `json:"noteId"`
	Key       string    `json:"key"`
	Value     *string   `json:"value,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// SetNoteMetadataInput represents input for setting metadata
type SetNoteMetadataInput struct {
	Key   string  `json:"key"`
	Value *string `json:"value,omitempty"`
}

// BulkSetNoteMetadataInput represents input for bulk setting metadata
type BulkSetNoteMetadataInput struct {
	Metadata []SetNoteMetadataInput `json:"metadata"`
}
