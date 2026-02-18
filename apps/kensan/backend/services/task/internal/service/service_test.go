package service

import (
	"context"
	"errors"
	"testing"

	task "github.com/kensan/backend/services/task/internal"
	"github.com/kensan/backend/services/task/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Compile-time check that MockRepository implements repository.Repository
var _ repository.Repository = (*MockRepository)(nil)

// MockRepository is a mock implementation of the repository.Repository interface
type MockRepository struct {
	mock.Mock
}

// Goal Operations
func (m *MockRepository) ListGoals(ctx context.Context, userID string, filter task.GoalFilter) ([]task.Goal, error) {
	args := m.Called(ctx, userID, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]task.Goal), args.Error(1)
}

func (m *MockRepository) GetGoalByID(ctx context.Context, userID, goalID string) (*task.Goal, error) {
	args := m.Called(ctx, userID, goalID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*task.Goal), args.Error(1)
}

func (m *MockRepository) CreateGoal(ctx context.Context, userID string, input task.CreateGoalInput) (*task.Goal, error) {
	args := m.Called(ctx, userID, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*task.Goal), args.Error(1)
}

func (m *MockRepository) UpdateGoal(ctx context.Context, userID, goalID string, input task.UpdateGoalInput) (*task.Goal, error) {
	args := m.Called(ctx, userID, goalID, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*task.Goal), args.Error(1)
}

func (m *MockRepository) DeleteGoal(ctx context.Context, userID, goalID string) error {
	args := m.Called(ctx, userID, goalID)
	return args.Error(0)
}

func (m *MockRepository) ReorderGoals(ctx context.Context, userID string, goalIDs []string) ([]task.Goal, error) {
	args := m.Called(ctx, userID, goalIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]task.Goal), args.Error(1)
}

// Milestone Operations
func (m *MockRepository) ListMilestones(ctx context.Context, userID string, filter task.MilestoneFilter) ([]task.Milestone, error) {
	args := m.Called(ctx, userID, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]task.Milestone), args.Error(1)
}

func (m *MockRepository) GetMilestoneByID(ctx context.Context, userID, milestoneID string) (*task.Milestone, error) {
	args := m.Called(ctx, userID, milestoneID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*task.Milestone), args.Error(1)
}

func (m *MockRepository) CreateMilestone(ctx context.Context, userID string, input task.CreateMilestoneInput) (*task.Milestone, error) {
	args := m.Called(ctx, userID, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*task.Milestone), args.Error(1)
}

func (m *MockRepository) UpdateMilestone(ctx context.Context, userID, milestoneID string, input task.UpdateMilestoneInput) (*task.Milestone, error) {
	args := m.Called(ctx, userID, milestoneID, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*task.Milestone), args.Error(1)
}

func (m *MockRepository) DeleteMilestone(ctx context.Context, userID, milestoneID string) error {
	args := m.Called(ctx, userID, milestoneID)
	return args.Error(0)
}

// Tag Operations
func (m *MockRepository) ListTags(ctx context.Context, userID string) ([]task.Tag, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]task.Tag), args.Error(1)
}

func (m *MockRepository) ListNoteTags(ctx context.Context, userID string) ([]task.Tag, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]task.Tag), args.Error(1)
}

func (m *MockRepository) GetTagByID(ctx context.Context, userID, tagID string) (*task.Tag, error) {
	args := m.Called(ctx, userID, tagID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*task.Tag), args.Error(1)
}

func (m *MockRepository) CreateTag(ctx context.Context, userID string, input task.CreateTagInput) (*task.Tag, error) {
	args := m.Called(ctx, userID, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*task.Tag), args.Error(1)
}

func (m *MockRepository) CreateNoteTag(ctx context.Context, userID string, input task.CreateTagInput) (*task.Tag, error) {
	args := m.Called(ctx, userID, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*task.Tag), args.Error(1)
}

func (m *MockRepository) UpdateTag(ctx context.Context, userID, tagID string, input task.UpdateTagInput) (*task.Tag, error) {
	args := m.Called(ctx, userID, tagID, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*task.Tag), args.Error(1)
}

func (m *MockRepository) DeleteTag(ctx context.Context, userID, tagID string) error {
	args := m.Called(ctx, userID, tagID)
	return args.Error(0)
}

// Task Operations
func (m *MockRepository) ListTasks(ctx context.Context, userID string, filter task.TaskFilter) ([]task.Task, error) {
	args := m.Called(ctx, userID, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]task.Task), args.Error(1)
}

func (m *MockRepository) GetTaskByID(ctx context.Context, userID, taskID string) (*task.Task, error) {
	args := m.Called(ctx, userID, taskID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*task.Task), args.Error(1)
}

func (m *MockRepository) CreateTask(ctx context.Context, userID string, input task.CreateTaskInput) (*task.Task, error) {
	args := m.Called(ctx, userID, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*task.Task), args.Error(1)
}

func (m *MockRepository) UpdateTask(ctx context.Context, userID, taskID string, input task.UpdateTaskInput) (*task.Task, error) {
	args := m.Called(ctx, userID, taskID, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*task.Task), args.Error(1)
}

func (m *MockRepository) DeleteTask(ctx context.Context, userID, taskID string) error {
	args := m.Called(ctx, userID, taskID)
	return args.Error(0)
}

func (m *MockRepository) ToggleTaskComplete(ctx context.Context, userID, taskID string) (*task.Task, error) {
	args := m.Called(ctx, userID, taskID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*task.Task), args.Error(1)
}

func (m *MockRepository) ReorderTasks(ctx context.Context, userID string, taskIDs []string) ([]task.Task, error) {
	args := m.Called(ctx, userID, taskIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]task.Task), args.Error(1)
}

func (m *MockRepository) BulkDeleteTasks(ctx context.Context, userID string, taskIDs []string) error {
	args := m.Called(ctx, userID, taskIDs)
	return args.Error(0)
}

func (m *MockRepository) BulkCompleteTasks(ctx context.Context, userID string, taskIDs []string, completed bool) ([]task.Task, error) {
	args := m.Called(ctx, userID, taskIDs, completed)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]task.Task), args.Error(1)
}

func (m *MockRepository) GetChildTasks(ctx context.Context, userID, parentTaskID string) ([]task.Task, error) {
	args := m.Called(ctx, userID, parentTaskID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]task.Task), args.Error(1)
}

// Task-Tag Operations
func (m *MockRepository) GetTaskTags(ctx context.Context, taskID string) ([]string, error) {
	args := m.Called(ctx, taskID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockRepository) SetTaskTags(ctx context.Context, taskID string, tagIDs []string) error {
	args := m.Called(ctx, taskID, tagIDs)
	return args.Error(0)
}

// EntityMemo Operations
func (m *MockRepository) ListEntityMemos(ctx context.Context, userID string, filter task.EntityMemoFilter) ([]task.EntityMemo, error) {
	args := m.Called(ctx, userID, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]task.EntityMemo), args.Error(1)
}

func (m *MockRepository) GetEntityMemoByID(ctx context.Context, userID, memoID string) (*task.EntityMemo, error) {
	args := m.Called(ctx, userID, memoID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*task.EntityMemo), args.Error(1)
}

func (m *MockRepository) CreateEntityMemo(ctx context.Context, userID string, input task.CreateEntityMemoInput) (*task.EntityMemo, error) {
	args := m.Called(ctx, userID, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*task.EntityMemo), args.Error(1)
}

func (m *MockRepository) UpdateEntityMemo(ctx context.Context, userID, memoID string, input task.UpdateEntityMemoInput) (*task.EntityMemo, error) {
	args := m.Called(ctx, userID, memoID, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*task.EntityMemo), args.Error(1)
}

func (m *MockRepository) DeleteEntityMemo(ctx context.Context, userID, memoID string) error {
	args := m.Called(ctx, userID, memoID)
	return args.Error(0)
}

// Todo Operations
func (m *MockRepository) ListTodos(ctx context.Context, userID string, filter task.TodoFilter) ([]task.Todo, error) {
	args := m.Called(ctx, userID, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]task.Todo), args.Error(1)
}

func (m *MockRepository) ListTodosWithStatus(ctx context.Context, userID string, date string) ([]task.TodoWithStatus, error) {
	args := m.Called(ctx, userID, date)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]task.TodoWithStatus), args.Error(1)
}

func (m *MockRepository) GetTodoByID(ctx context.Context, userID, todoID string) (*task.Todo, error) {
	args := m.Called(ctx, userID, todoID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*task.Todo), args.Error(1)
}

func (m *MockRepository) CreateTodo(ctx context.Context, userID string, input task.CreateTodoInput) (*task.Todo, error) {
	args := m.Called(ctx, userID, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*task.Todo), args.Error(1)
}

func (m *MockRepository) UpdateTodo(ctx context.Context, userID, todoID string, input task.UpdateTodoInput) (*task.Todo, error) {
	args := m.Called(ctx, userID, todoID, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*task.Todo), args.Error(1)
}

func (m *MockRepository) DeleteTodo(ctx context.Context, userID, todoID string) error {
	args := m.Called(ctx, userID, todoID)
	return args.Error(0)
}

// TodoCompletion Operations
func (m *MockRepository) GetTodoCompletion(ctx context.Context, userID, todoID, date string) (*task.TodoCompletion, error) {
	args := m.Called(ctx, userID, todoID, date)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*task.TodoCompletion), args.Error(1)
}

func (m *MockRepository) CreateTodoCompletion(ctx context.Context, userID, todoID, date string) (*task.TodoCompletion, error) {
	args := m.Called(ctx, userID, todoID, date)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*task.TodoCompletion), args.Error(1)
}

func (m *MockRepository) DeleteTodoCompletion(ctx context.Context, userID, todoID, date string) error {
	args := m.Called(ctx, userID, todoID, date)
	return args.Error(0)
}

// ========== Task Tests ==========

func TestListTasks_Success(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	filter := task.TaskFilter{}

	expectedTasks := []task.Task{
		{ID: "task-1", Name: "Task 1", UserID: userID},
		{ID: "task-2", Name: "Task 2", UserID: userID},
	}

	mockRepo.On("ListTasks", ctx, userID, filter).Return(expectedTasks, nil)

	tasks, err := svc.ListTasks(ctx, userID, filter)

	assert.NoError(t, err)
	assert.Len(t, tasks, 2)
	assert.Equal(t, "Task 1", tasks[0].Name)
	mockRepo.AssertExpectations(t)
}

func TestListTasks_WithMilestoneFilter(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	milestoneID := "milestone-123"
	filter := task.TaskFilter{
		MilestoneID: &milestoneID,
	}

	expectedMilestone := &task.Milestone{ID: milestoneID, UserID: userID, Name: "Test Milestone"}
	expectedTasks := []task.Task{
		{ID: "task-1", Name: "Task 1", UserID: userID, MilestoneID: &milestoneID},
	}

	mockRepo.On("GetMilestoneByID", ctx, userID, milestoneID).Return(expectedMilestone, nil)
	mockRepo.On("ListTasks", ctx, userID, filter).Return(expectedTasks, nil)

	tasks, err := svc.ListTasks(ctx, userID, filter)

	assert.NoError(t, err)
	assert.Len(t, tasks, 1)
	assert.Equal(t, &milestoneID, tasks[0].MilestoneID)
	mockRepo.AssertExpectations(t)
}

func TestListTasks_WithMilestoneFilter_MilestoneNotFound(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	milestoneID := "nonexistent"
	filter := task.TaskFilter{
		MilestoneID: &milestoneID,
	}

	mockRepo.On("GetMilestoneByID", ctx, userID, milestoneID).Return(nil, nil)

	tasks, err := svc.ListTasks(ctx, userID, filter)

	assert.Error(t, err)
	assert.Equal(t, ErrMilestoneNotFound, err)
	assert.Nil(t, tasks)
	mockRepo.AssertExpectations(t)
}

func TestListTasks_WithParentTaskFilter(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	parentTaskID := "task-parent"
	filter := task.TaskFilter{
		ParentTaskID: &parentTaskID,
	}

	expectedParentTask := &task.Task{ID: parentTaskID, UserID: userID, Name: "Parent Task"}
	expectedTasks := []task.Task{
		{ID: "task-child-1", Name: "Child Task 1", UserID: userID, ParentTaskID: &parentTaskID},
	}

	mockRepo.On("GetTaskByID", ctx, userID, parentTaskID).Return(expectedParentTask, nil)
	mockRepo.On("ListTasks", ctx, userID, filter).Return(expectedTasks, nil)

	tasks, err := svc.ListTasks(ctx, userID, filter)

	assert.NoError(t, err)
	assert.Len(t, tasks, 1)
	mockRepo.AssertExpectations(t)
}

func TestListTasks_WithParentTaskFilter_ParentNotFound(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	parentTaskID := "nonexistent"
	filter := task.TaskFilter{
		ParentTaskID: &parentTaskID,
	}

	mockRepo.On("GetTaskByID", ctx, userID, parentTaskID).Return(nil, nil)

	tasks, err := svc.ListTasks(ctx, userID, filter)

	assert.Error(t, err)
	assert.Equal(t, ErrTaskNotFound, err)
	assert.Nil(t, tasks)
	mockRepo.AssertExpectations(t)
}

func TestGetTask_Success(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	taskID := "task-123"

	expectedTask := &task.Task{
		ID:        taskID,
		UserID:    userID,
		Name:      "Test Task",
		Completed: false,
	}

	mockRepo.On("GetTaskByID", ctx, userID, taskID).Return(expectedTask, nil)

	resultTask, err := svc.GetTask(ctx, userID, taskID)

	assert.NoError(t, err)
	assert.NotNil(t, resultTask)
	assert.Equal(t, "Test Task", resultTask.Name)
	assert.False(t, resultTask.Completed)
	mockRepo.AssertExpectations(t)
}

func TestGetTask_NotFound(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	taskID := "nonexistent"

	mockRepo.On("GetTaskByID", ctx, userID, taskID).Return(nil, nil)

	resultTask, err := svc.GetTask(ctx, userID, taskID)

	assert.Error(t, err)
	assert.Equal(t, ErrTaskNotFound, err)
	assert.Nil(t, resultTask)
	mockRepo.AssertExpectations(t)
}

func TestCreateTask_Success(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	input := task.CreateTaskInput{
		Name: "New Task",
	}

	expectedTask := &task.Task{
		ID:        "task-new",
		UserID:    userID,
		Name:      "New Task",
		Completed: false,
	}

	mockRepo.On("CreateTask", ctx, userID, input).Return(expectedTask, nil)

	resultTask, err := svc.CreateTask(ctx, userID, input)

	assert.NoError(t, err)
	assert.NotNil(t, resultTask)
	assert.Equal(t, "New Task", resultTask.Name)
	mockRepo.AssertExpectations(t)
}

func TestCreateTask_EmptyName(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	input := task.CreateTaskInput{
		Name: "",
	}

	resultTask, err := svc.CreateTask(ctx, userID, input)

	assert.Error(t, err)
	assert.Equal(t, ErrInvalidInput, err)
	assert.Nil(t, resultTask)
	mockRepo.AssertExpectations(t)
}

func TestCreateTask_WithMilestone_Success(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	milestoneID := "milestone-123"
	input := task.CreateTaskInput{
		Name:        "New Task",
		MilestoneID: &milestoneID,
	}

	expectedMilestone := &task.Milestone{ID: milestoneID, UserID: userID, Name: "Test Milestone"}
	expectedTask := &task.Task{
		ID:          "task-new",
		UserID:      userID,
		MilestoneID: &milestoneID,
		Name:        "New Task",
		Completed:   false,
	}

	mockRepo.On("GetMilestoneByID", ctx, userID, milestoneID).Return(expectedMilestone, nil)
	mockRepo.On("CreateTask", ctx, userID, input).Return(expectedTask, nil)

	resultTask, err := svc.CreateTask(ctx, userID, input)

	assert.NoError(t, err)
	assert.NotNil(t, resultTask)
	assert.Equal(t, &milestoneID, resultTask.MilestoneID)
	mockRepo.AssertExpectations(t)
}

func TestCreateTask_WithMilestone_MilestoneNotFound(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	milestoneID := "nonexistent"
	input := task.CreateTaskInput{
		Name:        "New Task",
		MilestoneID: &milestoneID,
	}

	mockRepo.On("GetMilestoneByID", ctx, userID, milestoneID).Return(nil, nil)

	resultTask, err := svc.CreateTask(ctx, userID, input)

	assert.Error(t, err)
	assert.Equal(t, ErrMilestoneNotFound, err)
	assert.Nil(t, resultTask)
	mockRepo.AssertExpectations(t)
}

func TestCreateTask_WithParentTask_Success(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	parentTaskID := "task-parent"
	input := task.CreateTaskInput{
		Name:         "Child Task",
		ParentTaskID: &parentTaskID,
	}

	expectedParentTask := &task.Task{ID: parentTaskID, UserID: userID, Name: "Parent Task"}
	expectedTask := &task.Task{
		ID:           "task-new",
		UserID:       userID,
		Name:         "Child Task",
		ParentTaskID: &parentTaskID,
		Completed:    false,
	}

	mockRepo.On("GetTaskByID", ctx, userID, parentTaskID).Return(expectedParentTask, nil)
	mockRepo.On("CreateTask", ctx, userID, input).Return(expectedTask, nil)

	resultTask, err := svc.CreateTask(ctx, userID, input)

	assert.NoError(t, err)
	assert.NotNil(t, resultTask)
	assert.Equal(t, &parentTaskID, resultTask.ParentTaskID)
	mockRepo.AssertExpectations(t)
}

func TestCreateTask_WithParentTask_ParentNotFound(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	parentTaskID := "nonexistent"
	input := task.CreateTaskInput{
		Name:         "Child Task",
		ParentTaskID: &parentTaskID,
	}

	mockRepo.On("GetTaskByID", ctx, userID, parentTaskID).Return(nil, nil)

	resultTask, err := svc.CreateTask(ctx, userID, input)

	assert.Error(t, err)
	assert.Equal(t, ErrTaskNotFound, err)
	assert.Nil(t, resultTask)
	mockRepo.AssertExpectations(t)
}

func TestUpdateTask_Success(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	taskID := "task-123"
	newName := "Updated Task"
	input := task.UpdateTaskInput{
		Name: &newName,
	}

	existingTask := &task.Task{ID: taskID, UserID: userID, Name: "Old Task"}
	expectedTask := &task.Task{
		ID:     taskID,
		UserID: userID,
		Name:   newName,
	}

	mockRepo.On("GetTaskByID", ctx, userID, taskID).Return(existingTask, nil)
	mockRepo.On("UpdateTask", ctx, userID, taskID, input).Return(expectedTask, nil)

	resultTask, err := svc.UpdateTask(ctx, userID, taskID, input)

	assert.NoError(t, err)
	assert.NotNil(t, resultTask)
	assert.Equal(t, "Updated Task", resultTask.Name)
	mockRepo.AssertExpectations(t)
}

func TestUpdateTask_NotFound(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	taskID := "nonexistent"
	newName := "Updated Task"
	input := task.UpdateTaskInput{
		Name: &newName,
	}

	mockRepo.On("GetTaskByID", ctx, userID, taskID).Return(nil, nil)

	resultTask, err := svc.UpdateTask(ctx, userID, taskID, input)

	assert.Error(t, err)
	assert.Equal(t, ErrTaskNotFound, err)
	assert.Nil(t, resultTask)
	mockRepo.AssertExpectations(t)
}

func TestUpdateTask_ChangeMilestone_MilestoneNotFound(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	taskID := "task-123"
	newMilestoneID := "nonexistent"
	input := task.UpdateTaskInput{
		MilestoneID: &newMilestoneID,
	}

	existingTask := &task.Task{ID: taskID, UserID: userID, Name: "Task"}
	mockRepo.On("GetTaskByID", ctx, userID, taskID).Return(existingTask, nil)
	mockRepo.On("GetMilestoneByID", ctx, userID, newMilestoneID).Return(nil, nil)

	resultTask, err := svc.UpdateTask(ctx, userID, taskID, input)

	assert.Error(t, err)
	assert.Equal(t, ErrMilestoneNotFound, err)
	assert.Nil(t, resultTask)
	mockRepo.AssertExpectations(t)
}

func TestUpdateTask_ChangeParent_CircularReference(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	taskID := "task-123"
	input := task.UpdateTaskInput{
		ParentTaskID: &taskID, // Self-reference
	}

	existingTask := &task.Task{ID: taskID, UserID: userID, Name: "Task"}
	mockRepo.On("GetTaskByID", ctx, userID, taskID).Return(existingTask, nil).Once()
	mockRepo.On("GetTaskByID", ctx, userID, taskID).Return(existingTask, nil).Once()

	resultTask, err := svc.UpdateTask(ctx, userID, taskID, input)

	assert.Error(t, err)
	assert.Equal(t, ErrInvalidInput, err)
	assert.Nil(t, resultTask)
	mockRepo.AssertExpectations(t)
}

func TestDeleteTask_Success(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	taskID := "task-123"

	existingTask := &task.Task{ID: taskID, UserID: userID}
	mockRepo.On("GetTaskByID", ctx, userID, taskID).Return(existingTask, nil)
	mockRepo.On("DeleteTask", ctx, userID, taskID).Return(nil)

	err := svc.DeleteTask(ctx, userID, taskID)

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestDeleteTask_NotFound(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	taskID := "nonexistent"

	mockRepo.On("GetTaskByID", ctx, userID, taskID).Return(nil, nil)

	err := svc.DeleteTask(ctx, userID, taskID)

	assert.Error(t, err)
	assert.Equal(t, ErrTaskNotFound, err)
	mockRepo.AssertExpectations(t)
}

func TestToggleTaskComplete_Success(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	taskID := "task-123"

	existingTask := &task.Task{
		ID:        taskID,
		UserID:    userID,
		Name:      "Test Task",
		Completed: false,
	}
	// Task is initially not completed, toggle should make it completed
	expectedTask := &task.Task{
		ID:        taskID,
		UserID:    userID,
		Name:      "Test Task",
		Completed: true, // After toggle
	}

	mockRepo.On("GetTaskByID", ctx, userID, taskID).Return(existingTask, nil)
	mockRepo.On("ToggleTaskComplete", ctx, userID, taskID).Return(expectedTask, nil)

	resultTask, err := svc.ToggleTaskComplete(ctx, userID, taskID)

	assert.NoError(t, err)
	assert.NotNil(t, resultTask)
	assert.True(t, resultTask.Completed)
	mockRepo.AssertExpectations(t)
}

func TestToggleTaskComplete_NotFound(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	taskID := "nonexistent"

	mockRepo.On("GetTaskByID", ctx, userID, taskID).Return(nil, nil)

	resultTask, err := svc.ToggleTaskComplete(ctx, userID, taskID)

	assert.Error(t, err)
	assert.Equal(t, ErrTaskNotFound, err)
	assert.Nil(t, resultTask)
	mockRepo.AssertExpectations(t)
}

func TestGetChildTasks_Success(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	parentTaskID := "task-parent"

	parentTask := &task.Task{
		ID:     parentTaskID,
		UserID: userID,
		Name:   "Parent Task",
	}
	expectedTasks := []task.Task{
		{ID: "task-child-1", Name: "Child Task 1", UserID: userID, ParentTaskID: &parentTaskID},
		{ID: "task-child-2", Name: "Child Task 2", UserID: userID, ParentTaskID: &parentTaskID},
	}

	mockRepo.On("GetTaskByID", ctx, userID, parentTaskID).Return(parentTask, nil)
	mockRepo.On("GetChildTasks", ctx, userID, parentTaskID).Return(expectedTasks, nil)

	tasks, err := svc.GetChildTasks(ctx, userID, parentTaskID)

	assert.NoError(t, err)
	assert.Len(t, tasks, 2)
	assert.Equal(t, "Child Task 1", tasks[0].Name)
	mockRepo.AssertExpectations(t)
}

func TestGetChildTasks_ParentNotFound(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	parentTaskID := "nonexistent"

	mockRepo.On("GetTaskByID", ctx, userID, parentTaskID).Return(nil, nil)

	tasks, err := svc.GetChildTasks(ctx, userID, parentTaskID)

	assert.Error(t, err)
	assert.Equal(t, ErrTaskNotFound, err)
	assert.Nil(t, tasks)
	mockRepo.AssertExpectations(t)
}

// ========== Goal Tests ==========

func TestListGoals_Success(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	filter := task.GoalFilter{}

	expectedGoals := []task.Goal{
		{ID: "goal-1", Name: "Goal 1", UserID: userID},
		{ID: "goal-2", Name: "Goal 2", UserID: userID},
	}

	mockRepo.On("ListGoals", ctx, userID, filter).Return(expectedGoals, nil)

	goals, err := svc.ListGoals(ctx, userID, filter)

	assert.NoError(t, err)
	assert.Len(t, goals, 2)
	assert.Equal(t, "Goal 1", goals[0].Name)
	mockRepo.AssertExpectations(t)
}

func TestGetGoal_Success(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	goalID := "goal-123"

	expectedGoal := &task.Goal{
		ID:     goalID,
		UserID: userID,
		Name:   "Test Goal",
	}

	mockRepo.On("GetGoalByID", ctx, userID, goalID).Return(expectedGoal, nil)

	goal, err := svc.GetGoal(ctx, userID, goalID)

	assert.NoError(t, err)
	assert.NotNil(t, goal)
	assert.Equal(t, "Test Goal", goal.Name)
	mockRepo.AssertExpectations(t)
}

func TestGetGoal_NotFound(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	goalID := "nonexistent"

	mockRepo.On("GetGoalByID", ctx, userID, goalID).Return(nil, nil)

	goal, err := svc.GetGoal(ctx, userID, goalID)

	assert.Error(t, err)
	assert.Equal(t, ErrGoalNotFound, err)
	assert.Nil(t, goal)
	mockRepo.AssertExpectations(t)
}

func TestCreateGoal_Success(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	input := task.CreateGoalInput{
		Name:  "New Goal",
		Color: "#FF0000",
	}

	expectedGoal := &task.Goal{
		ID:     "goal-new",
		UserID: userID,
		Name:   "New Goal",
		Color:  "#FF0000",
	}

	mockRepo.On("CreateGoal", ctx, userID, input).Return(expectedGoal, nil)

	goal, err := svc.CreateGoal(ctx, userID, input)

	assert.NoError(t, err)
	assert.NotNil(t, goal)
	assert.Equal(t, "New Goal", goal.Name)
	mockRepo.AssertExpectations(t)
}

func TestCreateGoal_EmptyName(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	input := task.CreateGoalInput{
		Name: "",
	}

	goal, err := svc.CreateGoal(ctx, userID, input)

	assert.Error(t, err)
	assert.Equal(t, ErrInvalidInput, err)
	assert.Nil(t, goal)
	mockRepo.AssertExpectations(t)
}

func TestDeleteGoal_Success(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	goalID := "goal-123"

	existingGoal := &task.Goal{ID: goalID, UserID: userID}
	mockRepo.On("GetGoalByID", ctx, userID, goalID).Return(existingGoal, nil)
	mockRepo.On("DeleteGoal", ctx, userID, goalID).Return(nil)

	err := svc.DeleteGoal(ctx, userID, goalID)

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestDeleteGoal_NotFound(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	goalID := "nonexistent"

	mockRepo.On("GetGoalByID", ctx, userID, goalID).Return(nil, nil)

	err := svc.DeleteGoal(ctx, userID, goalID)

	assert.Error(t, err)
	assert.Equal(t, ErrGoalNotFound, err)
	mockRepo.AssertExpectations(t)
}

// ========== Milestone Tests ==========

func TestListMilestones_Success(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	filter := task.MilestoneFilter{}

	expectedMilestones := []task.Milestone{
		{ID: "milestone-1", Name: "Milestone 1", UserID: userID},
		{ID: "milestone-2", Name: "Milestone 2", UserID: userID},
	}

	mockRepo.On("ListMilestones", ctx, userID, filter).Return(expectedMilestones, nil)

	milestones, err := svc.ListMilestones(ctx, userID, filter)

	assert.NoError(t, err)
	assert.Len(t, milestones, 2)
	assert.Equal(t, "Milestone 1", milestones[0].Name)
	mockRepo.AssertExpectations(t)
}

func TestGetMilestone_NotFound(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	milestoneID := "nonexistent"

	mockRepo.On("GetMilestoneByID", ctx, userID, milestoneID).Return(nil, nil)

	milestone, err := svc.GetMilestone(ctx, userID, milestoneID)

	assert.Error(t, err)
	assert.Equal(t, ErrMilestoneNotFound, err)
	assert.Nil(t, milestone)
	mockRepo.AssertExpectations(t)
}

func TestCreateMilestone_Success(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	goalID := "goal-123"
	input := task.CreateMilestoneInput{
		Name:   "New Milestone",
		GoalID: goalID,
	}

	expectedGoal := &task.Goal{ID: goalID, UserID: userID}
	expectedMilestone := &task.Milestone{
		ID:     "milestone-new",
		UserID: userID,
		GoalID: goalID,
		Name:   "New Milestone",
	}

	mockRepo.On("GetGoalByID", ctx, userID, goalID).Return(expectedGoal, nil)
	mockRepo.On("CreateMilestone", ctx, userID, input).Return(expectedMilestone, nil)

	milestone, err := svc.CreateMilestone(ctx, userID, input)

	assert.NoError(t, err)
	assert.NotNil(t, milestone)
	assert.Equal(t, "New Milestone", milestone.Name)
	mockRepo.AssertExpectations(t)
}

func TestCreateMilestone_EmptyName(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	input := task.CreateMilestoneInput{
		Name:   "",
		GoalID: "goal-123",
	}

	milestone, err := svc.CreateMilestone(ctx, userID, input)

	assert.Error(t, err)
	assert.Equal(t, ErrInvalidInput, err)
	assert.Nil(t, milestone)
	mockRepo.AssertExpectations(t)
}

func TestCreateMilestone_GoalNotFound(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	goalID := "nonexistent"
	input := task.CreateMilestoneInput{
		Name:   "New Milestone",
		GoalID: goalID,
	}

	mockRepo.On("GetGoalByID", ctx, userID, goalID).Return(nil, nil)

	milestone, err := svc.CreateMilestone(ctx, userID, input)

	assert.Error(t, err)
	assert.Equal(t, ErrGoalNotFound, err)
	assert.Nil(t, milestone)
	mockRepo.AssertExpectations(t)
}

func TestDeleteMilestone_Success(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	milestoneID := "milestone-123"

	existingMilestone := &task.Milestone{ID: milestoneID, UserID: userID}
	mockRepo.On("GetMilestoneByID", ctx, userID, milestoneID).Return(existingMilestone, nil)
	mockRepo.On("DeleteMilestone", ctx, userID, milestoneID).Return(nil)

	err := svc.DeleteMilestone(ctx, userID, milestoneID)

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestDeleteMilestone_NotFound(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	milestoneID := "nonexistent"

	mockRepo.On("GetMilestoneByID", ctx, userID, milestoneID).Return(nil, nil)

	err := svc.DeleteMilestone(ctx, userID, milestoneID)

	assert.Error(t, err)
	assert.Equal(t, ErrMilestoneNotFound, err)
	mockRepo.AssertExpectations(t)
}

// ========== Tag Tests ==========

func TestListTags_Success(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)

	ctx := context.Background()
	userID := "user-123"

	expectedTags := []task.Tag{
		{ID: "tag-1", Name: "Tag 1", UserID: userID},
		{ID: "tag-2", Name: "Tag 2", UserID: userID},
	}

	mockRepo.On("ListTags", ctx, userID).Return(expectedTags, nil)

	tags, err := svc.ListTags(ctx, userID)

	assert.NoError(t, err)
	assert.Len(t, tags, 2)
	assert.Equal(t, "Tag 1", tags[0].Name)
	mockRepo.AssertExpectations(t)
}

func TestGetTag_NotFound(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	tagID := "nonexistent"

	mockRepo.On("GetTagByID", ctx, userID, tagID).Return(nil, nil)

	tag, err := svc.GetTag(ctx, userID, tagID)

	assert.Error(t, err)
	assert.Equal(t, ErrTagNotFound, err)
	assert.Nil(t, tag)
	mockRepo.AssertExpectations(t)
}

func TestCreateTag_Success(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	input := task.CreateTagInput{
		Name:  "New Tag",
		Color: "#FF0000",
	}

	expectedTag := &task.Tag{
		ID:       "tag-new",
		UserID:   userID,
		Name:     "New Tag",
		Color:    "#FF0000",
		Category: "general",
	}

	// Service sets default category to "general"
	expectedInput := input
	expectedInput.Category = "general"
	mockRepo.On("CreateTag", ctx, userID, expectedInput).Return(expectedTag, nil)

	tag, err := svc.CreateTag(ctx, userID, input)

	assert.NoError(t, err)
	assert.NotNil(t, tag)
	assert.Equal(t, "New Tag", tag.Name)
	mockRepo.AssertExpectations(t)
}

func TestCreateTag_EmptyName(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	input := task.CreateTagInput{
		Name: "",
	}

	tag, err := svc.CreateTag(ctx, userID, input)

	assert.Error(t, err)
	assert.Equal(t, ErrInvalidInput, err)
	assert.Nil(t, tag)
	mockRepo.AssertExpectations(t)
}

func TestDeleteTag_Success(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	tagID := "tag-123"

	existingTag := &task.Tag{ID: tagID, UserID: userID}
	mockRepo.On("GetTagByID", ctx, userID, tagID).Return(existingTag, nil)
	mockRepo.On("DeleteTag", ctx, userID, tagID).Return(nil)

	err := svc.DeleteTag(ctx, userID, tagID)

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestDeleteTag_NotFound(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	tagID := "nonexistent"

	mockRepo.On("GetTagByID", ctx, userID, tagID).Return(nil, nil)

	err := svc.DeleteTag(ctx, userID, tagID)

	assert.Error(t, err)
	assert.Equal(t, ErrTagNotFound, err)
	mockRepo.AssertExpectations(t)
}

// ========== MilestoneStatus Validation Tests ==========

func TestMilestoneStatus_IsValid(t *testing.T) {
	testCases := []struct {
		status   task.MilestoneStatus
		expected bool
	}{
		{task.MilestoneStatusActive, true},
		{task.MilestoneStatusCompleted, true},
		{task.MilestoneStatusArchived, true},
		{task.MilestoneStatus("Invalid"), false},
		{task.MilestoneStatus(""), false},
	}

	for _, tc := range testCases {
		t.Run(string(tc.status), func(t *testing.T) {
			result := tc.status.IsValid()
			assert.Equal(t, tc.expected, result)
		})
	}
}

// ========== Repository Error Tests ==========

func TestListTasks_RepositoryError(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	filter := task.TaskFilter{}

	mockRepo.On("ListTasks", ctx, userID, filter).Return(nil, errors.New("database error"))

	tasks, err := svc.ListTasks(ctx, userID, filter)

	assert.Error(t, err)
	assert.Nil(t, tasks)
	mockRepo.AssertExpectations(t)
}
