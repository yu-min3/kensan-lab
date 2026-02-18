package handler

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/kensan/backend/services/timeblock/internal"
	"github.com/kensan/backend/services/timeblock/internal/service"
	"github.com/kensan/backend/shared/middleware"
	"log/slog"
)

// Handler handles HTTP requests for time blocks and time entries
type Handler struct {
	service service.FullService
}

// NewHandler creates a new timeblock handler
func NewHandler(svc service.FullService) *Handler {
	return &Handler{service: svc}
}

// RegisterRoutes registers the timeblock routes
func (h *Handler) RegisterRoutes(r chi.Router) {
	// TimeBlock routes
	r.Route("/timeblocks", func(r chi.Router) {
		r.Get("/", h.ListTimeBlocks)
		r.Post("/", h.CreateTimeBlock)
		r.Put("/{timeBlockId}", h.UpdateTimeBlock)
		r.Delete("/{timeBlockId}", h.DeleteTimeBlock)
	})

	// TimeEntry routes
	r.Route("/time-entries", func(r chi.Router) {
		r.Get("/", h.ListTimeEntries)
		r.Post("/", h.CreateTimeEntry)
		r.Put("/{entryId}", h.UpdateTimeEntry)
		r.Delete("/{entryId}", h.DeleteTimeEntry)
	})

	// Timer routes
	r.Route("/timer", func(r chi.Router) {
		r.Get("/current", h.GetCurrentTimer)
		r.Post("/start", h.StartTimer)
		r.Post("/stop", h.StopTimer)
	})
}

// ========== TimeBlock Handlers ==========

// ListTimeBlocks handles GET /timeblocks
func (h *Handler) ListTimeBlocks(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	filter := timeblock.TimeBlockFilter{}

	// Parse start_datetime and end_datetime (UTC ISO 8601 range)
	if startDt := r.URL.Query().Get("start_datetime"); startDt != "" {
		filter.StartDatetime = &startDt
	}
	if endDt := r.URL.Query().Get("end_datetime"); endDt != "" {
		filter.EndDatetime = &endDt
	}

	blocks, err := h.service.ListTimeBlocks(r.Context(), userID, filter)
	if err != nil {
		slog.ErrorContext(r.Context(), "Failed to list time blocks", "error", err, "request_id", middleware.GetRequestID(r.Context()))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list time blocks")
		return
	}

	middleware.JSON(w, r, http.StatusOK, blocks)
}

// CreateTimeBlock handles POST /timeblocks
func (h *Handler) CreateTimeBlock(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var input timeblock.CreateTimeBlockInput
	if !middleware.DecodeJSONBody(w, r, &input) {
		return
	}

	// Validation
	var validationErrors []middleware.ErrorDetail
	if input.TaskName == "" {
		validationErrors = append(validationErrors, middleware.ErrorDetail{
			Field: "taskName", Message: "Task name is required",
		})
	}
	if input.StartDatetime == "" {
		validationErrors = append(validationErrors, middleware.ErrorDetail{
			Field: "startDatetime", Message: "Start datetime is required",
		})
	}
	if input.EndDatetime == "" {
		validationErrors = append(validationErrors, middleware.ErrorDetail{
			Field: "endDatetime", Message: "End datetime is required",
		})
	}
	if len(validationErrors) > 0 {
		middleware.ValidationError(w, r, validationErrors)
		return
	}

	tb, err := h.service.CreateTimeBlock(r.Context(), userID, input)
	if err != nil {
		if errors.Is(err, service.ErrInvalidDatetime) {
			middleware.Error(w, r, http.StatusBadRequest, "INVALID_DATETIME", "Invalid datetime format (expected ISO 8601, e.g., 2026-01-20T15:00:00Z)")
			return
		}
		if errors.Is(err, service.ErrInvalidInput) {
			middleware.Error(w, r, http.StatusBadRequest, "INVALID_INPUT", "Invalid input")
			return
		}
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create time block")
		return
	}

	middleware.JSON(w, r, http.StatusCreated, tb)
}

// UpdateTimeBlock handles PUT /timeblocks/{timeBlockId}
func (h *Handler) UpdateTimeBlock(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	timeBlockID := chi.URLParam(r, "timeBlockId")

	var input timeblock.UpdateTimeBlockInput
	if !middleware.DecodeJSONBody(w, r, &input) {
		return
	}

	tb, err := h.service.UpdateTimeBlock(r.Context(), userID, timeBlockID, input)
	if err != nil {
		if errors.Is(err, service.ErrTimeBlockNotFound) {
			middleware.Error(w, r, http.StatusNotFound, "TIME_BLOCK_NOT_FOUND", "Time block not found")
			return
		}
		if errors.Is(err, service.ErrInvalidDatetime) {
			middleware.Error(w, r, http.StatusBadRequest, "INVALID_DATETIME", "Invalid datetime format (expected ISO 8601)")
			return
		}
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update time block")
		return
	}

	middleware.JSON(w, r, http.StatusOK, tb)
}

// DeleteTimeBlock handles DELETE /timeblocks/{timeBlockId}
func (h *Handler) DeleteTimeBlock(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	timeBlockID := chi.URLParam(r, "timeBlockId")

	err := h.service.DeleteTimeBlock(r.Context(), userID, timeBlockID)
	if err != nil {
		if errors.Is(err, service.ErrTimeBlockNotFound) {
			middleware.Error(w, r, http.StatusNotFound, "TIME_BLOCK_NOT_FOUND", "Time block not found")
			return
		}
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to delete time block")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ========== TimeEntry Handlers ==========

// ListTimeEntries handles GET /time-entries
func (h *Handler) ListTimeEntries(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	filter := timeblock.TimeEntryFilter{}

	// Parse start_datetime and end_datetime (UTC ISO 8601 range)
	if startDt := r.URL.Query().Get("start_datetime"); startDt != "" {
		filter.StartDatetime = &startDt
	}
	if endDt := r.URL.Query().Get("end_datetime"); endDt != "" {
		filter.EndDatetime = &endDt
	}

	entries, err := h.service.ListTimeEntries(r.Context(), userID, filter)
	if err != nil {
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list time entries")
		return
	}

	middleware.JSON(w, r, http.StatusOK, entries)
}

// CreateTimeEntry handles POST /time-entries
func (h *Handler) CreateTimeEntry(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var input timeblock.CreateTimeEntryInput
	if !middleware.DecodeJSONBody(w, r, &input) {
		return
	}

	// Validation
	var validationErrors []middleware.ErrorDetail
	if input.TaskName == "" {
		validationErrors = append(validationErrors, middleware.ErrorDetail{
			Field: "taskName", Message: "Task name is required",
		})
	}
	if input.StartDatetime == "" {
		validationErrors = append(validationErrors, middleware.ErrorDetail{
			Field: "startDatetime", Message: "Start datetime is required",
		})
	}
	if input.EndDatetime == "" {
		validationErrors = append(validationErrors, middleware.ErrorDetail{
			Field: "endDatetime", Message: "End datetime is required",
		})
	}
	if len(validationErrors) > 0 {
		middleware.ValidationError(w, r, validationErrors)
		return
	}

	te, err := h.service.CreateTimeEntry(r.Context(), userID, input)
	if err != nil {
		if errors.Is(err, service.ErrInvalidDatetime) {
			middleware.Error(w, r, http.StatusBadRequest, "INVALID_DATETIME", "Invalid datetime format (expected ISO 8601, e.g., 2026-01-20T15:00:00Z)")
			return
		}
		if errors.Is(err, service.ErrInvalidInput) {
			middleware.Error(w, r, http.StatusBadRequest, "INVALID_INPUT", "Invalid input")
			return
		}
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create time entry")
		return
	}

	middleware.JSON(w, r, http.StatusCreated, te)
}

// UpdateTimeEntry handles PUT /time-entries/{entryId}
func (h *Handler) UpdateTimeEntry(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	entryID := chi.URLParam(r, "entryId")

	var input timeblock.UpdateTimeEntryInput
	if !middleware.DecodeJSONBody(w, r, &input) {
		return
	}

	te, err := h.service.UpdateTimeEntry(r.Context(), userID, entryID, input)
	if err != nil {
		if errors.Is(err, service.ErrTimeEntryNotFound) {
			middleware.Error(w, r, http.StatusNotFound, "TIME_ENTRY_NOT_FOUND", "Time entry not found")
			return
		}
		if errors.Is(err, service.ErrInvalidDatetime) {
			middleware.Error(w, r, http.StatusBadRequest, "INVALID_DATETIME", "Invalid datetime format (expected ISO 8601)")
			return
		}
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update time entry")
		return
	}

	middleware.JSON(w, r, http.StatusOK, te)
}

// DeleteTimeEntry handles DELETE /time-entries/{entryId}
func (h *Handler) DeleteTimeEntry(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	entryID := chi.URLParam(r, "entryId")

	err := h.service.DeleteTimeEntry(r.Context(), userID, entryID)
	if err != nil {
		if errors.Is(err, service.ErrTimeEntryNotFound) {
			middleware.Error(w, r, http.StatusNotFound, "TIME_ENTRY_NOT_FOUND", "Time entry not found")
			return
		}
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to delete time entry")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ========== Timer Handlers ==========

// GetCurrentTimer handles GET /timer/current
func (h *Handler) GetCurrentTimer(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	timer, err := h.service.GetRunningTimer(r.Context(), userID)
	if err != nil {
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get running timer")
		return
	}

	middleware.JSON(w, r, http.StatusOK, timer)
}

// StartTimer handles POST /timer/start
func (h *Handler) StartTimer(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var input timeblock.StartTimerInput
	if !middleware.DecodeJSONBody(w, r, &input) {
		return
	}

	// Validation
	var validationErrors []middleware.ErrorDetail
	if input.TaskName == "" {
		validationErrors = append(validationErrors, middleware.ErrorDetail{
			Field: "taskName", Message: "Task name is required",
		})
	}
	if len(validationErrors) > 0 {
		middleware.ValidationError(w, r, validationErrors)
		return
	}

	timer, err := h.service.StartTimer(r.Context(), userID, input)
	if err != nil {
		if errors.Is(err, service.ErrTimerAlreadyRunning) {
			middleware.Error(w, r, http.StatusConflict, "TIMER_ALREADY_RUNNING", "A timer is already running. Stop it first before starting a new one.")
			return
		}
		if errors.Is(err, service.ErrInvalidInput) {
			middleware.Error(w, r, http.StatusBadRequest, "INVALID_INPUT", "Invalid input")
			return
		}
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to start timer")
		return
	}

	middleware.JSON(w, r, http.StatusCreated, timer)
}

// StopTimer handles POST /timer/stop
func (h *Handler) StopTimer(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	result, err := h.service.StopTimer(r.Context(), userID)
	if err != nil {
		if errors.Is(err, service.ErrRunningTimerNotFound) {
			middleware.Error(w, r, http.StatusNotFound, "NO_RUNNING_TIMER", "No running timer to stop")
			return
		}
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to stop timer")
		return
	}

	middleware.JSON(w, r, http.StatusOK, result)
}
