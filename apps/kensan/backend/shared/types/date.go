// Package types provides custom types for database and JSON handling
package types

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

// DateOnly represents a date without time component.
// It scans from PostgreSQL DATE type (time.Time) and marshals to JSON as "YYYY-MM-DD" string.
type DateOnly struct {
	Time  time.Time
	Valid bool // Valid is true if Time is not NULL
}

// DateFormat is the format used for JSON serialization
const DateFormat = "2006-01-02"

// Scan implements the sql.Scanner interface for database reads.
// PostgreSQL DATE type is scanned as time.Time by pgx.
func (d *DateOnly) Scan(src interface{}) error {
	if src == nil {
		d.Time, d.Valid = time.Time{}, false
		return nil
	}

	switch v := src.(type) {
	case time.Time:
		d.Time, d.Valid = v, true
		return nil
	default:
		return fmt.Errorf("DateOnly.Scan: cannot scan type %T into DateOnly", src)
	}
}

// Value implements the driver.Valuer interface for database writes.
func (d DateOnly) Value() (driver.Value, error) {
	if !d.Valid {
		return nil, nil
	}
	return d.Time, nil
}

// MarshalJSON implements the json.Marshaler interface.
// Returns "YYYY-MM-DD" string or null if not valid.
func (d DateOnly) MarshalJSON() ([]byte, error) {
	if !d.Valid {
		return []byte("null"), nil
	}
	return json.Marshal(d.Time.Format(DateFormat))
}

// UnmarshalJSON implements the json.Unmarshaler interface.
// Accepts "YYYY-MM-DD" string or null.
func (d *DateOnly) UnmarshalJSON(data []byte) error {
	// Handle null
	if string(data) == "null" || string(data) == `""` {
		d.Time, d.Valid = time.Time{}, false
		return nil
	}

	// Parse the string
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("DateOnly.UnmarshalJSON: %w", err)
	}

	if s == "" {
		d.Time, d.Valid = time.Time{}, false
		return nil
	}

	t, err := time.Parse(DateFormat, s)
	if err != nil {
		return fmt.Errorf("DateOnly.UnmarshalJSON: invalid date format %q, expected YYYY-MM-DD: %w", s, err)
	}

	d.Time, d.Valid = t, true
	return nil
}

// String returns the date as "YYYY-MM-DD" string or empty string if not valid.
func (d DateOnly) String() string {
	if !d.Valid {
		return ""
	}
	return d.Time.Format(DateFormat)
}

// NewDateOnly creates a new valid DateOnly from time.Time.
func NewDateOnly(t time.Time) DateOnly {
	return DateOnly{Time: t, Valid: true}
}

// NewDateOnlyFromString parses a "YYYY-MM-DD" string into DateOnly.
func NewDateOnlyFromString(s string) (DateOnly, error) {
	if s == "" {
		return DateOnly{}, nil
	}
	t, err := time.Parse(DateFormat, s)
	if err != nil {
		return DateOnly{}, fmt.Errorf("invalid date format %q, expected YYYY-MM-DD: %w", s, err)
	}
	return DateOnly{Time: t, Valid: true}, nil
}
