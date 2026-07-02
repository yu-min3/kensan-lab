package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kensan/backend/services/note/internal"
	sharedErrors "github.com/kensan/backend/shared/errors"
	"github.com/kensan/backend/shared/sqlbuilder"
	"log/slog"
)

var (
	ErrNoteNotFound      = errors.New("note not found")
	ErrNoteAlreadyExists = errors.New("note already exists for this date")
)

// PostgresRepository handles note data persistence using PostgreSQL
type PostgresRepository struct {
	pool *pgxpool.Pool
}

// Ensure PostgresRepository implements Repository interface
var _ Repository = (*PostgresRepository)(nil)

// NewPostgresRepository creates a new PostgreSQL note repository
func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

// GetByIDAndUserID retrieves a note by ID and user ID (with content)
func (r *PostgresRepository) GetByIDAndUserID(ctx context.Context, id, userID string) (*note.Note, error) {
	query := `
		SELECT id, user_id, type, title, content, format, date, task_id,
		       milestone_id, goal_id, milestone_name, goal_name, goal_color,
		       related_time_entry_ids, file_url, archived, created_at, updated_at
		FROM notes
		WHERE id = $1 AND user_id = $2
	`

	n, err := r.scanNote(ctx, r.pool.QueryRow(ctx, query, id, userID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNoteNotFound
		}
		return nil, err
	}

	// Get tag IDs from junction table
	tagIDs, err := r.getTagIDs(ctx, id)
	if err != nil {
		return nil, err
	}
	n.TagIDs = tagIDs

	// Get metadata
	metadataItems, err := r.ListMetadata(ctx, id)
	if err != nil {
		return nil, err
	}
	n.Metadata = convertMetadataToValue(metadataItems)

	return n, nil
}

// List retrieves notes for a user with optional filters (without content)
func (r *PostgresRepository) List(ctx context.Context, userID string, filter *note.NoteFilter) ([]*note.NoteListItem, error) {
	w := sqlbuilder.NewWhereBuilder(userID)

	if filter != nil {
		if len(filter.Types) > 0 {
			typeStrs := make([]string, len(filter.Types))
			for i, t := range filter.Types {
				typeStrs[i] = string(t)
			}
			w.AddInClause("n.type", typeStrs)
		}
		sqlbuilder.AddFilter(w, "n.goal_id", filter.GoalID)
		sqlbuilder.AddFilter(w, "n.milestone_id", filter.MilestoneID)
		sqlbuilder.AddFilter(w, "n.task_id", filter.TaskID)
		if filter.Format != nil {
			w.AddCondition("n.format = $%d", string(*filter.Format))
		}
		sqlbuilder.AddFilterWithCast(w, "n.date", ">=", "", filter.DateFrom)
		sqlbuilder.AddFilterWithCast(w, "n.date", "<=", "", filter.DateTo)
		sqlbuilder.AddFilter(w, "n.archived", filter.Archived)
		if filter.Query != nil && *filter.Query != "" {
			w.AddLike([]string{"n.title", "n.content"}, *filter.Query)
		}
		if len(filter.TagIDs) > 0 {
			for _, tagID := range filter.TagIDs {
				w.AddExists("SELECT 1 FROM note_tags nt WHERE nt.note_id = n.id AND nt.tag_id = $%d", tagID)
			}
		}
	}

	query := fmt.Sprintf(`
		SELECT n.id, n.user_id, n.type, n.title, n.format, n.date, n.task_id,
		       n.milestone_id, n.goal_id, n.milestone_name, n.goal_name, n.goal_color,
		       n.related_time_entry_ids, n.file_url, n.archived, n.created_at, n.updated_at
		FROM notes n
		%s ORDER BY n.created_at DESC
	`, w.WhereClause())

	rows, err := r.pool.Query(ctx, query, w.Args()...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notes []*note.NoteListItem
	for rows.Next() {
		item, err := r.scanNoteListItem(rows)
		if err != nil {
			slog.Error("Failed to scan note list item", "error", err)
			return nil, err
		}

		// Get tag IDs for each note
		tagIDs, err := r.getTagIDs(ctx, item.ID)
		if err != nil {
			slog.Error("Failed to get tag IDs", "error", err, "noteID", item.ID)
			return nil, err
		}
		item.TagIDs = tagIDs

		notes = append(notes, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return notes, nil
}

// Create creates a new note
func (r *PostgresRepository) Create(ctx context.Context, n *note.Note) error {
	// Generate UUID if not set
	if n.ID == "" {
		n.ID = uuid.New().String()
	}

	now := time.Now()
	n.CreatedAt = now
	n.UpdatedAt = now

	query := `
		INSERT INTO notes (
			id, user_id, type, title, content, format, date, task_id,
			milestone_id, milestone_name, goal_id, goal_name, goal_color,
			related_time_entry_ids, file_url, archived, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
	`

	_, err := r.pool.Exec(ctx, query,
		n.ID,
		n.UserID,
		string(n.Type),
		n.Title,
		n.Content,
		string(n.Format),
		n.Date,
		n.TaskID,
		n.MilestoneID,
		n.MilestoneName,
		n.GoalID,
		n.GoalName,
		n.GoalColor,
		n.RelatedTimeEntryIDs,
		n.FileURL,
		n.Archived,
		n.CreatedAt,
		n.UpdatedAt,
	)
	if err != nil {
		if sharedErrors.IsUniqueViolation(err) {
			return ErrNoteAlreadyExists
		}
		return err
	}

	// Insert tag associations
	if len(n.TagIDs) > 0 {
		if err := r.updateTags(ctx, n.ID, n.TagIDs); err != nil {
			return err
		}
	}

	// Insert metadata
	if len(n.Metadata) > 0 {
		metadataInputs := make([]note.SetNoteMetadataInput, len(n.Metadata))
		for i, m := range n.Metadata {
			metadataInputs[i] = note.SetNoteMetadataInput{Key: m.Key, Value: m.Value}
		}
		if err := r.BulkSetMetadata(ctx, n.ID, metadataInputs); err != nil {
			return err
		}
	}

	return nil
}

// Update updates an existing note
func (r *PostgresRepository) Update(ctx context.Context, n *note.Note) error {
	n.UpdatedAt = time.Now()

	query := `
		UPDATE notes
		SET title = $2, content = $3, format = $4, date = $5, task_id = $6,
		    milestone_id = $7, milestone_name = $8, goal_id = $9, goal_name = $10, goal_color = $11,
		    related_time_entry_ids = $12, file_url = $13, archived = $14, updated_at = $15
		WHERE id = $1 AND user_id = $16
	`

	result, err := r.pool.Exec(ctx, query,
		n.ID,
		n.Title,
		n.Content,
		string(n.Format),
		n.Date,
		n.TaskID,
		n.MilestoneID,
		n.MilestoneName,
		n.GoalID,
		n.GoalName,
		n.GoalColor,
		n.RelatedTimeEntryIDs,
		n.FileURL,
		n.Archived,
		n.UpdatedAt,
		n.UserID,
	)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrNoteNotFound
	}

	// Update tag associations
	if err := r.updateTags(ctx, n.ID, n.TagIDs); err != nil {
		return err
	}

	// Update metadata
	if len(n.Metadata) > 0 {
		metadataInputs := make([]note.SetNoteMetadataInput, len(n.Metadata))
		for i, m := range n.Metadata {
			metadataInputs[i] = note.SetNoteMetadataInput{Key: m.Key, Value: m.Value}
		}
		if err := r.BulkSetMetadata(ctx, n.ID, metadataInputs); err != nil {
			return err
		}
	}

	return nil
}

// DeleteByIDAndUserID deletes a note by ID and user ID
func (r *PostgresRepository) DeleteByIDAndUserID(ctx context.Context, id, userID string) error {
	query := `DELETE FROM notes WHERE id = $1 AND user_id = $2`

	result, err := r.pool.Exec(ctx, query, id, userID)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrNoteNotFound
	}

	return nil
}

// Search performs a full-text search on notes
func (r *PostgresRepository) Search(ctx context.Context, userID, query string, filter *note.NoteFilter, limit int) ([]*note.SearchResult, error) {
	if limit <= 0 {
		limit = 20
	}

	w := sqlbuilder.NewWhereBuilder(userID)
	// Add the search pattern as the second arg ($2) for use in both WHERE and SELECT
	w.AddLike([]string{"n.title", "n.content"}, query)

	if filter != nil {
		if len(filter.Types) > 0 {
			typeStrs := make([]string, len(filter.Types))
			for i, t := range filter.Types {
				typeStrs[i] = string(t)
			}
			w.AddInClause("n.type", typeStrs)
		}
		sqlbuilder.AddFilter(w, "n.archived", filter.Archived)
	}

	limitClause := w.AddLimit(limit)

	// Note: We use a separate $2 reference for the CASE expression in the score calculation.
	// The search pattern is at arg position 2 because WhereBuilder puts user_id at $1
	// and AddLike adds the pattern as the next arg.
	sqlQuery := fmt.Sprintf(`
		SELECT n.id, n.user_id, n.type, n.title, n.format, n.date, n.task_id,
		       n.milestone_id, n.goal_id, n.milestone_name, n.goal_name, n.goal_color,
		       n.related_time_entry_ids, n.file_url, n.archived, n.created_at, n.updated_at,
		       CASE
		           WHEN LOWER(n.title) LIKE $2 THEN 1.0
		           WHEN LOWER(n.content) LIKE $2 THEN 0.5
		           ELSE 0.0
		       END as score
		FROM notes n
		%s ORDER BY score DESC, n.created_at DESC %s
	`, w.WhereClause(), limitClause)

	rows, err := r.pool.Query(ctx, sqlQuery, w.Args()...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*note.SearchResult
	for rows.Next() {
		item, score, err := r.scanNoteListItemWithScore(rows)
		if err != nil {
			return nil, err
		}

		// Get tag IDs
		tagIDs, err := r.getTagIDs(ctx, item.ID)
		if err != nil {
			return nil, err
		}
		item.TagIDs = tagIDs

		results = append(results, &note.SearchResult{
			Note:  item,
			Score: score,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return results, nil
}

// updateTags updates the tags for a note (internal use only)
func (r *PostgresRepository) updateTags(ctx context.Context, noteID string, tagIDs []string) error {
	// Delete existing tags
	_, err := r.pool.Exec(ctx, `DELETE FROM note_tags WHERE note_id = $1`, noteID)
	if err != nil {
		return err
	}

	// Insert new tags
	if len(tagIDs) > 0 {
		for _, tagID := range tagIDs {
			_, err := r.pool.Exec(ctx,
				`INSERT INTO note_tags (note_id, tag_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
				noteID, tagID)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// getTagIDs retrieves the tag IDs for a note (internal use only)
func (r *PostgresRepository) getTagIDs(ctx context.Context, noteID string) ([]string, error) {
	rows, err := r.pool.Query(ctx, `SELECT tag_id FROM note_tags WHERE note_id = $1`, noteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tagIDs []string
	for rows.Next() {
		var tagID string
		if err := rows.Scan(&tagID); err != nil {
			return nil, err
		}
		tagIDs = append(tagIDs, tagID)
	}

	return tagIDs, rows.Err()
}

// Helper functions for scanning

func (r *PostgresRepository) scanNote(ctx context.Context, row pgx.Row) (*note.Note, error) {
	var n note.Note
	var title, taskID, milestoneID, milestoneName, goalID, goalName, goalColor, fileURL *string
	var relatedTimeEntryIDs []string

	err := row.Scan(
		&n.ID,
		&n.UserID,
		&n.Type,
		&title,
		&n.Content,
		&n.Format,
		&n.Date,
		&taskID,
		&milestoneID,
		&goalID,
		&milestoneName,
		&goalName,
		&goalColor,
		&relatedTimeEntryIDs,
		&fileURL,
		&n.Archived,
		&n.CreatedAt,
		&n.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	n.Title = title
	n.TaskID = taskID
	n.MilestoneID = milestoneID
	n.MilestoneName = milestoneName
	n.GoalID = goalID
	n.GoalName = goalName
	n.GoalColor = goalColor
	n.RelatedTimeEntryIDs = relatedTimeEntryIDs
	n.FileURL = fileURL

	return &n, nil
}

func (r *PostgresRepository) scanNoteListItem(rows pgx.Rows) (*note.NoteListItem, error) {
	var item note.NoteListItem
	var title, taskID, milestoneID, milestoneName, goalID, goalName, goalColor, fileURL *string
	var relatedTimeEntryIDs []string

	err := rows.Scan(
		&item.ID,
		&item.UserID,
		&item.Type,
		&title,
		&item.Format,
		&item.Date,
		&taskID,
		&milestoneID,
		&goalID,
		&milestoneName,
		&goalName,
		&goalColor,
		&relatedTimeEntryIDs,
		&fileURL,
		&item.Archived,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	item.Title = title
	item.TaskID = taskID
	item.MilestoneID = milestoneID
	item.MilestoneName = milestoneName
	item.GoalID = goalID
	item.GoalName = goalName
	item.GoalColor = goalColor
	item.RelatedTimeEntryIDs = relatedTimeEntryIDs
	item.FileURL = fileURL

	return &item, nil
}

func (r *PostgresRepository) scanNoteListItemWithScore(rows pgx.Rows) (*note.NoteListItem, float64, error) {
	var item note.NoteListItem
	var title, taskID, milestoneID, milestoneName, goalID, goalName, goalColor, fileURL *string
	var relatedTimeEntryIDs []string
	var score float64

	err := rows.Scan(
		&item.ID,
		&item.UserID,
		&item.Type,
		&title,
		&item.Format,
		&item.Date,
		&taskID,
		&milestoneID,
		&goalID,
		&milestoneName,
		&goalName,
		&goalColor,
		&relatedTimeEntryIDs,
		&fileURL,
		&item.Archived,
		&item.CreatedAt,
		&item.UpdatedAt,
		&score,
	)
	if err != nil {
		return nil, 0, err
	}

	item.Title = title
	item.TaskID = taskID
	item.MilestoneID = milestoneID
	item.MilestoneName = milestoneName
	item.GoalID = goalID
	item.GoalName = goalName
	item.GoalColor = goalColor
	item.RelatedTimeEntryIDs = relatedTimeEntryIDs
	item.FileURL = fileURL

	return &item, score, nil
}

// convertMetadataToValue converts []*NoteMetadataItem to []NoteMetadataItem
func convertMetadataToValue(items []*note.NoteMetadataItem) []note.NoteMetadataItem {
	if items == nil {
		return nil
	}
	result := make([]note.NoteMetadataItem, len(items))
	for i, item := range items {
		result[i] = *item
	}
	return result
}

// ========== NoteType Operations ==========

// ListNoteTypes retrieves note type configurations
func (r *PostgresRepository) ListNoteTypes(ctx context.Context, activeOnly bool) ([]*note.NoteTypeConfig, error) {
	query := `
		SELECT id, slug, display_name, display_name_en, description, icon, color,
		       constraints, metadata_schema, sort_order, is_system, is_active
		FROM note_types
	`
	if activeOnly {
		query += ` WHERE is_active = TRUE`
	}
	query += ` ORDER BY sort_order, slug`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list note types: %w", err)
	}
	defer rows.Close()

	var types []*note.NoteTypeConfig
	for rows.Next() {
		t, err := r.scanNoteType(rows)
		if err != nil {
			return nil, err
		}
		types = append(types, t)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating note types: %w", err)
	}

	return types, nil
}

// GetNoteTypeBySlug retrieves a note type configuration by slug
func (r *PostgresRepository) GetNoteTypeBySlug(ctx context.Context, slug string) (*note.NoteTypeConfig, error) {
	query := `
		SELECT id, slug, display_name, display_name_en, description, icon, color,
		       constraints, metadata_schema, sort_order, is_system, is_active
		FROM note_types
		WHERE slug = $1
	`

	t, err := r.scanNoteType(r.pool.QueryRow(ctx, query, slug))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get note type: %w", err)
	}

	return t, nil
}

// scanNoteType scans a row into a NoteTypeConfig struct
func (r *PostgresRepository) scanNoteType(row pgx.Row) (*note.NoteTypeConfig, error) {
	var t note.NoteTypeConfig
	var constraintsJSON, metadataSchemaJSON []byte

	err := row.Scan(
		&t.ID,
		&t.Slug,
		&t.DisplayName,
		&t.DisplayNameEn,
		&t.Description,
		&t.Icon,
		&t.Color,
		&constraintsJSON,
		&metadataSchemaJSON,
		&t.SortOrder,
		&t.IsSystem,
		&t.IsActive,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to scan note type: %w", err)
	}

	if err := json.Unmarshal(constraintsJSON, &t.Constraints); err != nil {
		return nil, fmt.Errorf("failed to parse constraints JSON: %w", err)
	}

	if err := json.Unmarshal(metadataSchemaJSON, &t.MetadataSchema); err != nil {
		return nil, fmt.Errorf("failed to parse metadata_schema JSON: %w", err)
	}

	return &t, nil
}

// ========== NoteContent Operations ==========

// ListContents retrieves all contents for a note
func (r *PostgresRepository) ListContents(ctx context.Context, noteID string) ([]*note.NoteContent, error) {
	query := `
		SELECT id, note_id, content_type, content, storage_provider, storage_key,
		       file_name, mime_type, file_size_bytes, checksum, thumbnail_base64,
		       sort_order, metadata, created_at, updated_at
		FROM note_contents
		WHERE note_id = $1
		ORDER BY sort_order, created_at
	`

	rows, err := r.pool.Query(ctx, query, noteID)
	if err != nil {
		return nil, fmt.Errorf("failed to list contents: %w", err)
	}
	defer rows.Close()

	var contents []*note.NoteContent
	for rows.Next() {
		content, err := r.scanNoteContent(rows)
		if err != nil {
			return nil, err
		}
		contents = append(contents, content)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating contents: %w", err)
	}

	return contents, nil
}

// GetContent retrieves a content by ID
func (r *PostgresRepository) GetContent(ctx context.Context, contentID string) (*note.NoteContent, error) {
	query := `
		SELECT id, note_id, content_type, content, storage_provider, storage_key,
		       file_name, mime_type, file_size_bytes, checksum, thumbnail_base64,
		       sort_order, metadata, created_at, updated_at
		FROM note_contents
		WHERE id = $1
	`

	row := r.pool.QueryRow(ctx, query, contentID)
	content, err := r.scanNoteContent(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return content, nil
}

// CreateContent creates a new note content
func (r *PostgresRepository) CreateContent(ctx context.Context, content *note.NoteContent) error {
	if content.ID == "" {
		content.ID = uuid.New().String()
	}
	now := time.Now()
	content.CreatedAt = now
	content.UpdatedAt = now

	query := `
		INSERT INTO note_contents (
			id, note_id, content_type, content, storage_provider, storage_key,
			file_name, mime_type, file_size_bytes, checksum, thumbnail_base64,
			sort_order, metadata, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
	`

	_, err := r.pool.Exec(ctx, query,
		content.ID,
		content.NoteID,
		content.ContentType,
		content.Content,
		content.StorageProvider,
		content.StorageKey,
		content.FileName,
		content.MimeType,
		content.FileSizeBytes,
		content.Checksum,
		content.ThumbnailBase64,
		content.SortOrder,
		content.Metadata,
		content.CreatedAt,
		content.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create content: %w", err)
	}

	return nil
}

// UpdateContent updates an existing note content
func (r *PostgresRepository) UpdateContent(ctx context.Context, content *note.NoteContent) error {
	content.UpdatedAt = time.Now()

	query := `
		UPDATE note_contents
		SET content = $2, thumbnail_base64 = $3, sort_order = $4, metadata = $5, updated_at = $6
		WHERE id = $1
	`

	result, err := r.pool.Exec(ctx, query,
		content.ID,
		content.Content,
		content.ThumbnailBase64,
		content.SortOrder,
		content.Metadata,
		content.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to update content: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("content not found")
	}

	return nil
}

// DeleteContent deletes a note content
func (r *PostgresRepository) DeleteContent(ctx context.Context, contentID string) error {
	query := `DELETE FROM note_contents WHERE id = $1`
	result, err := r.pool.Exec(ctx, query, contentID)
	if err != nil {
		return fmt.Errorf("failed to delete content: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("content not found")
	}

	return nil
}

// ReorderContents updates the sort order of contents
func (r *PostgresRepository) ReorderContents(ctx context.Context, noteID string, contentIDs []string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	now := time.Now()
	for i, contentID := range contentIDs {
		_, err := tx.Exec(ctx, `
			UPDATE note_contents
			SET sort_order = $1, updated_at = $2
			WHERE id = $3 AND note_id = $4
		`, i, now, contentID, noteID)
		if err != nil {
			return fmt.Errorf("failed to update sort order: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// UpdateIndexStatus updates the index status of a note
func (r *PostgresRepository) UpdateIndexStatus(ctx context.Context, noteID string, status note.IndexStatus) error {
	var indexedAt *time.Time
	if status == note.IndexStatusIndexed {
		now := time.Now()
		indexedAt = &now
	}

	query := `
		UPDATE notes
		SET index_status = $2, indexed_at = $3, updated_at = NOW()
		WHERE id = $1
	`

	result, err := r.pool.Exec(ctx, query, noteID, status, indexedAt)
	if err != nil {
		return fmt.Errorf("failed to update index status: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrNoteNotFound
	}

	return nil
}

// scanNoteContent scans a row into a NoteContent struct
func (r *PostgresRepository) scanNoteContent(row pgx.Row) (*note.NoteContent, error) {
	var content note.NoteContent
	var storageProvider *string

	err := row.Scan(
		&content.ID,
		&content.NoteID,
		&content.ContentType,
		&content.Content,
		&storageProvider,
		&content.StorageKey,
		&content.FileName,
		&content.MimeType,
		&content.FileSizeBytes,
		&content.Checksum,
		&content.ThumbnailBase64,
		&content.SortOrder,
		&content.Metadata,
		&content.CreatedAt,
		&content.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to scan content: %w", err)
	}

	if storageProvider != nil {
		sp := note.StorageProvider(*storageProvider)
		content.StorageProvider = &sp
	}

	return &content, nil
}

// ========== NoteMetadata Operations ==========

// ListMetadata retrieves all metadata for a note
func (r *PostgresRepository) ListMetadata(ctx context.Context, noteID string) ([]*note.NoteMetadataItem, error) {
	query := `
		SELECT id, note_id, key, value, created_at, updated_at
		FROM note_metadata
		WHERE note_id = $1
		ORDER BY key
	`

	rows, err := r.pool.Query(ctx, query, noteID)
	if err != nil {
		return nil, fmt.Errorf("failed to list metadata: %w", err)
	}
	defer rows.Close()

	var items []*note.NoteMetadataItem
	for rows.Next() {
		item, err := r.scanNoteMetadataItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating metadata: %w", err)
	}

	return items, nil
}

// SetMetadata sets a metadata key-value pair (upsert)
func (r *PostgresRepository) SetMetadata(ctx context.Context, noteID, key string, value *string) error {
	query := `
		INSERT INTO note_metadata (note_id, key, value, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), NOW())
		ON CONFLICT (note_id, key) DO UPDATE SET value = $3, updated_at = NOW()
	`

	_, err := r.pool.Exec(ctx, query, noteID, key, value)
	if err != nil {
		return fmt.Errorf("failed to set metadata: %w", err)
	}

	return nil
}

// DeleteMetadata deletes a metadata key
func (r *PostgresRepository) DeleteMetadata(ctx context.Context, noteID, key string) error {
	query := `DELETE FROM note_metadata WHERE note_id = $1 AND key = $2`
	_, err := r.pool.Exec(ctx, query, noteID, key)
	if err != nil {
		return fmt.Errorf("failed to delete metadata: %w", err)
	}

	return nil
}

// BulkSetMetadata replaces all metadata for a note
func (r *PostgresRepository) BulkSetMetadata(ctx context.Context, noteID string, metadata []note.SetNoteMetadataInput) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Delete all existing metadata for the note
	_, err = tx.Exec(ctx, `DELETE FROM note_metadata WHERE note_id = $1`, noteID)
	if err != nil {
		return fmt.Errorf("failed to delete existing metadata: %w", err)
	}

	// Insert new metadata
	for _, m := range metadata {
		_, err := tx.Exec(ctx, `
			INSERT INTO note_metadata (note_id, key, value, created_at, updated_at)
			VALUES ($1, $2, $3, NOW(), NOW())
		`, noteID, m.Key, m.Value)
		if err != nil {
			return fmt.Errorf("failed to insert metadata: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// scanNoteMetadataItem scans a row into a NoteMetadataItem struct
func (r *PostgresRepository) scanNoteMetadataItem(row pgx.Row) (*note.NoteMetadataItem, error) {
	var item note.NoteMetadataItem

	err := row.Scan(
		&item.ID,
		&item.NoteID,
		&item.Key,
		&item.Value,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to scan metadata item: %w", err)
	}

	return &item, nil
}
