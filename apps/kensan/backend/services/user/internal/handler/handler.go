package handler

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/kensan/backend/services/user/internal"
	"github.com/kensan/backend/services/user/internal/service"
	"github.com/kensan/backend/shared/middleware"
	"log/slog"
)

// Handler handles HTTP requests for user operations
type Handler struct {
	service service.FullService
}

// NewHandler creates a new user handler
func NewHandler(svc service.FullService) *Handler {
	return &Handler{service: svc}
}

// RegisterPublicRoutes registers routes that don't require authentication.
func (h *Handler) RegisterPublicRoutes(r chi.Router) {
	r.Post("/auth/register", h.Register)
	r.Post("/auth/login", h.Login)
}

// RegisterRoutes registers routes that require authentication.
// Authentication middleware is expected to be applied by the caller.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/users/me", h.GetProfile)
	r.Put("/users/me", h.UpdateProfile)
	r.Get("/users/me/settings", h.GetSettings)
	r.Put("/users/me/settings", h.UpdateSettings)
	r.Post("/users/me/ai-consent", h.GiveAIConsent)
}

// Register handles user registration
// POST /auth/register
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req user.RegisterRequest
	if !middleware.DecodeJSONBody(w, r, &req) {
		return
	}

	result, err := h.service.Register(r.Context(), &req)
	if err != nil {
		h.handleError(w, r, err)
		return
	}

	middleware.JSON(w, r, http.StatusCreated, result)
}

// Login handles user login
// POST /auth/login
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req user.LoginRequest
	if !middleware.DecodeJSONBody(w, r, &req) {
		return
	}

	result, err := h.service.Login(r.Context(), &req)
	if err != nil {
		h.handleError(w, r, err)
		return
	}

	middleware.JSON(w, r, http.StatusOK, result)
}

// GetProfile handles getting the current user's profile
// GET /users/me
func (h *Handler) GetProfile(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	profile, err := h.service.GetProfile(r.Context(), userID)
	if err != nil {
		h.handleError(w, r, err)
		return
	}

	middleware.JSON(w, r, http.StatusOK, profile)
}

// UpdateProfile handles updating the current user's profile
// PUT /users/me
func (h *Handler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var req user.UpdateProfileRequest
	if !middleware.DecodeJSONBody(w, r, &req) {
		return
	}

	profile, err := h.service.UpdateProfile(r.Context(), userID, &req)
	if err != nil {
		h.handleError(w, r, err)
		return
	}

	middleware.JSON(w, r, http.StatusOK, profile)
}

// GetSettings handles getting the current user's settings
// GET /users/me/settings
func (h *Handler) GetSettings(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	settings, err := h.service.GetSettings(r.Context(), userID)
	if err != nil {
		h.handleError(w, r, err)
		return
	}

	middleware.JSON(w, r, http.StatusOK, settings)
}

// UpdateSettings handles updating the current user's settings
// PUT /users/me/settings
func (h *Handler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var req user.UpdateSettingsRequest
	if !middleware.DecodeJSONBody(w, r, &req) {
		return
	}

	settings, err := h.service.UpdateSettings(r.Context(), userID, &req)
	if err != nil {
		h.handleError(w, r, err)
		return
	}

	middleware.JSON(w, r, http.StatusOK, settings)
}

// GiveAIConsent handles recording the user's AI consent
// POST /users/me/ai-consent
func (h *Handler) GiveAIConsent(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var req user.AIConsentRequest
	if !middleware.DecodeJSONBody(w, r, &req) {
		return
	}

	settings, err := h.service.GiveAIConsent(r.Context(), userID, req.Consent)
	if err != nil {
		h.handleError(w, r, err)
		return
	}

	middleware.JSON(w, r, http.StatusOK, settings)
}

// handleError handles service errors and returns appropriate HTTP responses
func (h *Handler) handleError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidCredentials):
		middleware.Error(w, r, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid email or password")
	case errors.Is(err, service.ErrUserExists):
		middleware.Error(w, r, http.StatusConflict, "USER_EXISTS", "User with this email already exists")
	case errors.Is(err, service.ErrUserNotFound):
		middleware.Error(w, r, http.StatusNotFound, "USER_NOT_FOUND", "User not found")
	case errors.Is(err, service.ErrEmailRequired):
		middleware.ValidationError(w, r, []middleware.ErrorDetail{{Field: "email", Message: "Email is required"}})
	case errors.Is(err, service.ErrPasswordRequired):
		middleware.ValidationError(w, r, []middleware.ErrorDetail{{Field: "password", Message: "Password is required"}})
	case errors.Is(err, service.ErrNameRequired):
		middleware.ValidationError(w, r, []middleware.ErrorDetail{{Field: "name", Message: "Name is required"}})
	case errors.Is(err, service.ErrInvalidEmail):
		middleware.ValidationError(w, r, []middleware.ErrorDetail{{Field: "email", Message: "Invalid email format"}})
	case errors.Is(err, service.ErrPasswordTooShort):
		middleware.ValidationError(w, r, []middleware.ErrorDetail{{Field: "password", Message: "Password must be at least 8 characters"}})
	case errors.Is(err, service.ErrInvalidTheme):
		middleware.ValidationError(w, r, []middleware.ErrorDetail{{Field: "theme", Message: "Theme must be light, dark, or system"}})
	default:
		slog.ErrorContext(r.Context(), "Unhandled error in user-service", "error", err, "request_id", middleware.GetRequestID(r.Context()))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal error occurred")
	}
}
