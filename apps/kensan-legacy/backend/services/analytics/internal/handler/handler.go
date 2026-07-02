package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/kensan/backend/services/analytics/internal"
	"github.com/kensan/backend/services/analytics/internal/service"
	"github.com/kensan/backend/shared/middleware"
)

// Handler handles HTTP requests for analytics
type Handler struct {
	service service.AnalyticsService
}

// NewHandler creates a new analytics handler
func NewHandler(svc service.AnalyticsService) *Handler {
	return &Handler{service: svc}
}

// RegisterRoutes registers the analytics routes
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/analytics", func(r chi.Router) {
		r.Get("/summary", h.GetSummary)
		r.Get("/summary/weekly", h.GetWeeklySummary)
		r.Get("/summary/monthly", h.GetMonthlySummary)
		r.Get("/trends", h.GetTrends)
		r.Get("/daily-study-hours", h.GetDailyStudyHours)
	})
}

// GetSummary handles GET /analytics/summary?start_date=...&end_date=...&timezone=...
func (h *Handler) GetSummary(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	filter := analytics.SummaryFilter{
		StartDate: r.URL.Query().Get("start_date"),
		EndDate:   r.URL.Query().Get("end_date"),
		Timezone:  r.URL.Query().Get("timezone"),
	}

	summary, err := h.service.GetSummaryByDateRange(r.Context(), userID, filter)
	if err != nil {
		if errors.Is(err, service.ErrMissingDateRange) {
			middleware.Error(w, r, http.StatusBadRequest, "MISSING_DATE_RANGE", "start_date and end_date are required")
			return
		}
		if errors.Is(err, service.ErrInvalidDateRange) {
			middleware.Error(w, r, http.StatusBadRequest, "INVALID_DATE_RANGE", "Invalid date format. Use YYYY-MM-DD")
			return
		}
		if middleware.HandleDBSchemaError(w, r, err) {
			return
		}
		slog.ErrorContext(r.Context(), "Failed to get summary", "error", err, "request_id", middleware.GetRequestID(r.Context()))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get summary")
		return
	}

	middleware.JSON(w, r, http.StatusOK, summary)
}

// GetWeeklySummary handles GET /analytics/summary/weekly?week_start=...&timezone=...
func (h *Handler) GetWeeklySummary(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	filter := analytics.WeeklySummaryFilter{
		WeekStart: r.URL.Query().Get("week_start"),
		Timezone:  r.URL.Query().Get("timezone"),
	}

	summary, err := h.service.GetWeeklySummary(r.Context(), userID, filter)
	if err != nil {
		if errors.Is(err, service.ErrInvalidWeekStart) {
			middleware.Error(w, r, http.StatusBadRequest, "INVALID_WEEK_START", "Invalid week_start format. Use YYYY-MM-DD")
			return
		}
		if middleware.HandleDBSchemaError(w, r, err) {
			return
		}
		slog.ErrorContext(r.Context(), "Failed to get weekly summary", "error", err, "request_id", middleware.GetRequestID(r.Context()))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get weekly summary")
		return
	}

	middleware.JSON(w, r, http.StatusOK, summary)
}

// GetMonthlySummary handles GET /analytics/summary/monthly?year=...&month=...&timezone=...
func (h *Handler) GetMonthlySummary(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	filter := analytics.MonthlySummaryFilter{
		Timezone: r.URL.Query().Get("timezone"),
	}

	// Parse year
	if yearStr := r.URL.Query().Get("year"); yearStr != "" {
		year, err := strconv.Atoi(yearStr)
		if err != nil {
			middleware.Error(w, r, http.StatusBadRequest, "INVALID_YEAR", "Invalid year format")
			return
		}
		filter.Year = year
	}

	// Parse month
	if monthStr := r.URL.Query().Get("month"); monthStr != "" {
		month, err := strconv.Atoi(monthStr)
		if err != nil {
			middleware.Error(w, r, http.StatusBadRequest, "INVALID_MONTH", "Invalid month format")
			return
		}
		filter.Month = month
	}

	summary, err := h.service.GetMonthlySummary(r.Context(), userID, filter)
	if err != nil {
		if errors.Is(err, service.ErrInvalidMonth) {
			middleware.Error(w, r, http.StatusBadRequest, "INVALID_MONTH", "Month must be between 1 and 12")
			return
		}
		if errors.Is(err, service.ErrInvalidYear) {
			middleware.Error(w, r, http.StatusBadRequest, "INVALID_YEAR", "Invalid year value")
			return
		}
		if middleware.HandleDBSchemaError(w, r, err) {
			return
		}
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get monthly summary")
		return
	}

	middleware.JSON(w, r, http.StatusOK, summary)
}

// GetTrends handles GET /analytics/trends
func (h *Handler) GetTrends(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	filter := analytics.TrendFilter{
		Period: analytics.TrendPeriod(r.URL.Query().Get("period")),
	}

	// Parse count
	if countStr := r.URL.Query().Get("count"); countStr != "" {
		count, err := strconv.Atoi(countStr)
		if err != nil {
			middleware.Error(w, r, http.StatusBadRequest, "INVALID_COUNT", "Invalid count format")
			return
		}
		filter.Count = count
	}

	trends, err := h.service.GetTrends(r.Context(), userID, filter)
	if err != nil {
		if errors.Is(err, service.ErrInvalidPeriod) {
			middleware.Error(w, r, http.StatusBadRequest, "INVALID_PERIOD", "Period must be 'week', 'month', or 'quarter'")
			return
		}
		if errors.Is(err, service.ErrInvalidCount) {
			middleware.Error(w, r, http.StatusBadRequest, "INVALID_COUNT", "Count must be a positive integer")
			return
		}
		if middleware.HandleDBSchemaError(w, r, err) {
			return
		}
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get trends")
		return
	}

	middleware.JSON(w, r, http.StatusOK, trends)
}

// GetDailyStudyHours handles GET /analytics/daily-study-hours?timezone=...
func (h *Handler) GetDailyStudyHours(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	filter := analytics.DailyStudyHoursFilter{
		StartDate: r.URL.Query().Get("start_date"),
		EndDate:   r.URL.Query().Get("end_date"),
		Timezone:  r.URL.Query().Get("timezone"),
	}

	// Parse days (only used if start_date/end_date not provided)
	if daysStr := r.URL.Query().Get("days"); daysStr != "" {
		days, err := strconv.Atoi(daysStr)
		if err != nil {
			middleware.Error(w, r, http.StatusBadRequest, "INVALID_DAYS", "Invalid days format")
			return
		}
		filter.Days = days
	}

	hours, err := h.service.GetDailyStudyHours(r.Context(), userID, filter)
	if err != nil {
		if errors.Is(err, service.ErrInvalidDateRange) {
			middleware.Error(w, r, http.StatusBadRequest, "INVALID_DATE_RANGE", "Invalid date format. Use YYYY-MM-DD")
			return
		}
		if middleware.HandleDBSchemaError(w, r, err) {
			return
		}
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get daily study hours")
		return
	}

	middleware.JSON(w, r, http.StatusOK, hours)
}
