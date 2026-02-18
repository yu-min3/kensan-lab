// Package sqlbuilder provides utilities for building dynamic SQL queries
// with parameterized placeholders ($1, $2, ...) for PostgreSQL.
package sqlbuilder

import (
	"fmt"
	"strings"
	"time"
)

// UpdateBuilder builds dynamic UPDATE SET clauses with parameterized args.
type UpdateBuilder struct {
	setClauses []string
	args       []interface{}
	argCount   int
}

// NewUpdateBuilder creates a new UpdateBuilder.
func NewUpdateBuilder() *UpdateBuilder {
	return &UpdateBuilder{
		setClauses: []string{},
		args:       []interface{}{},
		argCount:   0,
	}
}

// AddField adds a field to the SET clause if the pointer is not nil.
// Uses generics to support any pointer type.
func AddField[T any](b *UpdateBuilder, fieldName string, ptr *T) {
	if ptr != nil {
		b.argCount++
		b.setClauses = append(b.setClauses, fmt.Sprintf("%s = $%d", fieldName, b.argCount))
		b.args = append(b.args, *ptr)
	}
}

// AddFieldValue adds a field with a direct value (not pointer) to the SET clause.
func (b *UpdateBuilder) AddFieldValue(fieldName string, value interface{}) {
	b.argCount++
	b.setClauses = append(b.setClauses, fmt.Sprintf("%s = $%d", fieldName, b.argCount))
	b.args = append(b.args, value)
}

// AddTimestamp adds an updated_at timestamp to the SET clause.
func (b *UpdateBuilder) AddTimestamp() {
	b.AddFieldValue("updated_at", time.Now())
}

// AddArg adds an argument without a SET clause (for WHERE conditions).
// Returns the placeholder number.
func (b *UpdateBuilder) AddArg(value interface{}) int {
	b.argCount++
	b.args = append(b.args, value)
	return b.argCount
}

// HasUpdates returns true if any fields were added.
func (b *UpdateBuilder) HasUpdates() bool {
	return len(b.setClauses) > 0
}

// SetClause returns the SET clause string (e.g., "name = $1, color = $2").
func (b *UpdateBuilder) SetClause() string {
	return strings.Join(b.setClauses, ", ")
}

// Args returns all arguments in order.
func (b *UpdateBuilder) Args() []interface{} {
	return b.args
}

// BuildUpdate builds a complete UPDATE query with WHERE id = ? AND user_id = ?.
// Returns the query string, args, and whether there are updates.
func (b *UpdateBuilder) BuildUpdate(table, returning string) (query string, args []interface{}, hasUpdates bool) {
	if !b.HasUpdates() {
		return "", nil, false
	}

	b.AddTimestamp()
	idArg := b.AddArg(nil)    // placeholder, caller sets via args
	userArg := b.AddArg(nil)  // placeholder, caller sets via args

	query = fmt.Sprintf(
		`UPDATE %s SET %s WHERE id = $%d AND user_id = $%d`,
		table, b.SetClause(), idArg, userArg,
	)
	if returning != "" {
		query += " RETURNING " + returning
	}

	return query, b.args, true
}

// WhereBuilder builds dynamic WHERE clauses with parameterized args.
type WhereBuilder struct {
	conditions []string
	args       []interface{}
	argCount   int
}

// NewWhereBuilder creates a new WhereBuilder with the initial user_id condition.
func NewWhereBuilder(userID string) *WhereBuilder {
	return &WhereBuilder{
		conditions: []string{"user_id = $1"},
		args:       []interface{}{userID},
		argCount:   1,
	}
}

// AddFilter adds a WHERE condition if the pointer is not nil.
func AddFilter[T any](w *WhereBuilder, column string, ptr *T) {
	if ptr != nil {
		w.argCount++
		w.conditions = append(w.conditions, fmt.Sprintf("%s = $%d", column, w.argCount))
		w.args = append(w.args, *ptr)
	}
}

// AddFilterWithCast adds a WHERE condition with a type cast (e.g., "::timestamptz").
func AddFilterWithCast[T any](w *WhereBuilder, column, op, cast string, ptr *T) {
	if ptr != nil {
		w.argCount++
		w.conditions = append(w.conditions, fmt.Sprintf("%s %s $%d%s", column, op, w.argCount, cast))
		w.args = append(w.args, *ptr)
	}
}

// AddCondition adds a raw condition string with one argument.
func (w *WhereBuilder) AddCondition(condition string, value interface{}) {
	w.argCount++
	w.conditions = append(w.conditions, fmt.Sprintf(condition, w.argCount))
	w.args = append(w.args, value)
}

// AddInClause adds an IN clause for a slice of values.
func (w *WhereBuilder) AddInClause(column string, values []string) {
	if len(values) == 0 {
		return
	}
	placeholders := make([]string, len(values))
	for i, v := range values {
		w.argCount++
		placeholders[i] = fmt.Sprintf("$%d", w.argCount)
		w.args = append(w.args, v)
	}
	w.conditions = append(w.conditions, fmt.Sprintf("%s IN (%s)", column, strings.Join(placeholders, ", ")))
}

// AddLike adds a LIKE condition with a search pattern.
func (w *WhereBuilder) AddLike(columns []string, pattern string) {
	if pattern == "" {
		return
	}
	w.argCount++
	likes := make([]string, len(columns))
	for i, col := range columns {
		likes[i] = fmt.Sprintf("LOWER(%s) LIKE $%d", col, w.argCount)
	}
	w.conditions = append(w.conditions, "("+strings.Join(likes, " OR ")+")")
	w.args = append(w.args, "%"+strings.ToLower(pattern)+"%")
}

// AddExists adds an EXISTS subquery condition.
func (w *WhereBuilder) AddExists(subquery string, value interface{}) {
	w.argCount++
	w.conditions = append(w.conditions, fmt.Sprintf("EXISTS ("+subquery+")", w.argCount))
	w.args = append(w.args, value)
}

// AddLimit adds a LIMIT argument and returns the placeholder string.
func (w *WhereBuilder) AddLimit(limit int) string {
	w.argCount++
	w.args = append(w.args, limit)
	return fmt.Sprintf("LIMIT $%d", w.argCount)
}

// WhereClause returns the full WHERE clause string.
func (w *WhereBuilder) WhereClause() string {
	return "WHERE " + strings.Join(w.conditions, " AND ")
}

// Args returns all arguments in order.
func (w *WhereBuilder) Args() []interface{} {
	return w.args
}

// ArgCount returns the current argument count.
func (w *WhereBuilder) ArgCount() int {
	return w.argCount
}
