package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/kensan/backend/services/task/internal"
	"github.com/kensan/backend/services/task/internal/service"
	"github.com/kensan/backend/shared/middleware"
	"log/slog"
)

// Handler handles HTTP requests for tasks, goals, milestones, and tags
type Handler struct {
	service service.FullService
}

// NewHandler creates a new task handler
func NewHandler(svc service.FullService) *Handler {
	return &Handler{service: svc}
}

// handleServiceError maps service errors to HTTP responses.
// Returns true if an error was handled, false if no error.
// If defaultMsg is empty, a generic message is used.
func (h *Handler) handleServiceError(w http.ResponseWriter, r *http.Request, err error, defaultMsg string) bool {
	if err == nil {
		return false
	}

	// Not found errors
	if errors.Is(err, service.ErrTaskNotFound) {
		middleware.Error(w, r, http.StatusNotFound, "TASK_NOT_FOUND", "Task not found")
		return true
	}
	if errors.Is(err, service.ErrGoalNotFound) {
		middleware.Error(w, r, http.StatusNotFound, "GOAL_NOT_FOUND", "Goal not found")
		return true
	}
	if errors.Is(err, service.ErrMilestoneNotFound) {
		middleware.Error(w, r, http.StatusNotFound, "MILESTONE_NOT_FOUND", "Milestone not found")
		return true
	}
	if errors.Is(err, service.ErrTagNotFound) {
		middleware.Error(w, r, http.StatusNotFound, "TAG_NOT_FOUND", "Tag not found")
		return true
	}
	if errors.Is(err, service.ErrEntityMemoNotFound) {
		middleware.Error(w, r, http.StatusNotFound, "ENTITY_MEMO_NOT_FOUND", "Entity memo not found")
		return true
	}
	if errors.Is(err, service.ErrTodoNotFound) {
		middleware.Error(w, r, http.StatusNotFound, "TODO_NOT_FOUND", "Todo not found")
		return true
	}

	// Already exists errors (conflict)
	if errors.Is(err, service.ErrTagAlreadyExists) {
		middleware.Error(w, r, http.StatusConflict, "TAG_ALREADY_EXISTS", "A tag with this name already exists")
		return true
	}
	if errors.Is(err, service.ErrTodoCompletionAlreadyExists) {
		middleware.Error(w, r, http.StatusConflict, "TODO_COMPLETION_ALREADY_EXISTS", "This todo is already marked as completed for this date")
		return true
	}

	// Validation errors
	if errors.Is(err, service.ErrInvalidInput) {
		middleware.Error(w, r, http.StatusBadRequest, "INVALID_INPUT", "Invalid input")
		return true
	}
	if errors.Is(err, service.ErrInvalidStatus) {
		middleware.Error(w, r, http.StatusBadRequest, "INVALID_STATUS", "Invalid status")
		return true
	}
	if errors.Is(err, service.ErrInvalidEntityType) {
		middleware.Error(w, r, http.StatusBadRequest, "INVALID_ENTITY_TYPE", "Invalid entity type")
		return true
	}
	if errors.Is(err, service.ErrInvalidFrequency) {
		middleware.Error(w, r, http.StatusBadRequest, "INVALID_FREQUENCY", "Invalid frequency")
		return true
	}

	if middleware.HandleDBSchemaError(w, r, err) {
		return true
	}

	// Default: internal error
	slog.ErrorContext(r.Context(), "Unhandled error in task-service", "error", err, "request_id", middleware.GetRequestID(r.Context()))
	if defaultMsg == "" {
		defaultMsg = "An internal error occurred"
	}
	middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", defaultMsg)
	return true
}

// RegisterRoutes registers the task routes
func (h *Handler) RegisterRoutes(r chi.Router) {
	// Goal routes
	r.Route("/goals", func(r chi.Router) {
		r.Get("/", h.ListGoals)
		r.Post("/", h.CreateGoal)
		r.Post("/reorder", h.ReorderGoals)
		r.Get("/{goalId}", h.GetGoal)
		r.Put("/{goalId}", h.UpdateGoal)
		r.Delete("/{goalId}", h.DeleteGoal)
	})

	// Milestone routes
	r.Route("/milestones", func(r chi.Router) {
		r.Get("/", h.ListMilestones)
		r.Post("/", h.CreateMilestone)
		r.Get("/{milestoneId}", h.GetMilestone)
		r.Put("/{milestoneId}", h.UpdateMilestone)
		r.Delete("/{milestoneId}", h.DeleteMilestone)
	})

	// Tag routes (task-type tags)
	r.Route("/tags", func(r chi.Router) {
		r.Get("/", h.ListTags)
		r.Post("/", h.CreateTag)
		r.Get("/{tagId}", h.GetTag)
		r.Put("/{tagId}", h.UpdateTag)
		r.Delete("/{tagId}", h.DeleteTag)
	})

	// Note tag routes (note-type tags)
	r.Route("/note-tags", func(r chi.Router) {
		r.Get("/", h.ListNoteTags)
		r.Post("/", h.CreateNoteTag)
		r.Put("/{tagId}", h.UpdateTag)
		r.Delete("/{tagId}", h.DeleteTag)
	})

	// Task routes
	r.Route("/tasks", func(r chi.Router) {
		r.Get("/", h.ListTasks)
		r.Post("/", h.CreateTask)
		r.Post("/reorder", h.ReorderTasks)
		r.Post("/bulk-delete", h.BulkDeleteTasks)
		r.Post("/bulk-complete", h.BulkCompleteTasks)
		r.Get("/{taskId}", h.GetTask)
		r.Put("/{taskId}", h.UpdateTask)
		r.Patch("/{taskId}/complete", h.ToggleTaskComplete)
		r.Delete("/{taskId}", h.DeleteTask)
	})

	// EntityMemo routes
	r.Route("/entity-memos", func(r chi.Router) {
		r.Get("/", h.ListEntityMemos)
		r.Post("/", h.CreateEntityMemo)
		r.Get("/{memoId}", h.GetEntityMemo)
		r.Put("/{memoId}", h.UpdateEntityMemo)
		r.Delete("/{memoId}", h.DeleteEntityMemo)
	})

	// Todo routes
	r.Route("/todos", func(r chi.Router) {
		r.Get("/", h.ListTodos)
		r.Post("/", h.CreateTodo)
		r.Get("/{todoId}", h.GetTodo)
		r.Put("/{todoId}", h.UpdateTodo)
		r.Delete("/{todoId}", h.DeleteTodo)
		r.Patch("/{todoId}/complete", h.ToggleTodoComplete)
	})
}

// ========== Task Handlers ==========

// ListTasks handles GET /tasks
func (h *Handler) ListTasks(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	filter := task.TaskFilter{}

	// Parse milestone_id filter
	if milestoneID := r.URL.Query().Get("milestone_id"); milestoneID != "" {
		filter.MilestoneID = &milestoneID
	}

	// Parse completed filter
	if completed := r.URL.Query().Get("completed"); completed != "" {
		if b, err := strconv.ParseBool(completed); err == nil {
			filter.Completed = &b
		}
	}

	// Parse parent_id filter
	if parentID := r.URL.Query().Get("parent_id"); parentID != "" {
		filter.ParentTaskID = &parentID
	}

	tasks, err := h.service.ListTasks(r.Context(), userID, filter)
	if err != nil {
		if errors.Is(err, service.ErrMilestoneNotFound) {
			middleware.Error(w, r, http.StatusNotFound, "MILESTONE_NOT_FOUND", "Milestone not found")
			return
		}
		if errors.Is(err, service.ErrTaskNotFound) {
			middleware.Error(w, r, http.StatusNotFound, "TASK_NOT_FOUND", "Parent task not found")
			return
		}
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list tasks")
		return
	}

	middleware.JSON(w, r, http.StatusOK, tasks)
}

// GetTask handles GET /tasks/{taskId}
func (h *Handler) GetTask(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	taskID := chi.URLParam(r, "taskId")

	t, err := h.service.GetTask(r.Context(), userID, taskID)
	if h.handleServiceError(w, r, err, "Failed to get task") {
		return
	}

	middleware.JSON(w, r, http.StatusOK, t)
}

// CreateTask handles POST /tasks
func (h *Handler) CreateTask(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var input task.CreateTaskInput
	if !middleware.DecodeJSONBody(w, r, &input) {
		return
	}

	t, err := h.service.CreateTask(r.Context(), userID, input)
	if h.handleServiceError(w, r, err, "Failed to create task") {
		return
	}

	middleware.JSON(w, r, http.StatusCreated, t)
}

// UpdateTask handles PUT /tasks/{taskId}
func (h *Handler) UpdateTask(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	taskID := chi.URLParam(r, "taskId")

	var input task.UpdateTaskInput
	if !middleware.DecodeJSONBody(w, r, &input) {
		return
	}

	t, err := h.service.UpdateTask(r.Context(), userID, taskID, input)
	if h.handleServiceError(w, r, err, "Failed to update task") {
		return
	}

	middleware.JSON(w, r, http.StatusOK, t)
}

// ToggleTaskComplete handles PATCH /tasks/{taskId}/complete
func (h *Handler) ToggleTaskComplete(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	taskID := chi.URLParam(r, "taskId")

	t, err := h.service.ToggleTaskComplete(r.Context(), userID, taskID)
	if h.handleServiceError(w, r, err, "Failed to toggle task complete") {
		return
	}

	middleware.JSON(w, r, http.StatusOK, t)
}

// DeleteTask handles DELETE /tasks/{taskId}
func (h *Handler) DeleteTask(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	taskID := chi.URLParam(r, "taskId")

	err := h.service.DeleteTask(r.Context(), userID, taskID)
	if h.handleServiceError(w, r, err, "Failed to delete task") {
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ReorderTasks handles POST /tasks/reorder
func (h *Handler) ReorderTasks(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var input struct {
		TaskIDs []string `json:"taskIds"`
	}
	if !middleware.DecodeJSONBody(w, r, &input) {
		return
	}

	if len(input.TaskIDs) == 0 {
		middleware.ValidationError(w, r, []middleware.ErrorDetail{
			{Field: "taskIds", Message: "Task IDs are required"},
		})
		return
	}

	tasks, err := h.service.ReorderTasks(r.Context(), userID, input.TaskIDs)
	if err != nil {
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to reorder tasks")
		return
	}

	middleware.JSON(w, r, http.StatusOK, tasks)
}

// BulkDeleteTasks handles POST /tasks/bulk-delete
func (h *Handler) BulkDeleteTasks(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var input struct {
		TaskIDs []string `json:"taskIds"`
	}
	if !middleware.DecodeJSONBody(w, r, &input) {
		return
	}

	if len(input.TaskIDs) == 0 {
		middleware.ValidationError(w, r, []middleware.ErrorDetail{
			{Field: "taskIds", Message: "Task IDs are required"},
		})
		return
	}

	err := h.service.BulkDeleteTasks(r.Context(), userID, input.TaskIDs)
	if err != nil {
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to bulk delete tasks")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// BulkCompleteTasks handles POST /tasks/bulk-complete
func (h *Handler) BulkCompleteTasks(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var input struct {
		TaskIDs   []string `json:"taskIds"`
		Completed bool     `json:"completed"`
	}
	if !middleware.DecodeJSONBody(w, r, &input) {
		return
	}

	if len(input.TaskIDs) == 0 {
		middleware.ValidationError(w, r, []middleware.ErrorDetail{
			{Field: "taskIds", Message: "Task IDs are required"},
		})
		return
	}

	tasks, err := h.service.BulkCompleteTasks(r.Context(), userID, input.TaskIDs, input.Completed)
	if err != nil {
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to bulk complete tasks")
		return
	}

	middleware.JSON(w, r, http.StatusOK, tasks)
}

// ========== Goal Handlers ==========

// ListGoals handles GET /goals
func (h *Handler) ListGoals(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	filter := task.GoalFilter{}

	// Parse status filter
	if status := r.URL.Query().Get("status"); status != "" {
		s := task.GoalStatus(status)
		if s.IsValid() {
			filter.Status = &s
		}
	}

	goals, err := h.service.ListGoals(r.Context(), userID, filter)
	if err != nil {
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list goals")
		return
	}

	middleware.JSON(w, r, http.StatusOK, goals)
}

// GetGoal handles GET /goals/{goalId}
func (h *Handler) GetGoal(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	goalID := chi.URLParam(r, "goalId")

	goal, err := h.service.GetGoal(r.Context(), userID, goalID)
	if h.handleServiceError(w, r, err, "Failed to get goal") {
		return
	}

	middleware.JSON(w, r, http.StatusOK, goal)
}

// CreateGoal handles POST /goals
func (h *Handler) CreateGoal(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var input task.CreateGoalInput
	if !middleware.DecodeJSONBody(w, r, &input) {
		return
	}

	goal, err := h.service.CreateGoal(r.Context(), userID, input)
	if h.handleServiceError(w, r, err, "Failed to create goal") {
		return
	}

	middleware.JSON(w, r, http.StatusCreated, goal)
}

// UpdateGoal handles PUT /goals/{goalId}
func (h *Handler) UpdateGoal(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	goalID := chi.URLParam(r, "goalId")

	var input task.UpdateGoalInput
	if !middleware.DecodeJSONBody(w, r, &input) {
		return
	}

	goal, err := h.service.UpdateGoal(r.Context(), userID, goalID, input)
	if h.handleServiceError(w, r, err, "Failed to update goal") {
		return
	}

	middleware.JSON(w, r, http.StatusOK, goal)
}

// DeleteGoal handles DELETE /goals/{goalId}
func (h *Handler) DeleteGoal(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	goalID := chi.URLParam(r, "goalId")

	err := h.service.DeleteGoal(r.Context(), userID, goalID)
	if h.handleServiceError(w, r, err, "Failed to delete goal") {
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ReorderGoals handles POST /goals/reorder
func (h *Handler) ReorderGoals(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var input struct {
		GoalIDs []string `json:"goalIds"`
	}
	if !middleware.DecodeJSONBody(w, r, &input) {
		return
	}

	if len(input.GoalIDs) == 0 {
		middleware.ValidationError(w, r, []middleware.ErrorDetail{
			{Field: "goalIds", Message: "Goal IDs are required"},
		})
		return
	}

	goals, err := h.service.ReorderGoals(r.Context(), userID, input.GoalIDs)
	if h.handleServiceError(w, r, err, "Failed to reorder goals") {
		return
	}

	middleware.JSON(w, r, http.StatusOK, goals)
}

// ========== Milestone Handlers ==========

// ListMilestones handles GET /milestones
func (h *Handler) ListMilestones(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	filter := task.MilestoneFilter{}

	// Parse goal_id filter
	if goalID := r.URL.Query().Get("goal_id"); goalID != "" {
		filter.GoalID = &goalID
	}

	// Parse status filter
	if status := r.URL.Query().Get("status"); status != "" {
		s := task.MilestoneStatus(status)
		filter.Status = &s
	}

	milestones, err := h.service.ListMilestones(r.Context(), userID, filter)
	if h.handleServiceError(w, r, err, "Failed to list milestones") {
		return
	}

	middleware.JSON(w, r, http.StatusOK, milestones)
}

// GetMilestone handles GET /milestones/{milestoneId}
func (h *Handler) GetMilestone(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	milestoneID := chi.URLParam(r, "milestoneId")

	milestone, err := h.service.GetMilestone(r.Context(), userID, milestoneID)
	if err != nil {
		if errors.Is(err, service.ErrMilestoneNotFound) {
			middleware.Error(w, r, http.StatusNotFound, "MILESTONE_NOT_FOUND", "Milestone not found")
			return
		}
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get milestone")
		return
	}

	middleware.JSON(w, r, http.StatusOK, milestone)
}

// CreateMilestone handles POST /milestones
func (h *Handler) CreateMilestone(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var input task.CreateMilestoneInput
	if !middleware.DecodeJSONBody(w, r, &input) {
		return
	}

	milestone, err := h.service.CreateMilestone(r.Context(), userID, input)
	if h.handleServiceError(w, r, err, "Failed to create milestone") {
		return
	}

	middleware.JSON(w, r, http.StatusCreated, milestone)
}

// UpdateMilestone handles PUT /milestones/{milestoneId}
func (h *Handler) UpdateMilestone(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	milestoneID := chi.URLParam(r, "milestoneId")

	var input task.UpdateMilestoneInput
	if !middleware.DecodeJSONBody(w, r, &input) {
		return
	}

	milestone, err := h.service.UpdateMilestone(r.Context(), userID, milestoneID, input)
	if err != nil {
		if errors.Is(err, service.ErrMilestoneNotFound) {
			middleware.Error(w, r, http.StatusNotFound, "MILESTONE_NOT_FOUND", "Milestone not found")
			return
		}
		if errors.Is(err, service.ErrGoalNotFound) {
			middleware.Error(w, r, http.StatusNotFound, "GOAL_NOT_FOUND", "Goal not found")
			return
		}
		if errors.Is(err, service.ErrInvalidStatus) {
			middleware.Error(w, r, http.StatusBadRequest, "INVALID_STATUS", "Invalid milestone status")
			return
		}
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update milestone")
		return
	}

	middleware.JSON(w, r, http.StatusOK, milestone)
}

// DeleteMilestone handles DELETE /milestones/{milestoneId}
func (h *Handler) DeleteMilestone(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	milestoneID := chi.URLParam(r, "milestoneId")

	err := h.service.DeleteMilestone(r.Context(), userID, milestoneID)
	if err != nil {
		if errors.Is(err, service.ErrMilestoneNotFound) {
			middleware.Error(w, r, http.StatusNotFound, "MILESTONE_NOT_FOUND", "Milestone not found")
			return
		}
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to delete milestone")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ========== Tag Handlers ==========

// ListTags handles GET /tags (task-type tags)
func (h *Handler) ListTags(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	tags, err := h.service.ListTags(r.Context(), userID)
	if err != nil {
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list tags")
		return
	}

	middleware.JSON(w, r, http.StatusOK, tags)
}

// ListNoteTags handles GET /note-tags (note-type tags)
func (h *Handler) ListNoteTags(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	tags, err := h.service.ListNoteTags(r.Context(), userID)
	if err != nil {
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list note tags")
		return
	}

	middleware.JSON(w, r, http.StatusOK, tags)
}

// GetTag handles GET /tags/{tagId}
func (h *Handler) GetTag(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	tagID := chi.URLParam(r, "tagId")

	tag, err := h.service.GetTag(r.Context(), userID, tagID)
	if err != nil {
		if errors.Is(err, service.ErrTagNotFound) {
			middleware.Error(w, r, http.StatusNotFound, "TAG_NOT_FOUND", "Tag not found")
			return
		}
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get tag")
		return
	}

	middleware.JSON(w, r, http.StatusOK, tag)
}

// CreateTag handles POST /tags
func (h *Handler) CreateTag(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var input task.CreateTagInput
	if !middleware.DecodeJSONBody(w, r, &input) {
		return
	}

	tag, err := h.service.CreateTag(r.Context(), userID, input)
	if h.handleServiceError(w, r, err, "Failed to create tag") {
		return
	}

	middleware.JSON(w, r, http.StatusCreated, tag)
}

// CreateNoteTag handles POST /note-tags (note-type tag)
func (h *Handler) CreateNoteTag(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var input task.CreateTagInput
	if !middleware.DecodeJSONBody(w, r, &input) {
		return
	}

	tag, err := h.service.CreateNoteTag(r.Context(), userID, input)
	if h.handleServiceError(w, r, err, "Failed to create note tag") {
		return
	}

	middleware.JSON(w, r, http.StatusCreated, tag)
}

// UpdateTag handles PUT /tags/{tagId}
func (h *Handler) UpdateTag(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	tagID := chi.URLParam(r, "tagId")

	var input task.UpdateTagInput
	if !middleware.DecodeJSONBody(w, r, &input) {
		return
	}

	tag, err := h.service.UpdateTag(r.Context(), userID, tagID, input)
	if err != nil {
		if errors.Is(err, service.ErrTagNotFound) {
			middleware.Error(w, r, http.StatusNotFound, "TAG_NOT_FOUND", "Tag not found")
			return
		}
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update tag")
		return
	}

	middleware.JSON(w, r, http.StatusOK, tag)
}

// DeleteTag handles DELETE /tags/{tagId}
func (h *Handler) DeleteTag(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	tagID := chi.URLParam(r, "tagId")

	err := h.service.DeleteTag(r.Context(), userID, tagID)
	if err != nil {
		if errors.Is(err, service.ErrTagNotFound) {
			middleware.Error(w, r, http.StatusNotFound, "TAG_NOT_FOUND", "Tag not found")
			return
		}
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to delete tag")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ========== EntityMemo Handlers ==========

// ListEntityMemos handles GET /entity-memos
func (h *Handler) ListEntityMemos(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	filter := task.EntityMemoFilter{}

	// Parse entity_type filter
	if entityType := r.URL.Query().Get("entity_type"); entityType != "" {
		et := task.EntityType(entityType)
		filter.EntityType = &et
	}

	// Parse entity_id filter
	if entityID := r.URL.Query().Get("entity_id"); entityID != "" {
		filter.EntityID = &entityID
	}

	// Parse pinned filter
	if pinned := r.URL.Query().Get("pinned"); pinned != "" {
		if b, err := strconv.ParseBool(pinned); err == nil {
			filter.Pinned = &b
		}
	}

	memos, err := h.service.ListEntityMemos(r.Context(), userID, filter)
	if err != nil {
		if errors.Is(err, service.ErrInvalidEntityType) {
			middleware.Error(w, r, http.StatusBadRequest, "INVALID_ENTITY_TYPE", "Invalid entity type")
			return
		}
		if errors.Is(err, service.ErrGoalNotFound) || errors.Is(err, service.ErrMilestoneNotFound) || errors.Is(err, service.ErrTaskNotFound) {
			middleware.Error(w, r, http.StatusNotFound, "ENTITY_NOT_FOUND", "Entity not found")
			return
		}
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list entity memos")
		return
	}

	middleware.JSON(w, r, http.StatusOK, memos)
}

// GetEntityMemo handles GET /entity-memos/{memoId}
func (h *Handler) GetEntityMemo(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	memoID := chi.URLParam(r, "memoId")

	memo, err := h.service.GetEntityMemo(r.Context(), userID, memoID)
	if err != nil {
		if errors.Is(err, service.ErrEntityMemoNotFound) {
			middleware.Error(w, r, http.StatusNotFound, "ENTITY_MEMO_NOT_FOUND", "Entity memo not found")
			return
		}
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get entity memo")
		return
	}

	middleware.JSON(w, r, http.StatusOK, memo)
}

// CreateEntityMemo handles POST /entity-memos
func (h *Handler) CreateEntityMemo(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var input task.CreateEntityMemoInput
	if !middleware.DecodeJSONBody(w, r, &input) {
		return
	}

	memo, err := h.service.CreateEntityMemo(r.Context(), userID, input)
	if h.handleServiceError(w, r, err, "Failed to create entity memo") {
		return
	}

	middleware.JSON(w, r, http.StatusCreated, memo)
}

// UpdateEntityMemo handles PUT /entity-memos/{memoId}
func (h *Handler) UpdateEntityMemo(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	memoID := chi.URLParam(r, "memoId")

	var input task.UpdateEntityMemoInput
	if !middleware.DecodeJSONBody(w, r, &input) {
		return
	}

	memo, err := h.service.UpdateEntityMemo(r.Context(), userID, memoID, input)
	if err != nil {
		if errors.Is(err, service.ErrEntityMemoNotFound) {
			middleware.Error(w, r, http.StatusNotFound, "ENTITY_MEMO_NOT_FOUND", "Entity memo not found")
			return
		}
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update entity memo")
		return
	}

	middleware.JSON(w, r, http.StatusOK, memo)
}

// DeleteEntityMemo handles DELETE /entity-memos/{memoId}
func (h *Handler) DeleteEntityMemo(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	memoID := chi.URLParam(r, "memoId")

	err := h.service.DeleteEntityMemo(r.Context(), userID, memoID)
	if err != nil {
		if errors.Is(err, service.ErrEntityMemoNotFound) {
			middleware.Error(w, r, http.StatusNotFound, "ENTITY_MEMO_NOT_FOUND", "Entity memo not found")
			return
		}
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to delete entity memo")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ========== Todo Handlers ==========

// ListTodos handles GET /todos
func (h *Handler) ListTodos(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	// Check if date is specified (for getting todos with status)
	if date := r.URL.Query().Get("date"); date != "" {
		todos, err := h.service.ListTodosWithStatus(r.Context(), userID, date)
		if err != nil {
			if errors.Is(err, service.ErrInvalidInput) {
				middleware.Error(w, r, http.StatusBadRequest, "INVALID_INPUT", "Invalid date format")
				return
			}
			middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list todos")
			return
		}
		middleware.JSON(w, r, http.StatusOK, todos)
		return
	}

	// Otherwise return all todos without status
	filter := task.TodoFilter{}

	// Parse enabled filter
	if enabled := r.URL.Query().Get("enabled"); enabled != "" {
		if b, err := strconv.ParseBool(enabled); err == nil {
			filter.Enabled = &b
		}
	}

	// Parse is_recurring filter
	if isRecurring := r.URL.Query().Get("is_recurring"); isRecurring != "" {
		if b, err := strconv.ParseBool(isRecurring); err == nil {
			filter.IsRecurring = &b
		}
	}

	todos, err := h.service.ListTodos(r.Context(), userID, filter)
	if err != nil {
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list todos")
		return
	}

	middleware.JSON(w, r, http.StatusOK, todos)
}

// GetTodo handles GET /todos/{todoId}
func (h *Handler) GetTodo(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	todoID := chi.URLParam(r, "todoId")

	todo, err := h.service.GetTodo(r.Context(), userID, todoID)
	if err != nil {
		if errors.Is(err, service.ErrTodoNotFound) {
			middleware.Error(w, r, http.StatusNotFound, "TODO_NOT_FOUND", "Todo not found")
			return
		}
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get todo")
		return
	}

	middleware.JSON(w, r, http.StatusOK, todo)
}

// CreateTodo handles POST /todos
func (h *Handler) CreateTodo(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var input task.CreateTodoInput
	if !middleware.DecodeJSONBody(w, r, &input) {
		return
	}

	todo, err := h.service.CreateTodo(r.Context(), userID, input)
	if h.handleServiceError(w, r, err, "Failed to create todo") {
		return
	}

	middleware.JSON(w, r, http.StatusCreated, todo)
}

// UpdateTodo handles PUT /todos/{todoId}
func (h *Handler) UpdateTodo(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	todoID := chi.URLParam(r, "todoId")

	var input task.UpdateTodoInput
	if !middleware.DecodeJSONBody(w, r, &input) {
		return
	}

	todo, err := h.service.UpdateTodo(r.Context(), userID, todoID, input)
	if err != nil {
		if errors.Is(err, service.ErrTodoNotFound) {
			middleware.Error(w, r, http.StatusNotFound, "TODO_NOT_FOUND", "Todo not found")
			return
		}
		if errors.Is(err, service.ErrInvalidFrequency) {
			middleware.Error(w, r, http.StatusBadRequest, "INVALID_FREQUENCY", "Invalid frequency")
			return
		}
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update todo")
		return
	}

	middleware.JSON(w, r, http.StatusOK, todo)
}

// DeleteTodo handles DELETE /todos/{todoId}
func (h *Handler) DeleteTodo(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	todoID := chi.URLParam(r, "todoId")

	err := h.service.DeleteTodo(r.Context(), userID, todoID)
	if err != nil {
		if errors.Is(err, service.ErrTodoNotFound) {
			middleware.Error(w, r, http.StatusNotFound, "TODO_NOT_FOUND", "Todo not found")
			return
		}
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to delete todo")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ToggleTodoComplete handles PATCH /todos/{todoId}/complete
func (h *Handler) ToggleTodoComplete(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	todoID := chi.URLParam(r, "todoId")

	// Get date from query parameter (required)
	date := r.URL.Query().Get("date")
	if date == "" {
		middleware.ValidationError(w, r, []middleware.ErrorDetail{
			{Field: "date", Message: "Date query parameter is required"},
		})
		return
	}

	todo, err := h.service.ToggleTodoComplete(r.Context(), userID, todoID, date)
	if err != nil {
		if errors.Is(err, service.ErrTodoNotFound) {
			middleware.Error(w, r, http.StatusNotFound, "TODO_NOT_FOUND", "Todo not found")
			return
		}
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to toggle todo complete")
		return
	}

	middleware.JSON(w, r, http.StatusOK, todo)
}
