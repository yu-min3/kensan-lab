package service

import (
	"context"

	task "github.com/kensan/backend/services/task/internal"
)

// GoalService defines the interface for goal-related operations
type GoalService interface {
	ListGoals(ctx context.Context, userID string, filter task.GoalFilter) ([]task.Goal, error)
	GetGoal(ctx context.Context, userID, goalID string) (*task.Goal, error)
	CreateGoal(ctx context.Context, userID string, input task.CreateGoalInput) (*task.Goal, error)
	UpdateGoal(ctx context.Context, userID, goalID string, input task.UpdateGoalInput) (*task.Goal, error)
	DeleteGoal(ctx context.Context, userID, goalID string) error
	ReorderGoals(ctx context.Context, userID string, goalIDs []string) ([]task.Goal, error)
}

// MilestoneService defines the interface for milestone-related operations
type MilestoneService interface {
	ListMilestones(ctx context.Context, userID string, filter task.MilestoneFilter) ([]task.Milestone, error)
	GetMilestone(ctx context.Context, userID, milestoneID string) (*task.Milestone, error)
	CreateMilestone(ctx context.Context, userID string, input task.CreateMilestoneInput) (*task.Milestone, error)
	UpdateMilestone(ctx context.Context, userID, milestoneID string, input task.UpdateMilestoneInput) (*task.Milestone, error)
	DeleteMilestone(ctx context.Context, userID, milestoneID string) error
}

// TagService defines the interface for tag-related operations
type TagService interface {
	ListTags(ctx context.Context, userID string) ([]task.Tag, error)
	ListNoteTags(ctx context.Context, userID string) ([]task.Tag, error)
	GetTag(ctx context.Context, userID, tagID string) (*task.Tag, error)
	CreateTag(ctx context.Context, userID string, input task.CreateTagInput) (*task.Tag, error)
	CreateNoteTag(ctx context.Context, userID string, input task.CreateTagInput) (*task.Tag, error)
	UpdateTag(ctx context.Context, userID, tagID string, input task.UpdateTagInput) (*task.Tag, error)
	DeleteTag(ctx context.Context, userID, tagID string) error
}

// TaskService defines the interface for task-related operations
type TaskService interface {
	ListTasks(ctx context.Context, userID string, filter task.TaskFilter) ([]task.Task, error)
	GetTask(ctx context.Context, userID, taskID string) (*task.Task, error)
	CreateTask(ctx context.Context, userID string, input task.CreateTaskInput) (*task.Task, error)
	UpdateTask(ctx context.Context, userID, taskID string, input task.UpdateTaskInput) (*task.Task, error)
	DeleteTask(ctx context.Context, userID, taskID string) error
	ToggleTaskComplete(ctx context.Context, userID, taskID string) (*task.Task, error)
	ReorderTasks(ctx context.Context, userID string, taskIDs []string) ([]task.Task, error)
	BulkDeleteTasks(ctx context.Context, userID string, taskIDs []string) error
	BulkCompleteTasks(ctx context.Context, userID string, taskIDs []string, completed bool) ([]task.Task, error)
	GetChildTasks(ctx context.Context, userID, parentTaskID string) ([]task.Task, error)
}

// EntityMemoService defines the interface for entity memo operations
type EntityMemoService interface {
	ListEntityMemos(ctx context.Context, userID string, filter task.EntityMemoFilter) ([]task.EntityMemo, error)
	GetEntityMemo(ctx context.Context, userID, memoID string) (*task.EntityMemo, error)
	CreateEntityMemo(ctx context.Context, userID string, input task.CreateEntityMemoInput) (*task.EntityMemo, error)
	UpdateEntityMemo(ctx context.Context, userID, memoID string, input task.UpdateEntityMemoInput) (*task.EntityMemo, error)
	DeleteEntityMemo(ctx context.Context, userID, memoID string) error
}

// TodoService defines the interface for todo-related operations
type TodoService interface {
	ListTodos(ctx context.Context, userID string, filter task.TodoFilter) ([]task.Todo, error)
	ListTodosWithStatus(ctx context.Context, userID string, date string) ([]task.TodoWithStatus, error)
	GetTodo(ctx context.Context, userID, todoID string) (*task.Todo, error)
	CreateTodo(ctx context.Context, userID string, input task.CreateTodoInput) (*task.Todo, error)
	UpdateTodo(ctx context.Context, userID, todoID string, input task.UpdateTodoInput) (*task.Todo, error)
	DeleteTodo(ctx context.Context, userID, todoID string) error
	ToggleTodoComplete(ctx context.Context, userID, todoID, date string) (*task.TodoWithStatus, error)
}

// FullService is the combined interface that embeds all service interfaces
// This provides backward compatibility while allowing handlers to use specific interfaces
type FullService interface {
	GoalService
	MilestoneService
	TagService
	TaskService
	EntityMemoService
	TodoService
}

// Compile-time check to ensure Service implements FullService
var _ FullService = (*Service)(nil)
