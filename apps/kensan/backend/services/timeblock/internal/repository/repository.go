package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kensan/backend/services/timeblock/internal"
	sharedErrors "github.com/kensan/backend/shared/errors"
	"github.com/kensan/backend/shared/sqlbuilder"
)

// Repository-level errors
var (
	ErrTimerAlreadyRunning = errors.New("a timer is already running")
)

// PostgresRepository handles database operations for time blocks and time entries
type PostgresRepository struct {
	pool *pgxpool.Pool
}

// Ensure PostgresRepository implements Repository interface
var _ Repository = (*PostgresRepository)(nil)

// NewPostgresRepository creates a new timeblock repository
func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

// ========== TimeBlock Operations ==========

// ListTimeBlocks returns all time blocks for a user with optional filters
func (r *PostgresRepository) ListTimeBlocks(ctx context.Context, userID string, filter timeblock.TimeBlockFilter) ([]timeblock.TimeBlock, error) {
	w := sqlbuilder.NewWhereBuilder(userID)
	sqlbuilder.AddFilterWithCast(w, "start_datetime", ">=", "::timestamptz", filter.StartDatetime)
	sqlbuilder.AddFilterWithCast(w, "start_datetime", "<", "::timestamptz", filter.EndDatetime)
	sqlbuilder.AddFilter(w, "goal_id", filter.GoalID)
	sqlbuilder.AddFilter(w, "milestone_id", filter.MilestoneID)

	query := fmt.Sprintf(`
		SELECT id, user_id, start_datetime, end_datetime, task_id, task_name,
		       milestone_id, milestone_name, goal_id, goal_name, goal_color,
		       tag_ids, created_at, updated_at
		FROM time_blocks
		%s ORDER BY start_datetime ASC
	`, w.WhereClause())

	rows, err := r.pool.Query(ctx, query, w.Args()...)
	if err != nil {
		return nil, fmt.Errorf("failed to query time blocks: %w", err)
	}
	defer rows.Close()

	var blocks []timeblock.TimeBlock
	for rows.Next() {
		var tb timeblock.TimeBlock
		var tagIDs []string
		err := rows.Scan(
			&tb.ID,
			&tb.UserID,
			&tb.StartDatetime,
			&tb.EndDatetime,
			&tb.TaskID,
			&tb.TaskName,
			&tb.MilestoneID,
			&tb.MilestoneName,
			&tb.GoalID,
			&tb.GoalName,
			&tb.GoalColor,
			&tagIDs,
			&tb.CreatedAt,
			&tb.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan time block: %w", err)
		}
		tb.TagIDs = tagIDs
		blocks = append(blocks, tb)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating time blocks: %w", err)
	}

	return blocks, nil
}

// GetTimeBlockByID returns a time block by ID for a specific user
func (r *PostgresRepository) GetTimeBlockByID(ctx context.Context, userID, timeBlockID string) (*timeblock.TimeBlock, error) {
	query := `
		SELECT id, user_id, start_datetime, end_datetime, task_id, task_name,
		       milestone_id, milestone_name, goal_id, goal_name, goal_color,
		       tag_ids, created_at, updated_at
		FROM time_blocks
		WHERE id = $1 AND user_id = $2
	`

	var tb timeblock.TimeBlock
	var tagIDs []string
	err := r.pool.QueryRow(ctx, query, timeBlockID, userID).Scan(
		&tb.ID,
		&tb.UserID,
		&tb.StartDatetime,
		&tb.EndDatetime,
		&tb.TaskID,
		&tb.TaskName,
		&tb.MilestoneID,
		&tb.MilestoneName,
		&tb.GoalID,
		&tb.GoalName,
		&tb.GoalColor,
		&tagIDs,
		&tb.CreatedAt,
		&tb.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get time block: %w", err)
	}

	tb.TagIDs = tagIDs

	return &tb, nil
}

// CreateTimeBlock creates a new time block
func (r *PostgresRepository) CreateTimeBlock(ctx context.Context, userID string, input timeblock.CreateTimeBlockInput) (*timeblock.TimeBlock, error) {
	id := uuid.New().String()
	now := time.Now()

	tagIDs := input.TagIDs
	if tagIDs == nil {
		tagIDs = []string{}
	}

	// Parse datetime strings
	startDt, err := time.Parse(time.RFC3339, input.StartDatetime)
	if err != nil {
		return nil, fmt.Errorf("invalid start_datetime: %w", err)
	}
	endDt, err := time.Parse(time.RFC3339, input.EndDatetime)
	if err != nil {
		return nil, fmt.Errorf("invalid end_datetime: %w", err)
	}

	query := `
		INSERT INTO time_blocks (id, user_id, start_datetime, end_datetime, task_id, task_name,
		                         milestone_id, milestone_name,
		                         goal_id, goal_name, goal_color, tag_ids,
		                         created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		RETURNING id, user_id, start_datetime, end_datetime, task_id, task_name,
		          milestone_id, milestone_name, goal_id, goal_name, goal_color,
		          tag_ids, created_at, updated_at
	`

	var tb timeblock.TimeBlock
	var returnedTagIDs []string
	err = r.pool.QueryRow(ctx, query,
		id,
		userID,
		startDt,
		endDt,
		input.TaskID,
		input.TaskName,
		input.MilestoneID,
		input.MilestoneName,
		input.GoalID,
		input.GoalName,
		input.GoalColor,
		tagIDs,
		now,
		now,
	).Scan(
		&tb.ID,
		&tb.UserID,
		&tb.StartDatetime,
		&tb.EndDatetime,
		&tb.TaskID,
		&tb.TaskName,
		&tb.MilestoneID,
		&tb.MilestoneName,
		&tb.GoalID,
		&tb.GoalName,
		&tb.GoalColor,
		&returnedTagIDs,
		&tb.CreatedAt,
		&tb.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create time block: %w", err)
	}

	tb.TagIDs = returnedTagIDs

	return &tb, nil
}

// UpdateTimeBlock updates an existing time block
func (r *PostgresRepository) UpdateTimeBlock(ctx context.Context, userID, timeBlockID string, input timeblock.UpdateTimeBlockInput) (*timeblock.TimeBlock, error) {
	b := sqlbuilder.NewUpdateBuilder()

	// For datetime fields, parse string to time.Time before adding
	if input.StartDatetime != nil {
		startDt, err := time.Parse(time.RFC3339, *input.StartDatetime)
		if err != nil {
			return nil, fmt.Errorf("invalid start_datetime: %w", err)
		}
		b.AddFieldValue("start_datetime", startDt)
	}
	if input.EndDatetime != nil {
		endDt, err := time.Parse(time.RFC3339, *input.EndDatetime)
		if err != nil {
			return nil, fmt.Errorf("invalid end_datetime: %w", err)
		}
		b.AddFieldValue("end_datetime", endDt)
	}
	sqlbuilder.AddField(b, "task_id", input.TaskID)
	sqlbuilder.AddField(b, "task_name", input.TaskName)
	sqlbuilder.AddField(b, "milestone_id", input.MilestoneID)
	sqlbuilder.AddField(b, "milestone_name", input.MilestoneName)
	sqlbuilder.AddField(b, "goal_id", input.GoalID)
	sqlbuilder.AddField(b, "goal_name", input.GoalName)
	sqlbuilder.AddField(b, "goal_color", input.GoalColor)
	if input.TagIDs != nil {
		b.AddFieldValue("tag_ids", input.TagIDs)
	}

	if !b.HasUpdates() {
		return r.GetTimeBlockByID(ctx, userID, timeBlockID)
	}

	b.AddFieldValue("updated_at", time.Now())
	idArg := b.AddArg(timeBlockID)
	userArg := b.AddArg(userID)

	query := fmt.Sprintf(`
		UPDATE time_blocks
		SET %s
		WHERE id = $%d AND user_id = $%d
		RETURNING id, user_id, start_datetime, end_datetime, task_id, task_name,
		          milestone_id, milestone_name, goal_id, goal_name, goal_color,
		          tag_ids, created_at, updated_at
	`, b.SetClause(), idArg, userArg)

	var tb timeblock.TimeBlock
	var tagIDs []string
	err := r.pool.QueryRow(ctx, query, b.Args()...).Scan(
		&tb.ID,
		&tb.UserID,
		&tb.StartDatetime,
		&tb.EndDatetime,
		&tb.TaskID,
		&tb.TaskName,
		&tb.MilestoneID,
		&tb.MilestoneName,
		&tb.GoalID,
		&tb.GoalName,
		&tb.GoalColor,
		&tagIDs,
		&tb.CreatedAt,
		&tb.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to update time block: %w", err)
	}

	tb.TagIDs = tagIDs

	return &tb, nil
}

// DeleteTimeBlock deletes a time block
func (r *PostgresRepository) DeleteTimeBlock(ctx context.Context, userID, timeBlockID string) error {
	query := `DELETE FROM time_blocks WHERE id = $1 AND user_id = $2`
	result, err := r.pool.Exec(ctx, query, timeBlockID, userID)
	if err != nil {
		return fmt.Errorf("failed to delete time block: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("time block not found")
	}

	return nil
}

// CreateTimeBlockBatch creates multiple time blocks in a batch
func (r *PostgresRepository) CreateTimeBlockBatch(ctx context.Context, userID string, inputs []timeblock.CreateTimeBlockInput) ([]timeblock.TimeBlock, error) {
	if len(inputs) == 0 {
		return []timeblock.TimeBlock{}, nil
	}

	var blocks []timeblock.TimeBlock
	for _, input := range inputs {
		tb, err := r.CreateTimeBlock(ctx, userID, input)
		if err != nil {
			return nil, err
		}
		blocks = append(blocks, *tb)
	}

	return blocks, nil
}

// ========== TimeEntry Operations ==========

// ListTimeEntries returns all time entries for a user with optional filters
func (r *PostgresRepository) ListTimeEntries(ctx context.Context, userID string, filter timeblock.TimeEntryFilter) ([]timeblock.TimeEntry, error) {
	w := sqlbuilder.NewWhereBuilder(userID)
	sqlbuilder.AddFilterWithCast(w, "start_datetime", ">=", "::timestamptz", filter.StartDatetime)
	sqlbuilder.AddFilterWithCast(w, "start_datetime", "<", "::timestamptz", filter.EndDatetime)
	sqlbuilder.AddFilter(w, "goal_id", filter.GoalID)
	sqlbuilder.AddFilter(w, "milestone_id", filter.MilestoneID)

	query := fmt.Sprintf(`
		SELECT id, user_id, start_datetime, end_datetime, task_id, task_name,
		       milestone_id, milestone_name, goal_id, goal_name, goal_color,
		       tag_ids, description, created_at, updated_at
		FROM time_entries
		%s ORDER BY start_datetime ASC
	`, w.WhereClause())

	rows, err := r.pool.Query(ctx, query, w.Args()...)
	if err != nil {
		return nil, fmt.Errorf("failed to query time entries: %w", err)
	}
	defer rows.Close()

	var entries []timeblock.TimeEntry
	for rows.Next() {
		var te timeblock.TimeEntry
		var tagIDs []string
		err := rows.Scan(
			&te.ID,
			&te.UserID,
			&te.StartDatetime,
			&te.EndDatetime,
			&te.TaskID,
			&te.TaskName,
			&te.MilestoneID,
			&te.MilestoneName,
			&te.GoalID,
			&te.GoalName,
			&te.GoalColor,
			&tagIDs,
			&te.Description,
			&te.CreatedAt,
			&te.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan time entry: %w", err)
		}
		te.TagIDs = tagIDs
		entries = append(entries, te)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating time entries: %w", err)
	}

	return entries, nil
}

// GetTimeEntryByID returns a time entry by ID for a specific user
func (r *PostgresRepository) GetTimeEntryByID(ctx context.Context, userID, timeEntryID string) (*timeblock.TimeEntry, error) {
	query := `
		SELECT id, user_id, start_datetime, end_datetime, task_id, task_name,
		       milestone_id, milestone_name, goal_id, goal_name, goal_color,
		       tag_ids, description, created_at, updated_at
		FROM time_entries
		WHERE id = $1 AND user_id = $2
	`

	var te timeblock.TimeEntry
	var tagIDs []string
	err := r.pool.QueryRow(ctx, query, timeEntryID, userID).Scan(
		&te.ID,
		&te.UserID,
		&te.StartDatetime,
		&te.EndDatetime,
		&te.TaskID,
		&te.TaskName,
		&te.MilestoneID,
		&te.MilestoneName,
		&te.GoalID,
		&te.GoalName,
		&te.GoalColor,
		&tagIDs,
		&te.Description,
		&te.CreatedAt,
		&te.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get time entry: %w", err)
	}

	te.TagIDs = tagIDs

	return &te, nil
}

// CreateTimeEntry creates a new time entry
func (r *PostgresRepository) CreateTimeEntry(ctx context.Context, userID string, input timeblock.CreateTimeEntryInput) (*timeblock.TimeEntry, error) {
	id := uuid.New().String()
	now := time.Now()

	tagIDs := input.TagIDs
	if tagIDs == nil {
		tagIDs = []string{}
	}

	// Parse datetime strings
	startDt, err := time.Parse(time.RFC3339, input.StartDatetime)
	if err != nil {
		return nil, fmt.Errorf("invalid start_datetime: %w", err)
	}
	endDt, err := time.Parse(time.RFC3339, input.EndDatetime)
	if err != nil {
		return nil, fmt.Errorf("invalid end_datetime: %w", err)
	}

	query := `
		INSERT INTO time_entries (id, user_id, start_datetime, end_datetime, task_id, task_name,
		                          milestone_id, milestone_name,
		                          goal_id, goal_name, goal_color, tag_ids, description, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		RETURNING id, user_id, start_datetime, end_datetime, task_id, task_name,
		          milestone_id, milestone_name, goal_id, goal_name, goal_color,
		          tag_ids, description, created_at, updated_at
	`

	var te timeblock.TimeEntry
	var returnedTagIDs []string
	err = r.pool.QueryRow(ctx, query,
		id,
		userID,
		startDt,
		endDt,
		input.TaskID,
		input.TaskName,
		input.MilestoneID,
		input.MilestoneName,
		input.GoalID,
		input.GoalName,
		input.GoalColor,
		tagIDs,
		input.Description,
		now,
		now,
	).Scan(
		&te.ID,
		&te.UserID,
		&te.StartDatetime,
		&te.EndDatetime,
		&te.TaskID,
		&te.TaskName,
		&te.MilestoneID,
		&te.MilestoneName,
		&te.GoalID,
		&te.GoalName,
		&te.GoalColor,
		&returnedTagIDs,
		&te.Description,
		&te.CreatedAt,
		&te.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create time entry: %w", err)
	}

	te.TagIDs = returnedTagIDs

	return &te, nil
}

// UpdateTimeEntry updates an existing time entry
func (r *PostgresRepository) UpdateTimeEntry(ctx context.Context, userID, timeEntryID string, input timeblock.UpdateTimeEntryInput) (*timeblock.TimeEntry, error) {
	b := sqlbuilder.NewUpdateBuilder()

	// For datetime fields, parse string to time.Time before adding
	if input.StartDatetime != nil {
		startDt, err := time.Parse(time.RFC3339, *input.StartDatetime)
		if err != nil {
			return nil, fmt.Errorf("invalid start_datetime: %w", err)
		}
		b.AddFieldValue("start_datetime", startDt)
	}
	if input.EndDatetime != nil {
		endDt, err := time.Parse(time.RFC3339, *input.EndDatetime)
		if err != nil {
			return nil, fmt.Errorf("invalid end_datetime: %w", err)
		}
		b.AddFieldValue("end_datetime", endDt)
	}
	sqlbuilder.AddField(b, "task_id", input.TaskID)
	sqlbuilder.AddField(b, "task_name", input.TaskName)
	sqlbuilder.AddField(b, "milestone_id", input.MilestoneID)
	sqlbuilder.AddField(b, "milestone_name", input.MilestoneName)
	sqlbuilder.AddField(b, "goal_id", input.GoalID)
	sqlbuilder.AddField(b, "goal_name", input.GoalName)
	sqlbuilder.AddField(b, "goal_color", input.GoalColor)
	if input.TagIDs != nil {
		b.AddFieldValue("tag_ids", input.TagIDs)
	}
	sqlbuilder.AddField(b, "description", input.Description)

	if !b.HasUpdates() {
		return r.GetTimeEntryByID(ctx, userID, timeEntryID)
	}

	b.AddFieldValue("updated_at", time.Now())
	idArg := b.AddArg(timeEntryID)
	userArg := b.AddArg(userID)

	query := fmt.Sprintf(`
		UPDATE time_entries
		SET %s
		WHERE id = $%d AND user_id = $%d
		RETURNING id, user_id, start_datetime, end_datetime, task_id, task_name,
		          milestone_id, milestone_name, goal_id, goal_name, goal_color,
		          tag_ids, description, created_at, updated_at
	`, b.SetClause(), idArg, userArg)

	var te timeblock.TimeEntry
	var tagIDs []string
	err := r.pool.QueryRow(ctx, query, b.Args()...).Scan(
		&te.ID,
		&te.UserID,
		&te.StartDatetime,
		&te.EndDatetime,
		&te.TaskID,
		&te.TaskName,
		&te.MilestoneID,
		&te.MilestoneName,
		&te.GoalID,
		&te.GoalName,
		&te.GoalColor,
		&tagIDs,
		&te.Description,
		&te.CreatedAt,
		&te.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to update time entry: %w", err)
	}

	te.TagIDs = tagIDs

	return &te, nil
}

// DeleteTimeEntry deletes a time entry
func (r *PostgresRepository) DeleteTimeEntry(ctx context.Context, userID, timeEntryID string) error {
	query := `DELETE FROM time_entries WHERE id = $1 AND user_id = $2`
	result, err := r.pool.Exec(ctx, query, timeEntryID, userID)
	if err != nil {
		return fmt.Errorf("failed to delete time entry: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("time entry not found")
	}

	return nil
}

// ========== Timer Operations ==========

// GetRunningTimer returns the current running timer for a user
func (r *PostgresRepository) GetRunningTimer(ctx context.Context, userID string) (*timeblock.RunningTimer, error) {
	query := `
		SELECT id, user_id, task_id, task_name,
		       milestone_id, milestone_name, goal_id, goal_name, goal_color,
		       tag_ids, started_at, created_at
		FROM running_timers
		WHERE user_id = $1
	`

	var rt timeblock.RunningTimer
	var tagIDs []string
	err := r.pool.QueryRow(ctx, query, userID).Scan(
		&rt.ID,
		&rt.UserID,
		&rt.TaskID,
		&rt.TaskName,
		&rt.MilestoneID,
		&rt.MilestoneName,
		&rt.GoalID,
		&rt.GoalName,
		&rt.GoalColor,
		&tagIDs,
		&rt.StartedAt,
		&rt.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get running timer: %w", err)
	}

	rt.TagIDs = tagIDs

	return &rt, nil
}

// StartTimer starts a new timer for a user
func (r *PostgresRepository) StartTimer(ctx context.Context, userID string, input timeblock.StartTimerInput) (*timeblock.RunningTimer, error) {
	id := uuid.New().String()
	now := time.Now()

	tagIDs := input.TagIDs
	if tagIDs == nil {
		tagIDs = []string{}
	}

	query := `
		INSERT INTO running_timers (id, user_id, task_id, task_name,
		                            milestone_id, milestone_name, goal_id, goal_name, goal_color,
		                            tag_ids, started_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id, user_id, task_id, task_name,
		          milestone_id, milestone_name, goal_id, goal_name, goal_color,
		          tag_ids, started_at, created_at
	`

	var rt timeblock.RunningTimer
	var returnedTagIDs []string
	err := r.pool.QueryRow(ctx, query,
		id,
		userID,
		input.TaskID,
		input.TaskName,
		input.MilestoneID,
		input.MilestoneName,
		input.GoalID,
		input.GoalName,
		input.GoalColor,
		tagIDs,
		now,
		now,
	).Scan(
		&rt.ID,
		&rt.UserID,
		&rt.TaskID,
		&rt.TaskName,
		&rt.MilestoneID,
		&rt.MilestoneName,
		&rt.GoalID,
		&rt.GoalName,
		&rt.GoalColor,
		&returnedTagIDs,
		&rt.StartedAt,
		&rt.CreatedAt,
	)
	if err != nil {
		if sharedErrors.IsUniqueViolation(err) {
			return nil, ErrTimerAlreadyRunning
		}
		return nil, fmt.Errorf("failed to start timer: %w", err)
	}

	rt.TagIDs = returnedTagIDs

	return &rt, nil
}

// StopTimer stops the current timer and creates a time entry
func (r *PostgresRepository) StopTimer(ctx context.Context, userID string) (*timeblock.TimeEntry, error) {
	// Get the running timer first
	timer, err := r.GetRunningTimer(ctx, userID)
	if err != nil {
		return nil, err
	}
	if timer == nil {
		return nil, nil
	}

	now := time.Now()

	// Create time entry from the running timer using start_datetime/end_datetime directly
	entryID := uuid.New().String()

	tagIDs := timer.TagIDs
	if tagIDs == nil {
		tagIDs = []string{}
	}

	// Insert time entry with timestamps directly
	insertQuery := `
		INSERT INTO time_entries (id, user_id, start_datetime, end_datetime, task_id, task_name,
		                          milestone_id, milestone_name,
		                          goal_id, goal_name, goal_color, tag_ids, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		RETURNING id, user_id, start_datetime, end_datetime, task_id, task_name,
		          milestone_id, milestone_name, goal_id, goal_name, goal_color,
		          tag_ids, description, created_at, updated_at
	`

	var te timeblock.TimeEntry
	var returnedTagIDs []string
	err = r.pool.QueryRow(ctx, insertQuery,
		entryID,
		userID,
		timer.StartedAt,
		now,
		timer.TaskID,
		timer.TaskName,
		timer.MilestoneID,
		timer.MilestoneName,
		timer.GoalID,
		timer.GoalName,
		timer.GoalColor,
		tagIDs,
		now,
		now,
	).Scan(
		&te.ID,
		&te.UserID,
		&te.StartDatetime,
		&te.EndDatetime,
		&te.TaskID,
		&te.TaskName,
		&te.MilestoneID,
		&te.MilestoneName,
		&te.GoalID,
		&te.GoalName,
		&te.GoalColor,
		&returnedTagIDs,
		&te.Description,
		&te.CreatedAt,
		&te.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create time entry from timer: %w", err)
	}

	te.TagIDs = returnedTagIDs

	// Delete the running timer
	deleteQuery := `DELETE FROM running_timers WHERE user_id = $1`
	_, err = r.pool.Exec(ctx, deleteQuery, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to delete running timer: %w", err)
	}

	return &te, nil
}
