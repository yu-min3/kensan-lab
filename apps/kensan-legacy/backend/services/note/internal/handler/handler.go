package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/kensan/backend/services/note/internal"
	"github.com/kensan/backend/services/note/internal/service"
	"github.com/kensan/backend/shared/middleware"
	"log/slog"
)

// Handler handles HTTP requests for note operations
type Handler struct {
	service service.FullService
}

// NewHandler creates a new note handler
func NewHandler(svc service.FullService) *Handler {
	return &Handler{service: svc}
}

// RegisterRoutes registers the note routes.
// Authentication middleware is expected to be applied by the caller.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/note-types", h.ListNoteTypes)
	r.Get("/notes", h.List)
	r.Post("/notes", h.Create)
	r.Get("/notes/search", h.Search)
	r.Get("/notes/{noteId}", h.GetByID)
	r.Put("/notes/{noteId}", h.Update)
	r.Delete("/notes/{noteId}", h.Delete)
	r.Post("/notes/{noteId}/archive", h.Archive)

	// NoteContent routes
	r.Get("/notes/{noteId}/contents", h.ListContents)
	r.Post("/notes/{noteId}/contents", h.CreateContent)
	r.Get("/notes/{noteId}/contents/{contentId}", h.GetContent)
	r.Put("/notes/{noteId}/contents/{contentId}", h.UpdateContent)
	r.Delete("/notes/{noteId}/contents/{contentId}", h.DeleteContent)
	r.Patch("/notes/{noteId}/contents/reorder", h.ReorderContents)

	// Storage routes
	r.Post("/notes/{noteId}/contents/upload-url", h.GetUploadURL)
	r.Get("/notes/{noteId}/contents/{contentId}/download-url", h.GetDownloadURL)
}

// List handles listing notes
// GET /notes
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	// Parse query parameters for filtering
	filter := &note.NoteFilter{}

	if typesParam := r.URL.Query().Get("types"); typesParam != "" {
		types := strings.Split(typesParam, ",")
		for _, t := range types {
			trimmed := strings.TrimSpace(t)
			if trimmed != "" {
				filter.Types = append(filter.Types, note.NoteType(trimmed))
			}
		}
	}
	if goalID := r.URL.Query().Get("goal_id"); goalID != "" {
		filter.GoalID = &goalID
	}
	if milestoneID := r.URL.Query().Get("milestone_id"); milestoneID != "" {
		filter.MilestoneID = &milestoneID
	}
	if taskID := r.URL.Query().Get("task_id"); taskID != "" {
		filter.TaskID = &taskID
	}
	if formatStr := r.URL.Query().Get("format"); formatStr != "" {
		format := note.NoteFormat(formatStr)
		if format.IsValid() {
			filter.Format = &format
		}
	}
	if dateFrom := r.URL.Query().Get("date_from"); dateFrom != "" {
		filter.DateFrom = &dateFrom
	}
	if dateTo := r.URL.Query().Get("date_to"); dateTo != "" {
		filter.DateTo = &dateTo
	}
	if archivedStr := r.URL.Query().Get("archived"); archivedStr != "" {
		archived := archivedStr == "true"
		filter.Archived = &archived
	}
	if query := r.URL.Query().Get("q"); query != "" {
		filter.Query = &query
	}
	if tagIDsParam := r.URL.Query().Get("tag_ids"); tagIDsParam != "" {
		filter.TagIDs = strings.Split(tagIDsParam, ",")
	}

	notes, err := h.service.List(r.Context(), userID, filter)
	if err != nil {
		h.handleError(w, r, err)
		return
	}

	middleware.JSON(w, r, http.StatusOK, notes)
}

// GetByID handles getting a note by ID
// GET /notes/{noteId}
func (h *Handler) GetByID(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	noteID, ok := middleware.RequireURLParam(w, r, "noteId")
	if !ok {
		return
	}

	n, err := h.service.GetByID(r.Context(), userID, noteID)
	if err != nil {
		h.handleError(w, r, err)
		return
	}

	middleware.JSON(w, r, http.StatusOK, n)
}

// Create handles creating a new note
// POST /notes
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var input note.CreateNoteInput
	if !middleware.DecodeJSONBody(w, r, &input) {
		return
	}

	n, err := h.service.Create(r.Context(), userID, &input)
	if err != nil {
		h.handleError(w, r, err)
		return
	}

	middleware.JSON(w, r, http.StatusCreated, n)
}

// Update handles updating a note
// PUT /notes/{noteId}
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	noteID, ok := middleware.RequireURLParam(w, r, "noteId")
	if !ok {
		return
	}

	var input note.UpdateNoteInput
	if !middleware.DecodeJSONBody(w, r, &input) {
		return
	}

	n, err := h.service.Update(r.Context(), userID, noteID, &input)
	if err != nil {
		h.handleError(w, r, err)
		return
	}

	middleware.JSON(w, r, http.StatusOK, n)
}

// Delete handles deleting a note
// DELETE /notes/{noteId}
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	noteID, ok := middleware.RequireURLParam(w, r, "noteId")
	if !ok {
		return
	}

	if err := h.service.Delete(r.Context(), userID, noteID); err != nil {
		h.handleError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Archive handles archiving/unarchiving a note
// POST /notes/{noteId}/archive
func (h *Handler) Archive(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	noteID, ok := middleware.RequireURLParam(w, r, "noteId")
	if !ok {
		return
	}

	var input struct {
		Archived bool `json:"archived"`
	}
	if !middleware.DecodeJSONBody(w, r, &input) {
		return
	}

	n, err := h.service.Archive(r.Context(), userID, noteID, input.Archived)
	if err != nil {
		h.handleError(w, r, err)
		return
	}

	middleware.JSON(w, r, http.StatusOK, n)
}

// Search handles searching notes
// GET /notes/search
func (h *Handler) Search(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	query := r.URL.Query().Get("q")

	// Parse filter
	filter := &note.NoteFilter{}
	if typesParam := r.URL.Query().Get("types"); typesParam != "" {
		types := strings.Split(typesParam, ",")
		for _, t := range types {
			trimmed := strings.TrimSpace(t)
			if trimmed != "" {
				filter.Types = append(filter.Types, note.NoteType(trimmed))
			}
		}
	}
	if archivedStr := r.URL.Query().Get("archived"); archivedStr != "" {
		archived := archivedStr == "true"
		filter.Archived = &archived
	}

	// Parse limit
	limit := 20
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	results, err := h.service.Search(r.Context(), userID, query, filter, limit)
	if err != nil {
		h.handleError(w, r, err)
		return
	}

	middleware.JSON(w, r, http.StatusOK, results)
}

// handleError handles service errors and returns appropriate HTTP responses
func (h *Handler) handleError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, service.ErrNoteNotFound):
		middleware.Error(w, r, http.StatusNotFound, "NOTE_NOT_FOUND", "Note not found")
	case errors.Is(err, service.ErrNoteAlreadyExists):
		middleware.Error(w, r, http.StatusConflict, "NOTE_ALREADY_EXISTS", "A note of this type already exists for the specified date")
	case errors.Is(err, service.ErrUnauthorized):
		middleware.Error(w, r, http.StatusForbidden, "FORBIDDEN", "Not authorized to access this note")
	case errors.Is(err, service.ErrTypeRequired):
		middleware.ValidationError(w, r, []middleware.ErrorDetail{{Field: "type", Message: "Type is required"}})
	case errors.Is(err, service.ErrInvalidType):
		middleware.ValidationError(w, r, []middleware.ErrorDetail{{Field: "type", Message: "Invalid note type"}})
	case errors.Is(err, service.ErrMetadataValidation):
		middleware.ValidationError(w, r, []middleware.ErrorDetail{{Field: "metadata", Message: err.Error()}})
	case errors.Is(err, service.ErrTitleRequired):
		middleware.ValidationError(w, r, []middleware.ErrorDetail{{Field: "title", Message: "Title is required for diary and learning notes"}})
	case errors.Is(err, service.ErrContentRequired):
		middleware.ValidationError(w, r, []middleware.ErrorDetail{{Field: "content", Message: "Content is required"}})
	case errors.Is(err, service.ErrFormatRequired):
		middleware.ValidationError(w, r, []middleware.ErrorDetail{{Field: "format", Message: "Format is required"}})
	case errors.Is(err, service.ErrInvalidFormat):
		middleware.ValidationError(w, r, []middleware.ErrorDetail{{Field: "format", Message: "Format must be markdown or drawio"}})
	case errors.Is(err, service.ErrDateRequired):
		middleware.ValidationError(w, r, []middleware.ErrorDetail{{Field: "date", Message: "Date is required for diary and learning notes"}})
	case errors.Is(err, service.ErrQueryRequired):
		middleware.ValidationError(w, r, []middleware.ErrorDetail{{Field: "q", Message: "Query is required for search"}})
	case errors.Is(err, service.ErrContentNotFound):
		middleware.Error(w, r, http.StatusNotFound, "CONTENT_NOT_FOUND", "Content not found")
	case errors.Is(err, service.ErrContentTypeRequired):
		middleware.ValidationError(w, r, []middleware.ErrorDetail{{Field: "contentType", Message: "Content type is required"}})
	case errors.Is(err, service.ErrInvalidContentType):
		middleware.ValidationError(w, r, []middleware.ErrorDetail{{Field: "contentType", Message: "Invalid content type"}})
	case errors.Is(err, service.ErrStorageUnavailable):
		middleware.Error(w, r, http.StatusServiceUnavailable, "STORAGE_UNAVAILABLE", "Storage service is not configured")
	default:
		slog.ErrorContext(r.Context(), "Unhandled error in note-service", "error", err, "request_id", middleware.GetRequestID(r.Context()))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal error occurred")
	}
}

// ListNoteTypes handles listing available note types
// GET /note-types
func (h *Handler) ListNoteTypes(w http.ResponseWriter, r *http.Request) {
	types, err := h.service.GetNoteTypes(r.Context())
	if err != nil {
		h.handleError(w, r, err)
		return
	}

	middleware.JSON(w, r, http.StatusOK, types)
}

// ========== NoteContent Handlers ==========

// ListContents handles listing contents for a note
// GET /notes/{noteId}/contents
func (h *Handler) ListContents(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	noteID := chi.URLParam(r, "noteId")

	contents, err := h.service.ListContents(r.Context(), userID, noteID)
	if err != nil {
		h.handleError(w, r, err)
		return
	}

	middleware.JSON(w, r, http.StatusOK, contents)
}

// GetContent handles getting a specific content
// GET /notes/{noteId}/contents/{contentId}
func (h *Handler) GetContent(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	noteID := chi.URLParam(r, "noteId")
	contentID := chi.URLParam(r, "contentId")

	content, err := h.service.GetContent(r.Context(), userID, noteID, contentID)
	if err != nil {
		h.handleError(w, r, err)
		return
	}

	middleware.JSON(w, r, http.StatusOK, content)
}

// CreateContent handles creating a new content
// POST /notes/{noteId}/contents
func (h *Handler) CreateContent(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	noteID := chi.URLParam(r, "noteId")

	var input note.CreateNoteContentInput
	if !middleware.DecodeJSONBody(w, r, &input) {
		return
	}

	content, err := h.service.CreateContent(r.Context(), userID, noteID, &input)
	if err != nil {
		h.handleError(w, r, err)
		return
	}

	middleware.JSON(w, r, http.StatusCreated, content)
}

// UpdateContent handles updating an existing content
// PUT /notes/{noteId}/contents/{contentId}
func (h *Handler) UpdateContent(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	noteID := chi.URLParam(r, "noteId")
	contentID := chi.URLParam(r, "contentId")

	var input note.UpdateNoteContentInput
	if !middleware.DecodeJSONBody(w, r, &input) {
		return
	}

	content, err := h.service.UpdateContent(r.Context(), userID, noteID, contentID, &input)
	if err != nil {
		h.handleError(w, r, err)
		return
	}

	middleware.JSON(w, r, http.StatusOK, content)
}

// DeleteContent handles deleting a content
// DELETE /notes/{noteId}/contents/{contentId}
func (h *Handler) DeleteContent(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	noteID := chi.URLParam(r, "noteId")
	contentID := chi.URLParam(r, "contentId")

	if err := h.service.DeleteContent(r.Context(), userID, noteID, contentID); err != nil {
		h.handleError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ReorderContents handles reordering contents
// PATCH /notes/{noteId}/contents/reorder
func (h *Handler) ReorderContents(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	noteID := chi.URLParam(r, "noteId")

	var input struct {
		ContentIDs []string `json:"contentIds"`
	}
	if !middleware.DecodeJSONBody(w, r, &input) {
		return
	}

	if err := h.service.ReorderContents(r.Context(), userID, noteID, input.ContentIDs); err != nil {
		h.handleError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ========== Storage Handlers ==========

// GetUploadURL handles getting presigned upload URL
// POST /notes/{noteId}/contents/upload-url
func (h *Handler) GetUploadURL(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	noteID := chi.URLParam(r, "noteId")

	var input note.UploadURLRequest
	if !middleware.DecodeJSONBody(w, r, &input) {
		return
	}

	// Validate input
	if input.FileName == "" {
		middleware.ValidationError(w, r, []middleware.ErrorDetail{{Field: "fileName", Message: "File name is required"}})
		return
	}
	if input.MimeType == "" {
		middleware.ValidationError(w, r, []middleware.ErrorDetail{{Field: "mimeType", Message: "MIME type is required"}})
		return
	}
	if input.FileSize <= 0 {
		middleware.ValidationError(w, r, []middleware.ErrorDetail{{Field: "fileSize", Message: "File size must be positive"}})
		return
	}

	resp, err := h.service.GetUploadURL(r.Context(), userID, noteID, &input)
	if err != nil {
		h.handleError(w, r, err)
		return
	}

	middleware.JSON(w, r, http.StatusOK, resp)
}

// GetDownloadURL handles getting presigned download URL
// GET /notes/{noteId}/contents/{contentId}/download-url
func (h *Handler) GetDownloadURL(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	noteID := chi.URLParam(r, "noteId")
	contentID := chi.URLParam(r, "contentId")

	downloadURL, err := h.service.GetDownloadURL(r.Context(), userID, noteID, contentID)
	if err != nil {
		h.handleError(w, r, err)
		return
	}

	middleware.JSON(w, r, http.StatusOK, map[string]string{"downloadUrl": downloadURL})
}
