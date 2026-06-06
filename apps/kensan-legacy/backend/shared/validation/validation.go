// Package validation provides common validation utilities for all services.
package validation

import "regexp"

// Common regex patterns
var (
	// DateRegex matches YYYY-MM-DD format
	DateRegex = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

	// TimeRegex matches HH:mm format (24-hour)
	TimeRegex = regexp.MustCompile(`^([01]\d|2[0-3]):[0-5]\d$`)

	// EmailRegex matches basic email format
	EmailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
)

// IsValidDate checks if the string is in YYYY-MM-DD format
func IsValidDate(date string) bool {
	return DateRegex.MatchString(date)
}

// IsValidTime checks if the string is in HH:mm format (24-hour)
func IsValidTime(time string) bool {
	return TimeRegex.MatchString(time)
}

// IsValidEmail checks if the string is a valid email format
func IsValidEmail(email string) bool {
	return EmailRegex.MatchString(email)
}
