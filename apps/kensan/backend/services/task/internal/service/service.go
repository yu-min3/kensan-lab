package service

import (
	"context"
	"fmt"

	task "github.com/kensan/backend/services/task/internal"
	"github.com/kensan/backend/services/task/internal/repository"
	"github.com/kensan/backend/shared/errors"
)

// Service-specific errors
var (
	ErrTaskNotFound                = errors.NotFound("task")
	ErrGoalNotFound                = errors.NotFound("goal")
	ErrMilestoneNotFound           = errors.NotFound("milestone")
	ErrTagNotFound                 = errors.NotFound("tag")
	ErrTagAlreadyExists            = repository.ErrTagAlreadyExists
	ErrEntityMemoNotFound          = errors.NotFound("entity memo")
	ErrTodoNotFound                = errors.NotFound("todo")
	ErrTodoCompletionAlreadyExists = repository.ErrTodoCompletionAlreadyExists
	ErrInvalidStatus               = fmt.Errorf("invalid status: %w", errors.ErrInvalidInput)
	ErrInvalidEntityType           = fmt.Errorf("invalid entity type: %w", errors.ErrInvalidInput)
	ErrInvalidFrequency            = fmt.Errorf("invalid frequency: %w", errors.ErrInvalidInput)
	ErrInvalidInput                = errors.ErrInvalidInput
)

// Service handles business logic for tasks, goals, milestones, and tags
type Service struct {
	repo repository.Repository
}

// NewService creates a new task service
func NewService(repo repository.Repository) *Service {
	return &Service{repo: repo}
}

// ========== Task Operations ==========

// ListTasks returns all tasks for a user with optional filters
func (s *Service) ListTasks(ctx context.Context, userID string, filter task.TaskFilter) ([]task.Task, error) {
	// If milestone_id is specified, verify it belongs to the user
	if filter.MilestoneID != nil {
		milestone, err := s.repo.GetMilestoneByID(ctx, userID, *filter.MilestoneID)
		if err != nil {
			return nil, err
		}
		if milestone == nil {
			return nil, ErrMilestoneNotFound
		}
	}

	// If parent_id is specified, verify it belongs to the user
	if filter.ParentTaskID != nil {
		parentTask, err := s.repo.GetTaskByID(ctx, userID, *filter.ParentTaskID)
		if err != nil {
			return nil, err
		}
		if parentTask == nil {
			return nil, ErrTaskNotFound
		}
	}

	tasks, err := s.repo.ListTasks(ctx, userID, filter)
	if err != nil {
		return nil, err
	}

	if tasks == nil {
		return []task.Task{}, nil
	}

	return tasks, nil
}

// GetTask returns a task by ID
func (s *Service) GetTask(ctx context.Context, userID, taskID string) (*task.Task, error) {
	t, err := s.repo.GetTaskByID(ctx, userID, taskID)
	if err != nil {
		return nil, err
	}

	if t == nil {
		return nil, ErrTaskNotFound
	}

	return t, nil
}

// CreateTask creates a new task
func (s *Service) CreateTask(ctx context.Context, userID string, input task.CreateTaskInput) (*task.Task, error) {
	if input.Name == "" {
		return nil, ErrInvalidInput
	}

	// If milestone_id is specified, verify it exists and belongs to user
	if input.MilestoneID != nil {
		milestone, err := s.repo.GetMilestoneByID(ctx, userID, *input.MilestoneID)
		if err != nil {
			return nil, err
		}
		if milestone == nil {
			return nil, ErrMilestoneNotFound
		}
	}

	// If parent task ID is specified, verify it exists and belongs to user
	if input.ParentTaskID != nil {
		parentTask, err := s.repo.GetTaskByID(ctx, userID, *input.ParentTaskID)
		if err != nil {
			return nil, err
		}
		if parentTask == nil {
			return nil, ErrTaskNotFound
		}
	}

	return s.repo.CreateTask(ctx, userID, input)
}

// UpdateTask updates an existing task
func (s *Service) UpdateTask(ctx context.Context, userID, taskID string, input task.UpdateTaskInput) (*task.Task, error) {
	// Check if task exists and belongs to user
	existing, err := s.repo.GetTaskByID(ctx, userID, taskID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, ErrTaskNotFound
	}

	// If changing milestone, verify new milestone exists and belongs to user
	if input.MilestoneID != nil {
		milestone, err := s.repo.GetMilestoneByID(ctx, userID, *input.MilestoneID)
		if err != nil {
			return nil, err
		}
		if milestone == nil {
			return nil, ErrMilestoneNotFound
		}
	}

	// If changing parent task, verify it exists and belongs to user
	if input.ParentTaskID != nil {
		parentTask, err := s.repo.GetTaskByID(ctx, userID, *input.ParentTaskID)
		if err != nil {
			return nil, err
		}
		if parentTask == nil {
			return nil, ErrTaskNotFound
		}

		// Prevent circular reference
		if *input.ParentTaskID == taskID {
			return nil, ErrInvalidInput
		}
	}

	t, err := s.repo.UpdateTask(ctx, userID, taskID, input)
	if err != nil {
		return nil, err
	}

	if t == nil {
		return nil, ErrTaskNotFound
	}

	return t, nil
}

// DeleteTask deletes a task
func (s *Service) DeleteTask(ctx context.Context, userID, taskID string) error {
	// Check if task exists and belongs to user
	existing, err := s.repo.GetTaskByID(ctx, userID, taskID)
	if err != nil {
		return err
	}
	if existing == nil {
		return ErrTaskNotFound
	}

	return s.repo.DeleteTask(ctx, userID, taskID)
}

// ToggleTaskComplete toggles the completed status of a task
func (s *Service) ToggleTaskComplete(ctx context.Context, userID, taskID string) (*task.Task, error) {
	// Check if task exists and belongs to user
	existing, err := s.repo.GetTaskByID(ctx, userID, taskID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, ErrTaskNotFound
	}

	t, err := s.repo.ToggleTaskComplete(ctx, userID, taskID)
	if err != nil {
		return nil, err
	}

	if t == nil {
		return nil, ErrTaskNotFound
	}

	return t, nil
}

// ReorderTasks updates the sort order of tasks
func (s *Service) ReorderTasks(ctx context.Context, userID string, taskIDs []string) ([]task.Task, error) {
	if len(taskIDs) == 0 {
		return []task.Task{}, nil
	}

	return s.repo.ReorderTasks(ctx, userID, taskIDs)
}

// BulkDeleteTasks deletes multiple tasks
func (s *Service) BulkDeleteTasks(ctx context.Context, userID string, taskIDs []string) error {
	if len(taskIDs) == 0 {
		return nil
	}

	return s.repo.BulkDeleteTasks(ctx, userID, taskIDs)
}

// BulkCompleteTasks updates the completed status of multiple tasks
func (s *Service) BulkCompleteTasks(ctx context.Context, userID string, taskIDs []string, completed bool) ([]task.Task, error) {
	if len(taskIDs) == 0 {
		return []task.Task{}, nil
	}

	return s.repo.BulkCompleteTasks(ctx, userID, taskIDs, completed)
}

// GetChildTasks returns all child tasks for a given parent task
func (s *Service) GetChildTasks(ctx context.Context, userID, parentTaskID string) ([]task.Task, error) {
	// Check if parent task exists and belongs to user
	parentTask, err := s.repo.GetTaskByID(ctx, userID, parentTaskID)
	if err != nil {
		return nil, err
	}
	if parentTask == nil {
		return nil, ErrTaskNotFound
	}

	tasks, err := s.repo.GetChildTasks(ctx, userID, parentTaskID)
	if err != nil {
		return nil, err
	}

	if tasks == nil {
		return []task.Task{}, nil
	}

	return tasks, nil
}

// ========== Goal Operations ==========

// ListGoals returns all goals for a user
func (s *Service) ListGoals(ctx context.Context, userID string, filter task.GoalFilter) ([]task.Goal, error) {
	goals, err := s.repo.ListGoals(ctx, userID, filter)
	if err != nil {
		return nil, err
	}
	if goals == nil {
		return []task.Goal{}, nil
	}
	return goals, nil
}

// GetGoal returns a goal by ID
func (s *Service) GetGoal(ctx context.Context, userID, goalID string) (*task.Goal, error) {
	goal, err := s.repo.GetGoalByID(ctx, userID, goalID)
	if err != nil {
		return nil, err
	}
	if goal == nil {
		return nil, ErrGoalNotFound
	}
	return goal, nil
}

// CreateGoal creates a new goal
func (s *Service) CreateGoal(ctx context.Context, userID string, input task.CreateGoalInput) (*task.Goal, error) {
	if input.Name == "" {
		return nil, ErrInvalidInput
	}
	if input.Color == "" {
		input.Color = "#0EA5E9" // default color
	}
	return s.repo.CreateGoal(ctx, userID, input)
}

// UpdateGoal updates an existing goal
func (s *Service) UpdateGoal(ctx context.Context, userID, goalID string, input task.UpdateGoalInput) (*task.Goal, error) {
	existing, err := s.repo.GetGoalByID(ctx, userID, goalID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, ErrGoalNotFound
	}
	goal, err := s.repo.UpdateGoal(ctx, userID, goalID, input)
	if err != nil {
		return nil, err
	}
	if goal == nil {
		return nil, ErrGoalNotFound
	}
	return goal, nil
}

// DeleteGoal deletes a goal
func (s *Service) DeleteGoal(ctx context.Context, userID, goalID string) error {
	existing, err := s.repo.GetGoalByID(ctx, userID, goalID)
	if err != nil {
		return err
	}
	if existing == nil {
		return ErrGoalNotFound
	}
	return s.repo.DeleteGoal(ctx, userID, goalID)
}

// ReorderGoals updates the sort order of goals
func (s *Service) ReorderGoals(ctx context.Context, userID string, goalIDs []string) ([]task.Goal, error) {
	if len(goalIDs) == 0 {
		return []task.Goal{}, nil
	}

	return s.repo.ReorderGoals(ctx, userID, goalIDs)
}

// ========== Milestone Operations ==========

// ListMilestones returns all milestones for a user
func (s *Service) ListMilestones(ctx context.Context, userID string, filter task.MilestoneFilter) ([]task.Milestone, error) {
	// Verify goal exists if specified
	if filter.GoalID != nil {
		goal, err := s.repo.GetGoalByID(ctx, userID, *filter.GoalID)
		if err != nil {
			return nil, err
		}
		if goal == nil {
			return nil, ErrGoalNotFound
		}
	}
	if filter.Status != nil && !filter.Status.IsValid() {
		return nil, ErrInvalidStatus
	}
	milestones, err := s.repo.ListMilestones(ctx, userID, filter)
	if err != nil {
		return nil, err
	}
	if milestones == nil {
		return []task.Milestone{}, nil
	}
	return milestones, nil
}

// GetMilestone returns a milestone by ID
func (s *Service) GetMilestone(ctx context.Context, userID, milestoneID string) (*task.Milestone, error) {
	milestone, err := s.repo.GetMilestoneByID(ctx, userID, milestoneID)
	if err != nil {
		return nil, err
	}
	if milestone == nil {
		return nil, ErrMilestoneNotFound
	}
	return milestone, nil
}

// CreateMilestone creates a new milestone
func (s *Service) CreateMilestone(ctx context.Context, userID string, input task.CreateMilestoneInput) (*task.Milestone, error) {
	if input.Name == "" || input.GoalID == "" {
		return nil, ErrInvalidInput
	}
	// Verify goal exists
	goal, err := s.repo.GetGoalByID(ctx, userID, input.GoalID)
	if err != nil {
		return nil, err
	}
	if goal == nil {
		return nil, ErrGoalNotFound
	}
	return s.repo.CreateMilestone(ctx, userID, input)
}

// UpdateMilestone updates an existing milestone
func (s *Service) UpdateMilestone(ctx context.Context, userID, milestoneID string, input task.UpdateMilestoneInput) (*task.Milestone, error) {
	existing, err := s.repo.GetMilestoneByID(ctx, userID, milestoneID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, ErrMilestoneNotFound
	}
	// Verify new goal if specified
	if input.GoalID != nil {
		goal, err := s.repo.GetGoalByID(ctx, userID, *input.GoalID)
		if err != nil {
			return nil, err
		}
		if goal == nil {
			return nil, ErrGoalNotFound
		}
	}
	if input.Status != nil && !input.Status.IsValid() {
		return nil, ErrInvalidStatus
	}
	milestone, err := s.repo.UpdateMilestone(ctx, userID, milestoneID, input)
	if err != nil {
		return nil, err
	}
	if milestone == nil {
		return nil, ErrMilestoneNotFound
	}
	return milestone, nil
}

// DeleteMilestone deletes a milestone
func (s *Service) DeleteMilestone(ctx context.Context, userID, milestoneID string) error {
	existing, err := s.repo.GetMilestoneByID(ctx, userID, milestoneID)
	if err != nil {
		return err
	}
	if existing == nil {
		return ErrMilestoneNotFound
	}
	return s.repo.DeleteMilestone(ctx, userID, milestoneID)
}

// ========== Tag Operations ==========

// ListTags returns all task-type tags for a user
func (s *Service) ListTags(ctx context.Context, userID string) ([]task.Tag, error) {
	tags, err := s.repo.ListTags(ctx, userID)
	if err != nil {
		return nil, err
	}
	if tags == nil {
		return []task.Tag{}, nil
	}
	return tags, nil
}

// ListNoteTags returns all note-type tags for a user
func (s *Service) ListNoteTags(ctx context.Context, userID string) ([]task.Tag, error) {
	tags, err := s.repo.ListNoteTags(ctx, userID)
	if err != nil {
		return nil, err
	}
	if tags == nil {
		return []task.Tag{}, nil
	}
	return tags, nil
}

// GetTag returns a tag by ID
func (s *Service) GetTag(ctx context.Context, userID, tagID string) (*task.Tag, error) {
	tag, err := s.repo.GetTagByID(ctx, userID, tagID)
	if err != nil {
		return nil, err
	}
	if tag == nil {
		return nil, ErrTagNotFound
	}
	return tag, nil
}

// CreateTag creates a new task-type tag
func (s *Service) CreateTag(ctx context.Context, userID string, input task.CreateTagInput) (*task.Tag, error) {
	if input.Name == "" {
		return nil, ErrInvalidInput
	}
	if input.Color == "" {
		input.Color = "#6B7280" // default color
	}
	if input.Category == "" {
		input.Category = "general"
	}
	if !task.IsValidTagCategory(input.Category) {
		return nil, ErrInvalidInput
	}
	return s.repo.CreateTag(ctx, userID, input)
}

// CreateNoteTag creates a new note-type tag
func (s *Service) CreateNoteTag(ctx context.Context, userID string, input task.CreateTagInput) (*task.Tag, error) {
	if input.Name == "" {
		return nil, ErrInvalidInput
	}
	if input.Color == "" {
		input.Color = "#6B7280" // default color
	}
	if input.Category == "" {
		input.Category = "general"
	}
	if !task.IsValidTagCategory(input.Category) {
		return nil, ErrInvalidInput
	}
	return s.repo.CreateNoteTag(ctx, userID, input)
}

// UpdateTag updates an existing tag
func (s *Service) UpdateTag(ctx context.Context, userID, tagID string, input task.UpdateTagInput) (*task.Tag, error) {
	if input.Category != nil && !task.IsValidTagCategory(*input.Category) {
		return nil, ErrInvalidInput
	}
	existing, err := s.repo.GetTagByID(ctx, userID, tagID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, ErrTagNotFound
	}
	tag, err := s.repo.UpdateTag(ctx, userID, tagID, input)
	if err != nil {
		return nil, err
	}
	if tag == nil {
		return nil, ErrTagNotFound
	}
	return tag, nil
}

// DeleteTag deletes a tag
func (s *Service) DeleteTag(ctx context.Context, userID, tagID string) error {
	existing, err := s.repo.GetTagByID(ctx, userID, tagID)
	if err != nil {
		return err
	}
	if existing == nil {
		return ErrTagNotFound
	}
	return s.repo.DeleteTag(ctx, userID, tagID)
}

// ========== EntityMemo Operations ==========

// ListEntityMemos returns all entity memos for a user with optional filters
func (s *Service) ListEntityMemos(ctx context.Context, userID string, filter task.EntityMemoFilter) ([]task.EntityMemo, error) {
	// Validate entity type if specified
	if filter.EntityType != nil && !filter.EntityType.IsValid() {
		return nil, ErrInvalidEntityType
	}

	// If entity_id is specified, verify entity exists
	if filter.EntityID != nil && filter.EntityType != nil {
		exists, err := s.entityExists(ctx, userID, *filter.EntityType, *filter.EntityID)
		if err != nil {
			return nil, err
		}
		if !exists {
			return nil, s.entityNotFoundError(*filter.EntityType)
		}
	}

	memos, err := s.repo.ListEntityMemos(ctx, userID, filter)
	if err != nil {
		return nil, err
	}
	if memos == nil {
		return []task.EntityMemo{}, nil
	}
	return memos, nil
}

// GetEntityMemo returns an entity memo by ID
func (s *Service) GetEntityMemo(ctx context.Context, userID, memoID string) (*task.EntityMemo, error) {
	memo, err := s.repo.GetEntityMemoByID(ctx, userID, memoID)
	if err != nil {
		return nil, err
	}
	if memo == nil {
		return nil, ErrEntityMemoNotFound
	}
	return memo, nil
}

// CreateEntityMemo creates a new entity memo
func (s *Service) CreateEntityMemo(ctx context.Context, userID string, input task.CreateEntityMemoInput) (*task.EntityMemo, error) {
	if input.Content == "" || input.EntityID == "" {
		return nil, ErrInvalidInput
	}
	if input.EntityType == "" || !input.EntityType.IsValid() {
		return nil, ErrInvalidEntityType
	}

	// Verify entity exists
	exists, err := s.entityExists(ctx, userID, input.EntityType, input.EntityID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, s.entityNotFoundError(input.EntityType)
	}

	return s.repo.CreateEntityMemo(ctx, userID, input)
}

// UpdateEntityMemo updates an existing entity memo
func (s *Service) UpdateEntityMemo(ctx context.Context, userID, memoID string, input task.UpdateEntityMemoInput) (*task.EntityMemo, error) {
	existing, err := s.repo.GetEntityMemoByID(ctx, userID, memoID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, ErrEntityMemoNotFound
	}

	memo, err := s.repo.UpdateEntityMemo(ctx, userID, memoID, input)
	if err != nil {
		return nil, err
	}
	if memo == nil {
		return nil, ErrEntityMemoNotFound
	}
	return memo, nil
}

// DeleteEntityMemo deletes an entity memo
func (s *Service) DeleteEntityMemo(ctx context.Context, userID, memoID string) error {
	existing, err := s.repo.GetEntityMemoByID(ctx, userID, memoID)
	if err != nil {
		return err
	}
	if existing == nil {
		return ErrEntityMemoNotFound
	}
	return s.repo.DeleteEntityMemo(ctx, userID, memoID)
}

// entityExists checks if an entity exists
func (s *Service) entityExists(ctx context.Context, userID string, entityType task.EntityType, entityID string) (bool, error) {
	switch entityType {
	case task.EntityTypeGoal:
		goal, err := s.repo.GetGoalByID(ctx, userID, entityID)
		if err != nil {
			return false, err
		}
		return goal != nil, nil
	case task.EntityTypeMilestone:
		milestone, err := s.repo.GetMilestoneByID(ctx, userID, entityID)
		if err != nil {
			return false, err
		}
		return milestone != nil, nil
	case task.EntityTypeTask:
		t, err := s.repo.GetTaskByID(ctx, userID, entityID)
		if err != nil {
			return false, err
		}
		return t != nil, nil
	default:
		return false, ErrInvalidEntityType
	}
}

// entityNotFoundError returns the appropriate error for an entity type
func (s *Service) entityNotFoundError(entityType task.EntityType) error {
	switch entityType {
	case task.EntityTypeGoal:
		return ErrGoalNotFound
	case task.EntityTypeMilestone:
		return ErrMilestoneNotFound
	case task.EntityTypeTask:
		return ErrTaskNotFound
	default:
		return ErrInvalidEntityType
	}
}

// ========== Todo Operations ==========

// ListTodos returns all todos for a user
func (s *Service) ListTodos(ctx context.Context, userID string, filter task.TodoFilter) ([]task.Todo, error) {
	todos, err := s.repo.ListTodos(ctx, userID, filter)
	if err != nil {
		return nil, err
	}
	if todos == nil {
		return []task.Todo{}, nil
	}
	return todos, nil
}

// ListTodosWithStatus returns todos with completion status for a specific date
func (s *Service) ListTodosWithStatus(ctx context.Context, userID string, date string) ([]task.TodoWithStatus, error) {
	if date == "" {
		return nil, ErrInvalidInput
	}
	todos, err := s.repo.ListTodosWithStatus(ctx, userID, date)
	if err != nil {
		return nil, err
	}
	if todos == nil {
		return []task.TodoWithStatus{}, nil
	}
	return todos, nil
}

// GetTodo returns a todo by ID
func (s *Service) GetTodo(ctx context.Context, userID, todoID string) (*task.Todo, error) {
	todo, err := s.repo.GetTodoByID(ctx, userID, todoID)
	if err != nil {
		return nil, err
	}
	if todo == nil {
		return nil, ErrTodoNotFound
	}
	return todo, nil
}

// CreateTodo creates a new todo
func (s *Service) CreateTodo(ctx context.Context, userID string, input task.CreateTodoInput) (*task.Todo, error) {
	if input.Name == "" {
		return nil, ErrInvalidInput
	}
	if input.Frequency != nil && !input.Frequency.IsValid() {
		return nil, ErrInvalidFrequency
	}
	return s.repo.CreateTodo(ctx, userID, input)
}

// UpdateTodo updates an existing todo
func (s *Service) UpdateTodo(ctx context.Context, userID, todoID string, input task.UpdateTodoInput) (*task.Todo, error) {
	existing, err := s.repo.GetTodoByID(ctx, userID, todoID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, ErrTodoNotFound
	}
	if input.Frequency != nil && !input.Frequency.IsValid() {
		return nil, ErrInvalidFrequency
	}
	todo, err := s.repo.UpdateTodo(ctx, userID, todoID, input)
	if err != nil {
		return nil, err
	}
	if todo == nil {
		return nil, ErrTodoNotFound
	}
	return todo, nil
}

// DeleteTodo deletes a todo
func (s *Service) DeleteTodo(ctx context.Context, userID, todoID string) error {
	existing, err := s.repo.GetTodoByID(ctx, userID, todoID)
	if err != nil {
		return err
	}
	if existing == nil {
		return ErrTodoNotFound
	}
	return s.repo.DeleteTodo(ctx, userID, todoID)
}

// ToggleTodoComplete toggles the completion status of a todo for a specific date
func (s *Service) ToggleTodoComplete(ctx context.Context, userID, todoID, date string) (*task.TodoWithStatus, error) {
	// Check if todo exists and belongs to user
	todo, err := s.repo.GetTodoByID(ctx, userID, todoID)
	if err != nil {
		return nil, err
	}
	if todo == nil {
		return nil, ErrTodoNotFound
	}

	// Check if already completed today
	completion, err := s.repo.GetTodoCompletion(ctx, userID, todoID, date)
	if err != nil {
		return nil, err
	}

	if completion != nil {
		// Already completed, uncomplete it
		err = s.repo.DeleteTodoCompletion(ctx, userID, todoID, date)
		if err != nil {
			return nil, err
		}
	} else {
		// Not completed, complete it
		_, err = s.repo.CreateTodoCompletion(ctx, userID, todoID, date)
		if err != nil {
			return nil, err
		}
	}

	// Get updated status
	todos, err := s.repo.ListTodosWithStatus(ctx, userID, date)
	if err != nil {
		return nil, err
	}

	for _, t := range todos {
		if t.ID == todoID {
			return &t, nil
		}
	}

	// Build response manually if not in the list
	return &task.TodoWithStatus{
		Todo:           *todo,
		CompletedToday: completion == nil, // toggled state
	}, nil
}
