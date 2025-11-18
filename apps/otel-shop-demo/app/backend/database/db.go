package database

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/lib/pq"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var db *sql.DB

// InitDB initializes the database connection
func InitDB() error {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgresql://postgres:password@postgresql:5432/shopdb?sslmode=disable"
	}

	var err error
	db, err = sql.Open("postgres", dbURL)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	log.Println("Database connection established")
	return nil
}

// GetDB returns the database instance
func GetDB() *sql.DB {
	return db
}

// QueryWithSpan executes a query and creates a span for observability
func QueryWithSpan(ctx context.Context, tracer trace.Tracer, query string, args ...interface{}) (*sql.Rows, error) {
	ctx, span := tracer.Start(ctx, "database.query",
		trace.WithAttributes(
			attribute.String("db.system", "postgresql"),
			attribute.String("db.statement", query),
		),
	)
	defer span.End()

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}

	return rows, nil
}

// QueryRowWithSpan executes a single-row query and creates a span
func QueryRowWithSpan(ctx context.Context, tracer trace.Tracer, query string, args ...interface{}) *sql.Row {
	ctx, span := tracer.Start(ctx, "database.query_row",
		trace.WithAttributes(
			attribute.String("db.system", "postgresql"),
			attribute.String("db.statement", query),
		),
	)
	defer span.End()

	return db.QueryRowContext(ctx, query, args...)
}

// ExecWithSpan executes a command and creates a span
func ExecWithSpan(ctx context.Context, tracer trace.Tracer, query string, args ...interface{}) (sql.Result, error) {
	ctx, span := tracer.Start(ctx, "database.exec",
		trace.WithAttributes(
			attribute.String("db.system", "postgresql"),
			attribute.String("db.statement", query),
		),
	)
	defer span.End()

	result, err := db.ExecContext(ctx, query, args...)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}

	return result, nil
}

// CloseDB closes the database connection
func CloseDB() error {
	if db != nil {
		return db.Close()
	}
	return nil
}
