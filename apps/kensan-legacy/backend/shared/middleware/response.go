package middleware

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	sharedErrors "github.com/kensan/backend/shared/errors"
)

// Response is the standard API response format
type Response struct {
	Data       interface{}          `json:"data,omitempty"`
	Error      *ErrorResponse       `json:"error,omitempty"`
	Meta       MetaResponse         `json:"meta"`
	Pagination *PaginationResponse  `json:"pagination,omitempty"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details []ErrorDetail  `json:"details,omitempty"`
}

// ErrorDetail represents a validation error detail
type ErrorDetail struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// MetaResponse contains metadata about the response
type MetaResponse struct {
	RequestID string `json:"requestId"`
	Timestamp string `json:"timestamp"`
}

// PaginationResponse contains pagination information
type PaginationResponse struct {
	Page       int `json:"page"`
	PerPage    int `json:"perPage"`
	Total      int `json:"total"`
	TotalPages int `json:"totalPages"`
}

// JSON writes a JSON response
func JSON(w http.ResponseWriter, r *http.Request, status int, data interface{}) {
	response := Response{
		Data: data,
		Meta: MetaResponse{
			RequestID: GetRequestID(r.Context()),
			Timestamp: time.Now().Format(time.RFC3339),
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(response)
}

// JSONWithPagination writes a JSON response with pagination
func JSONWithPagination(w http.ResponseWriter, r *http.Request, status int, data interface{}, pagination PaginationResponse) {
	response := Response{
		Data: data,
		Meta: MetaResponse{
			RequestID: GetRequestID(r.Context()),
			Timestamp: time.Now().Format(time.RFC3339),
		},
		Pagination: &pagination,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(response)
}

// Error writes an error response
func Error(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	response := Response{
		Error: &ErrorResponse{
			Code:    code,
			Message: message,
		},
		Meta: MetaResponse{
			RequestID: GetRequestID(r.Context()),
			Timestamp: time.Now().Format(time.RFC3339),
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(response)
}

// ValidationError writes a validation error response
func ValidationError(w http.ResponseWriter, r *http.Request, details []ErrorDetail) {
	response := Response{
		Error: &ErrorResponse{
			Code:    "VALIDATION_ERROR",
			Message: "入力値が不正です",
			Details: details,
		},
		Meta: MetaResponse{
			RequestID: GetRequestID(r.Context()),
			Timestamp: time.Now().Format(time.RFC3339),
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	json.NewEncoder(w).Encode(response)
}

// DecodeJSONBody decodes JSON request body into the provided value.
// Returns false and writes an error response if decoding fails.
func DecodeJSONBody(w http.ResponseWriter, r *http.Request, v interface{}) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		Error(w, r, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON body")
		return false
	}
	return true
}

// RequireURLParam extracts a URL parameter and validates it's not empty.
// Returns the value and true if valid, or writes an error response and returns false.
func RequireURLParam(w http.ResponseWriter, r *http.Request, paramName string) (string, bool) {
	value := chi.URLParam(r, paramName)
	if value == "" {
		Error(w, r, http.StatusBadRequest, "INVALID_REQUEST", paramName+" is required")
		return "", false
	}
	return value, true
}

// HandleDBSchemaError checks if err is a database schema error and sends the appropriate response.
// Returns true if handled (caller should return), false if not a schema error.
func HandleDBSchemaError(w http.ResponseWriter, r *http.Request, err error) bool {
	if sharedErrors.IsDatabaseSchema(err) {
		slog.ErrorContext(r.Context(), "Database schema error", "error", err, "request_id", GetRequestID(r.Context()))
		Error(w, r, http.StatusInternalServerError, "DB_SCHEMA_ERROR", err.Error())
		return true
	}
	return false
}

// ErrorMapping defines how a service error should be mapped to an HTTP response.
type ErrorMapping struct {
	Status  int
	Code    string
	Message string
}

// HandleServiceError maps a service error to an HTTP response.
// It checks the error against the provided mappings and writes the appropriate response.
// If no mapping matches, it writes a 500 Internal Server Error.
//
// Usage:
//
//	err := h.service.DoSomething(...)
//	if err != nil {
//	    middleware.HandleServiceError(w, r, err, map[error]middleware.ErrorMapping{
//	        service.ErrNotFound: {http.StatusNotFound, "NOT_FOUND", "Resource not found"},
//	        service.ErrInvalidInput: {http.StatusBadRequest, "INVALID_INPUT", "Invalid input"},
//	    }, "Failed to do something")
//	    return
//	}
func HandleServiceError(w http.ResponseWriter, r *http.Request, err error, mappings map[error]ErrorMapping, defaultMessage string) {
	for targetErr, mapping := range mappings {
		if err == targetErr {
			Error(w, r, mapping.Status, mapping.Code, mapping.Message)
			return
		}
	}
	// Default: Internal Server Error
	Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", defaultMessage)
}

// ValidateRequired checks that all specified fields have non-empty values.
// Returns true if all fields are valid, false otherwise.
// On failure, writes a ValidationError response with details for all empty fields.
//
// Usage:
//
//	if !middleware.ValidateRequired(w, r, map[string]string{
//	    "name": input.Name,
//	    "email": input.Email,
//	}) {
//	    return
//	}
func ValidateRequired(w http.ResponseWriter, r *http.Request, fields map[string]string) bool {
	var details []ErrorDetail
	for field, value := range fields {
		if value == "" {
			details = append(details, ErrorDetail{
				Field:   field,
				Message: field + " is required",
			})
		}
	}
	if len(details) > 0 {
		ValidationError(w, r, details)
		return false
	}
	return true
}

