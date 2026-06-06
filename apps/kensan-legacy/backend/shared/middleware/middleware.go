package middleware

import (
	"context"
	"net/http"
	"strings"
	"time"

	"log/slog"

	"github.com/google/uuid"
	"github.com/kensan/backend/shared/auth"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type contextKey string

const (
	UserIDKey    contextKey = "userID"
	RequestIDKey contextKey = "requestID"
)

// RequestID adds a unique request ID to each request
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}

		ctx := context.WithValue(r.Context(), RequestIDKey, requestID)
		w.Header().Set("X-Request-ID", requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Logger logs each request
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap response writer to capture status code
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(wrapped, r)

		requestID, _ := r.Context().Value(RequestIDKey).(string)
		slog.InfoContext(r.Context(), "request completed",
			"request_id", requestID,
			"method", r.Method,
			"path", r.URL.Path,
			"status", wrapped.statusCode,
			"duration", time.Since(start),
		)
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Auth validates JWT token and adds user info to context
func Auth(jwtManager *auth.JWTManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				Error(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authorization header required")
				return
			}

			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				Error(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid authorization header format")
				return
			}

			claims, err := jwtManager.ValidateToken(parts[1])
			if err != nil {
				Error(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid or expired token")
				return
			}

			ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
			if span := trace.SpanFromContext(ctx); span.SpanContext().IsValid() {
				span.SetAttributes(attribute.String("user.id", claims.UserID))
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetUserID extracts user ID from context
func GetUserID(ctx context.Context) string {
	userID, _ := ctx.Value(UserIDKey).(string)
	return userID
}

// GetRequestID extracts request ID from context
func GetRequestID(ctx context.Context) string {
	requestID, _ := ctx.Value(RequestIDKey).(string)
	return requestID
}

