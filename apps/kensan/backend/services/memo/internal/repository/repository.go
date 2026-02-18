package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	memo "github.com/kensan/backend/services/memo/internal"
	sharedErrors "github.com/kensan/backend/shared/errors"
	"github.com/kensan/backend/shared/sqlbuilder"
)

// PostgresRepository handles database operations for memos
type PostgresRepository struct {
	pool *pgxpool.Pool
}

// Ensure PostgresRepository implements Repository interface
var _ Repository = (*PostgresRepository)(nil)

// NewPostgresRepository creates a new memo repository
func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

// List returns memos for a user with optional filters
func (r *PostgresRepository) List(ctx context.Context, userID string, filter memo.MemoFilter) ([]memo.Memo, error) {
	w := sqlbuilder.NewWhereBuilder(userID)

	// Filter by archived status
	if !filter.IncludeAll {
		if filter.Archived != nil {
			sqlbuilder.AddFilter(w, "archived", filter.Archived)
		} else {
			// Default: only show non-archived
			w.AddCondition("archived = $%d", false)
		}
	}

	// Filter by date
	if filter.Date != nil {
		w.AddCondition("DATE(created_at) = $%d", *filter.Date)
	}

	limitClause := ""
	if filter.Limit > 0 {
		limitClause = " " + w.AddLimit(filter.Limit)
	}

	query := fmt.Sprintf(`
		SELECT id, user_id, content, archived, created_at, updated_at
		FROM memos
		%s ORDER BY created_at DESC%s
	`, w.WhereClause(), limitClause)

	rows, err := r.pool.Query(ctx, query, w.Args()...)
	if err != nil {
		return nil, sharedErrors.WrapDBError("failed to query memos", err)
	}
	defer rows.Close()

	var memos []memo.Memo
	for rows.Next() {
		var m memo.Memo
		err := rows.Scan(
			&m.ID,
			&m.UserID,
			&m.Content,
			&m.Archived,
			&m.CreatedAt,
			&m.UpdatedAt,
		)
		if err != nil {
			return nil, sharedErrors.WrapDBError("failed to scan memo", err)
		}
		memos = append(memos, m)
	}

	if err := rows.Err(); err != nil {
		return nil, sharedErrors.WrapDBError("error iterating memos", err)
	}

	return memos, nil
}

// GetByID returns a memo by ID
func (r *PostgresRepository) GetByID(ctx context.Context, userID, memoID string) (*memo.Memo, error) {
	query := `
		SELECT id, user_id, content, archived, created_at, updated_at
		FROM memos
		WHERE id = $1 AND user_id = $2
	`

	var m memo.Memo
	err := r.pool.QueryRow(ctx, query, memoID, userID).Scan(
		&m.ID,
		&m.UserID,
		&m.Content,
		&m.Archived,
		&m.CreatedAt,
		&m.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, sharedErrors.WrapDBError("failed to get memo", err)
	}

	return &m, nil
}

// Create creates a new memo
func (r *PostgresRepository) Create(ctx context.Context, userID string, input memo.CreateMemoInput) (*memo.Memo, error) {
	id := uuid.New().String()
	now := time.Now()

	query := `
		INSERT INTO memos (id, user_id, content, archived, created_at, updated_at)
		VALUES ($1, $2, $3, false, $4, $5)
		RETURNING id, user_id, content, archived, created_at, updated_at
	`

	var m memo.Memo
	err := r.pool.QueryRow(ctx, query, id, userID, input.Content, now, now).Scan(
		&m.ID,
		&m.UserID,
		&m.Content,
		&m.Archived,
		&m.CreatedAt,
		&m.UpdatedAt,
	)
	if err != nil {
		return nil, sharedErrors.WrapDBError("failed to create memo", err)
	}

	return &m, nil
}

// Update updates an existing memo
func (r *PostgresRepository) Update(ctx context.Context, userID, memoID string, input memo.UpdateMemoInput) (*memo.Memo, error) {
	b := sqlbuilder.NewUpdateBuilder()
	sqlbuilder.AddField(b, "content", input.Content)
	sqlbuilder.AddField(b, "archived", input.Archived)

	if !b.HasUpdates() {
		return r.GetByID(ctx, userID, memoID)
	}

	b.AddTimestamp()
	idArg := b.AddArg(memoID)
	userArg := b.AddArg(userID)

	query := fmt.Sprintf(`
		UPDATE memos
		SET %s
		WHERE id = $%d AND user_id = $%d
		RETURNING id, user_id, content, archived, created_at, updated_at
	`, b.SetClause(), idArg, userArg)

	var m memo.Memo
	err := r.pool.QueryRow(ctx, query, b.Args()...).Scan(
		&m.ID,
		&m.UserID,
		&m.Content,
		&m.Archived,
		&m.CreatedAt,
		&m.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, sharedErrors.WrapDBError("failed to update memo", err)
	}

	return &m, nil
}

// Delete deletes a memo
func (r *PostgresRepository) Delete(ctx context.Context, userID, memoID string) error {
	query := `DELETE FROM memos WHERE id = $1 AND user_id = $2`
	result, err := r.pool.Exec(ctx, query, memoID, userID)
	if err != nil {
		return sharedErrors.WrapDBError("failed to delete memo", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("memo not found")
	}

	return nil
}

