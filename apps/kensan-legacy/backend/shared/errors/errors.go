// Package errors provides common error types and utilities for all services.
package errors

import (
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
)

// Common base errors that can be used with errors.Is()
var (
	// ErrNotFound is the base error for "not found" scenarios
	ErrNotFound = errors.New("not found")

	// ErrInvalidInput is the base error for invalid input scenarios
	ErrInvalidInput = errors.New("invalid input")

	// ErrUnauthorized is the base error for unauthorized access
	ErrUnauthorized = errors.New("unauthorized")

	// ErrAlreadyExists is the base error for duplicate entries
	ErrAlreadyExists = errors.New("already exists")

	// ErrRequired is the base error for missing required fields
	ErrRequired = errors.New("required")

	// ErrInvalidFormat is the base error for format validation failures
	ErrInvalidFormat = errors.New("invalid format")

	// ErrDatabaseSchema is the base error for database schema mismatches
	ErrDatabaseSchema = errors.New("database schema error")

	// ErrDatabaseConnection is the base error for database connection failures
	ErrDatabaseConnection = errors.New("database connection error")
)

// EntityError wraps a base error with an entity name for context
type EntityError struct {
	Entity string
	Base   error
}

func (e *EntityError) Error() string {
	return fmt.Sprintf("%s %s", e.Entity, e.Base.Error())
}

func (e *EntityError) Unwrap() error {
	return e.Base
}

// NotFound creates a "not found" error for a specific entity
func NotFound(entity string) error {
	return &EntityError{Entity: entity, Base: ErrNotFound}
}

// AlreadyExists creates an "already exists" error for a specific entity
func AlreadyExists(entity string) error {
	return &EntityError{Entity: entity, Base: ErrAlreadyExists}
}

// FieldError wraps a base error with a field name for validation context
type FieldError struct {
	Field string
	Base  error
}

func (e *FieldError) Error() string {
	return fmt.Sprintf("%s %s", e.Field, e.Base.Error())
}

func (e *FieldError) Unwrap() error {
	return e.Base
}

// Required creates a "required" error for a specific field
func Required(field string) error {
	return &FieldError{Field: field, Base: ErrRequired}
}

// InvalidFormat creates an "invalid format" error with a custom message
func InvalidFormat(field, expected string) error {
	return fmt.Errorf("%s: %w (expected %s)", field, ErrInvalidFormat, expected)
}

// Validation errors for common format patterns
var (
	ErrInvalidDate = InvalidFormat("date", "YYYY-MM-DD")
	ErrInvalidTime = InvalidFormat("time", "HH:mm")
)

// IsNotFound checks if the error is a "not found" error
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}

// IsInvalidInput checks if the error is an "invalid input" error
func IsInvalidInput(err error) bool {
	return errors.Is(err, ErrInvalidInput)
}

// IsUnauthorized checks if the error is an "unauthorized" error
func IsUnauthorized(err error) bool {
	return errors.Is(err, ErrUnauthorized)
}

// IsAlreadyExists checks if the error is an "already exists" error
func IsAlreadyExists(err error) bool {
	return errors.Is(err, ErrAlreadyExists)
}

// IsRequired checks if the error is a "required" error
func IsRequired(err error) bool {
	return errors.Is(err, ErrRequired)
}

// IsInvalidFormat checks if the error is an "invalid format" error
func IsInvalidFormat(err error) bool {
	return errors.Is(err, ErrInvalidFormat)
}

// ============================================================
// Validation error helper functions
// ============================================================

// InvalidStatus returns an invalid status error for a specific entity
func InvalidStatus(entity string) error {
	return fmt.Errorf("invalid %s status: %w", entity, ErrInvalidInput)
}

// ============================================================
// PostgreSQL error helpers
// ============================================================

// PostgreSQL error codes
const (
	// PgUniqueViolation is the PostgreSQL error code for unique constraint violations
	PgUniqueViolation = "23505"
	// PgForeignKeyViolation is the PostgreSQL error code for foreign key violations
	PgForeignKeyViolation = "23503"
	// PgNotNullViolation is the PostgreSQL error code for not-null violations
	PgNotNullViolation = "23502"
	// PgCheckViolation is the PostgreSQL error code for check constraint violations
	PgCheckViolation = "23514"
	// PgUndefinedColumn is the PostgreSQL error code for undefined column
	PgUndefinedColumn = "42703"
	// PgUndefinedTable is the PostgreSQL error code for undefined table
	PgUndefinedTable = "42P01"
	// PgUndefinedFunction is the PostgreSQL error code for undefined function
	PgUndefinedFunction = "42883"
)

// IsUniqueViolation checks if the error is a PostgreSQL unique constraint violation
func IsUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == PgUniqueViolation
	}
	return false
}

// IsForeignKeyViolation checks if the error is a PostgreSQL foreign key violation
func IsForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == PgForeignKeyViolation
	}
	return false
}

// GetPgErrorCode returns the PostgreSQL error code if the error is a PgError, empty string otherwise
func GetPgErrorCode(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}

// GetPgErrorConstraint returns the constraint name if the error is a PgError, empty string otherwise
func GetPgErrorConstraint(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.ConstraintName
	}
	return ""
}

// IsDatabaseSchemaError checks if the error is a schema-related PostgreSQL error
func IsDatabaseSchemaError(err error) bool {
	code := GetPgErrorCode(err)
	return code == PgUndefinedColumn || code == PgUndefinedTable || code == PgUndefinedFunction
}

// WrapDatabaseError wraps a database error with appropriate type for better error handling.
// Schema errors (missing columns/tables) are wrapped with ErrDatabaseSchema.
// Other errors are returned as-is.
func WrapDatabaseError(err error) error {
	if err == nil {
		return nil
	}
	if IsDatabaseSchemaError(err) {
		return fmt.Errorf("%w: %v", ErrDatabaseSchema, err)
	}
	return err
}

// IsDatabaseSchema checks if the error is a database schema error
func IsDatabaseSchema(err error) bool {
	return errors.Is(err, ErrDatabaseSchema)
}

// WrapDBError wraps a database error with a context message and detects schema errors.
// Use this in repositories to add context to database errors.
func WrapDBError(msg string, err error) error {
	return fmt.Errorf("%s: %w", msg, WrapDatabaseError(err))
}
