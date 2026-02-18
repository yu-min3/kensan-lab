package repository

import (
	"context"

	task "github.com/kensan/backend/services/task/internal"
)

// GoalRepository defines the interface for goal data access
type GoalRepository interface {
	ListGoals(ctx context.Context, userID string, filter task.GoalFilter) ([]task.Goal, error)
	GetGoalByID(ctx context.Context, userID, goalID string) (*task.Goal, error)
	CreateGoal(ctx context.Context, userID string, input task.CreateGoalInput) (*task.Goal, error)
	UpdateGoal(ctx context.Context, userID, goalID string, input task.UpdateGoalInput) (*task.Goal, error)
	DeleteGoal(ctx context.Context, userID, goalID string) error
	ReorderGoals(ctx context.Context, userID string, goalIDs []string) ([]task.Goal, error)
}

// MilestoneRepository defines the interface for milestone data access
type MilestoneRepository interface {
	ListMilestones(ctx context.Context, userID string, filter task.MilestoneFilter) ([]task.Milestone, error)
	GetMilestoneByID(ctx context.Context, userID, milestoneID string) (*task.Milestone, error)
	CreateMilestone(ctx context.Context, userID string, input task.CreateMilestoneInput) (*task.Milestone, error)
	UpdateMilestone(ctx context.Context, userID, milestoneID string, input task.UpdateMilestoneInput) (*task.Milestone, error)
	DeleteMilestone(ctx context.Context, userID, milestoneID string) error
}

// TagRepository defines the interface for tag data access
type TagRepository interface {
	ListTags(ctx context.Context, userID string) ([]task.Tag, error)
	ListNoteTags(ctx context.Context, userID string) ([]task.Tag, error)
	GetTagByID(ctx context.Context, userID, tagID string) (*task.Tag, error)
	CreateTag(ctx context.Context, userID string, input task.CreateTagInput) (*task.Tag, error)
	CreateNoteTag(ctx context.Context, userID string, input task.CreateTagInput) (*task.Tag, error)
	UpdateTag(ctx context.Context, userID, tagID string, input task.UpdateTagInput) (*task.Tag, error)
	DeleteTag(ctx context.Context, userID, tagID string) error
}

// TaskRepository defines the interface for task data access
type TaskRepository interface {
	ListTasks(ctx context.Context, userID string, filter task.TaskFilter) ([]task.Task, error)
	GetTaskByID(ctx context.Context, userID, taskID string) (*task.Task, error)
	CreateTask(ctx context.Context, userID string, input task.CreateTaskInput) (*task.Task, error)
	UpdateTask(ctx context.Context, userID, taskID string, input task.UpdateTaskInput) (*task.Task, error)
	DeleteTask(ctx context.Context, userID, taskID string) error
	GetChildTasks(ctx context.Context, userID, parentTaskID string) ([]task.Task, error)
	ToggleTaskComplete(ctx context.Context, userID, taskID string) (*task.Task, error)
	ReorderTasks(ctx context.Context, userID string, taskIDs []string) ([]task.Task, error)
	BulkDeleteTasks(ctx context.Context, userID string, taskIDs []string) error
	BulkCompleteTasks(ctx context.Context, userID string, taskIDs []string, completed bool) ([]task.Task, error)
}

// TaskTagRepository defines the interface for task-tag relationship operations
type TaskTagRepository interface {
	GetTaskTags(ctx context.Context, taskID string) ([]string, error)
	SetTaskTags(ctx context.Context, taskID string, tagIDs []string) error
}

// EntityMemoRepository defines the interface for entity memo data access
type EntityMemoRepository interface {
	ListEntityMemos(ctx context.Context, userID string, filter task.EntityMemoFilter) ([]task.EntityMemo, error)
	GetEntityMemoByID(ctx context.Context, userID, memoID string) (*task.EntityMemo, error)
	CreateEntityMemo(ctx context.Context, userID string, input task.CreateEntityMemoInput) (*task.EntityMemo, error)
	UpdateEntityMemo(ctx context.Context, userID, memoID string, input task.UpdateEntityMemoInput) (*task.EntityMemo, error)
	DeleteEntityMemo(ctx context.Context, userID, memoID string) error
}

// TodoRepository defines the interface for todo data access
type TodoRepository interface {
	ListTodos(ctx context.Context, userID string, filter task.TodoFilter) ([]task.Todo, error)
	ListTodosWithStatus(ctx context.Context, userID string, date string) ([]task.TodoWithStatus, error)
	GetTodoByID(ctx context.Context, userID, todoID string) (*task.Todo, error)
	CreateTodo(ctx context.Context, userID string, input task.CreateTodoInput) (*task.Todo, error)
	UpdateTodo(ctx context.Context, userID, todoID string, input task.UpdateTodoInput) (*task.Todo, error)
	DeleteTodo(ctx context.Context, userID, todoID string) error
}

// TodoCompletionRepository defines the interface for todo completion data access
type TodoCompletionRepository interface {
	GetTodoCompletion(ctx context.Context, userID, todoID, date string) (*task.TodoCompletion, error)
	CreateTodoCompletion(ctx context.Context, userID, todoID, date string) (*task.TodoCompletion, error)
	DeleteTodoCompletion(ctx context.Context, userID, todoID, date string) error
}

// Repository is the combined interface that embeds all repository interfaces
// This provides backward compatibility while allowing services to use specific interfaces
// following the Interface Segregation Principle (ISP)
type Repository interface {
	GoalRepository
	MilestoneRepository
	TagRepository
	TaskRepository
	TaskTagRepository
	EntityMemoRepository
	TodoRepository
	TodoCompletionRepository
}
