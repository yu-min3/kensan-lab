package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/kensan/backend/services/task/internal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockService is a mock implementation of the service
type MockService struct {
	mock.Mock
}

// Goal Operations
func (m *MockService) ListGoals(ctx context.Context, userID string, filter task.GoalFilter) ([]task.Goal, error) {
	args := m.Called(ctx, userID, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]task.Goal), args.Error(1)
}

func (m *MockService) GetGoal(ctx context.Context, userID, goalID string) (*task.Goal, error) {
	args := m.Called(ctx, userID, goalID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*task.Goal), args.Error(1)
}

func (m *MockService) CreateGoal(ctx context.Context, userID string, input task.CreateGoalInput) (*task.Goal, error) {
	args := m.Called(ctx, userID, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*task.Goal), args.Error(1)
}

func (m *MockService) UpdateGoal(ctx context.Context, userID, goalID string, input task.UpdateGoalInput) (*task.Goal, error) {
	args := m.Called(ctx, userID, goalID, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*task.Goal), args.Error(1)
}

func (m *MockService) DeleteGoal(ctx context.Context, userID, goalID string) error {
	args := m.Called(ctx, userID, goalID)
	return args.Error(0)
}

// Milestone Operations
func (m *MockService) ListMilestones(ctx context.Context, userID string, filter task.MilestoneFilter) ([]task.Milestone, error) {
	args := m.Called(ctx, userID, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]task.Milestone), args.Error(1)
}

func (m *MockService) GetMilestone(ctx context.Context, userID, milestoneID string) (*task.Milestone, error) {
	args := m.Called(ctx, userID, milestoneID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*task.Milestone), args.Error(1)
}

func (m *MockService) CreateMilestone(ctx context.Context, userID string, input task.CreateMilestoneInput) (*task.Milestone, error) {
	args := m.Called(ctx, userID, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*task.Milestone), args.Error(1)
}

func (m *MockService) UpdateMilestone(ctx context.Context, userID, milestoneID string, input task.UpdateMilestoneInput) (*task.Milestone, error) {
	args := m.Called(ctx, userID, milestoneID, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*task.Milestone), args.Error(1)
}

func (m *MockService) DeleteMilestone(ctx context.Context, userID, milestoneID string) error {
	args := m.Called(ctx, userID, milestoneID)
	return args.Error(0)
}

// Tag Operations
func (m *MockService) ListTags(ctx context.Context, userID string) ([]task.Tag, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]task.Tag), args.Error(1)
}

func (m *MockService) GetTag(ctx context.Context, userID, tagID string) (*task.Tag, error) {
	args := m.Called(ctx, userID, tagID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*task.Tag), args.Error(1)
}

func (m *MockService) CreateTag(ctx context.Context, userID string, input task.CreateTagInput) (*task.Tag, error) {
	args := m.Called(ctx, userID, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*task.Tag), args.Error(1)
}

func (m *MockService) UpdateTag(ctx context.Context, userID, tagID string, input task.UpdateTagInput) (*task.Tag, error) {
	args := m.Called(ctx, userID, tagID, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*task.Tag), args.Error(1)
}

func (m *MockService) DeleteTag(ctx context.Context, userID, tagID string) error {
	args := m.Called(ctx, userID, tagID)
	return args.Error(0)
}

// Task Operations
func (m *MockService) ListTasks(ctx context.Context, userID string, filter task.TaskFilter) ([]task.Task, error) {
	args := m.Called(ctx, userID, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]task.Task), args.Error(1)
}

func (m *MockService) GetTask(ctx context.Context, userID, taskID string) (*task.Task, error) {
	args := m.Called(ctx, userID, taskID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*task.Task), args.Error(1)
}

func (m *MockService) CreateTask(ctx context.Context, userID string, input task.CreateTaskInput) (*task.Task, error) {
	args := m.Called(ctx, userID, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*task.Task), args.Error(1)
}

func (m *MockService) UpdateTask(ctx context.Context, userID, taskID string, input task.UpdateTaskInput) (*task.Task, error) {
	args := m.Called(ctx, userID, taskID, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*task.Task), args.Error(1)
}

func (m *MockService) DeleteTask(ctx context.Context, userID, taskID string) error {
	args := m.Called(ctx, userID, taskID)
	return args.Error(0)
}

func (m *MockService) ToggleTaskComplete(ctx context.Context, userID, taskID string) (*task.Task, error) {
	args := m.Called(ctx, userID, taskID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*task.Task), args.Error(1)
}

func (m *MockService) GetChildTasks(ctx context.Context, userID, parentTaskID string) ([]task.Task, error) {
	args := m.Called(ctx, userID, parentTaskID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]task.Task), args.Error(1)
}

// Helper to create test context with user ID
func ctxWithUserID(userID string) context.Context {
	// Using a simple context key for testing
	type contextKey string
	return context.WithValue(context.Background(), contextKey("userID"), userID)
}

// Helper to create a request with chi URL params
func newRequestWithParams(method, path string, body interface{}, params map[string]string) *http.Request {
	var bodyReader *bytes.Reader
	if body != nil {
		bodyBytes, _ := json.Marshal(body)
		bodyReader = bytes.NewReader(bodyBytes)
	} else {
		bodyReader = bytes.NewReader(nil)
	}

	req := httptest.NewRequest(method, path, bodyReader)
	req.Header.Set("Content-Type", "application/json")

	// Add chi URL params
	rctx := chi.NewRouteContext()
	for key, value := range params {
		rctx.URLParams.Add(key, value)
	}
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	return req
}

// ========== Task Handler Tests ==========

func TestListTasks_Handler(t *testing.T) {
	t.Run("returns list of tasks", func(t *testing.T) {
		mockSvc := new(MockService)

		tasks := []task.Task{
			{ID: "t1", Name: "Task 1"},
			{ID: "t2", Name: "Task 2"},
		}

		mockSvc.On("ListTasks", mock.Anything, "user-123", mock.AnythingOfType("task.TaskFilter")).
			Return(tasks, nil)

		result, err := mockSvc.ListTasks(context.Background(), "user-123", task.TaskFilter{})

		assert.NoError(t, err)
		assert.Len(t, result, 2)
		mockSvc.AssertExpectations(t)
	})

	t.Run("filters by milestone ID", func(t *testing.T) {
		mockSvc := new(MockService)
		milestoneID := "m1"

		filter := task.TaskFilter{
			MilestoneID: &milestoneID,
		}

		tasks := []task.Task{
			{ID: "t1", Name: "Task 1", MilestoneID: &milestoneID},
		}

		mockSvc.On("ListTasks", mock.Anything, "user-123", filter).
			Return(tasks, nil)

		result, err := mockSvc.ListTasks(context.Background(), "user-123", filter)

		assert.NoError(t, err)
		assert.Len(t, result, 1)
		assert.Equal(t, &milestoneID, result[0].MilestoneID)
		mockSvc.AssertExpectations(t)
	})

	t.Run("filters by completed status", func(t *testing.T) {
		mockSvc := new(MockService)
		completed := true

		filter := task.TaskFilter{
			Completed: &completed,
		}

		tasks := []task.Task{
			{ID: "t1", Name: "Completed Task", Completed: true},
		}

		mockSvc.On("ListTasks", mock.Anything, "user-123", filter).
			Return(tasks, nil)

		result, err := mockSvc.ListTasks(context.Background(), "user-123", filter)

		assert.NoError(t, err)
		assert.Len(t, result, 1)
		assert.True(t, result[0].Completed)
		mockSvc.AssertExpectations(t)
	})
}

func TestCreateTask_Handler(t *testing.T) {
	t.Run("creates task successfully", func(t *testing.T) {
		mockSvc := new(MockService)

		input := task.CreateTaskInput{
			Name: "New Task",
		}

		expectedTask := &task.Task{
			ID:        "t-new",
			Name:      "New Task",
			Completed: false,
		}

		mockSvc.On("CreateTask", mock.Anything, "user-123", input).
			Return(expectedTask, nil)

		result, err := mockSvc.CreateTask(context.Background(), "user-123", input)

		assert.NoError(t, err)
		assert.Equal(t, "New Task", result.Name)
		assert.False(t, result.Completed)
		mockSvc.AssertExpectations(t)
	})

	t.Run("creates task with parent", func(t *testing.T) {
		mockSvc := new(MockService)
		parentID := "t-parent"

		input := task.CreateTaskInput{
			Name:         "Sub Task",
			ParentTaskID: &parentID,
		}

		expectedTask := &task.Task{
			ID:           "t-sub",
			Name:         "Sub Task",
			ParentTaskID: &parentID,
		}

		mockSvc.On("CreateTask", mock.Anything, "user-123", input).
			Return(expectedTask, nil)

		result, err := mockSvc.CreateTask(context.Background(), "user-123", input)

		assert.NoError(t, err)
		assert.Equal(t, &parentID, result.ParentTaskID)
		mockSvc.AssertExpectations(t)
	})

	t.Run("creates task with milestone", func(t *testing.T) {
		mockSvc := new(MockService)
		milestoneID := "m1"

		input := task.CreateTaskInput{
			Name:        "Milestone Task",
			MilestoneID: &milestoneID,
		}

		expectedTask := &task.Task{
			ID:          "t-ms",
			Name:        "Milestone Task",
			MilestoneID: &milestoneID,
		}

		mockSvc.On("CreateTask", mock.Anything, "user-123", input).
			Return(expectedTask, nil)

		result, err := mockSvc.CreateTask(context.Background(), "user-123", input)

		assert.NoError(t, err)
		assert.Equal(t, &milestoneID, result.MilestoneID)
		mockSvc.AssertExpectations(t)
	})
}

func TestGetTask_Handler(t *testing.T) {
	t.Run("returns task by ID", func(t *testing.T) {
		mockSvc := new(MockService)

		expectedTask := &task.Task{
			ID:   "t1",
			Name: "Test Task",
		}

		mockSvc.On("GetTask", mock.Anything, "user-123", "t1").
			Return(expectedTask, nil)

		result, err := mockSvc.GetTask(context.Background(), "user-123", "t1")

		assert.NoError(t, err)
		assert.Equal(t, "Test Task", result.Name)
		mockSvc.AssertExpectations(t)
	})
}

func TestToggleTaskComplete_Handler(t *testing.T) {
	t.Run("toggles task from incomplete to complete", func(t *testing.T) {
		mockSvc := new(MockService)

		expectedTask := &task.Task{
			ID:        "t1",
			Name:      "Test Task",
			Completed: true,
		}

		mockSvc.On("ToggleTaskComplete", mock.Anything, "user-123", "t1").
			Return(expectedTask, nil)

		result, err := mockSvc.ToggleTaskComplete(context.Background(), "user-123", "t1")

		assert.NoError(t, err)
		assert.True(t, result.Completed)
		mockSvc.AssertExpectations(t)
	})
}

func TestDeleteTask_Handler(t *testing.T) {
	t.Run("deletes task successfully", func(t *testing.T) {
		mockSvc := new(MockService)

		mockSvc.On("DeleteTask", mock.Anything, "user-123", "t1").
			Return(nil)

		err := mockSvc.DeleteTask(context.Background(), "user-123", "t1")

		assert.NoError(t, err)
		mockSvc.AssertExpectations(t)
	})
}

// ========== Goal Handler Tests ==========

func TestListGoals_Handler(t *testing.T) {
	t.Run("returns list of goals", func(t *testing.T) {
		mockSvc := new(MockService)

		goals := []task.Goal{
			{ID: "g1", Name: "Goal 1"},
			{ID: "g2", Name: "Goal 2"},
		}

		mockSvc.On("ListGoals", mock.Anything, "user-123", mock.AnythingOfType("task.GoalFilter")).
			Return(goals, nil)

		result, err := mockSvc.ListGoals(context.Background(), "user-123", task.GoalFilter{})

		assert.NoError(t, err)
		assert.Len(t, result, 2)
		mockSvc.AssertExpectations(t)
	})
}

func TestCreateGoal_Handler(t *testing.T) {
	t.Run("creates goal successfully", func(t *testing.T) {
		mockSvc := new(MockService)

		input := task.CreateGoalInput{
			Name:  "New Goal",
			Color: "#FF0000",
		}

		expectedGoal := &task.Goal{
			ID:    "g-new",
			Name:  "New Goal",
			Color: "#FF0000",
		}

		mockSvc.On("CreateGoal", mock.Anything, "user-123", input).
			Return(expectedGoal, nil)

		result, err := mockSvc.CreateGoal(context.Background(), "user-123", input)

		assert.NoError(t, err)
		assert.Equal(t, "New Goal", result.Name)
		mockSvc.AssertExpectations(t)
	})
}

func TestDeleteGoal_Handler(t *testing.T) {
	t.Run("deletes goal successfully", func(t *testing.T) {
		mockSvc := new(MockService)

		mockSvc.On("DeleteGoal", mock.Anything, "user-123", "g1").
			Return(nil)

		err := mockSvc.DeleteGoal(context.Background(), "user-123", "g1")

		assert.NoError(t, err)
		mockSvc.AssertExpectations(t)
	})
}

// ========== Milestone Handler Tests ==========

func TestListMilestones_Handler(t *testing.T) {
	t.Run("returns list of milestones", func(t *testing.T) {
		mockSvc := new(MockService)

		milestones := []task.Milestone{
			{ID: "m1", Name: "Milestone 1"},
			{ID: "m2", Name: "Milestone 2"},
		}

		mockSvc.On("ListMilestones", mock.Anything, "user-123", mock.AnythingOfType("task.MilestoneFilter")).
			Return(milestones, nil)

		result, err := mockSvc.ListMilestones(context.Background(), "user-123", task.MilestoneFilter{})

		assert.NoError(t, err)
		assert.Len(t, result, 2)
		mockSvc.AssertExpectations(t)
	})
}

func TestCreateMilestone_Handler(t *testing.T) {
	t.Run("creates milestone successfully", func(t *testing.T) {
		mockSvc := new(MockService)

		input := task.CreateMilestoneInput{
			Name:   "New Milestone",
			GoalID: "g1",
		}

		expectedMilestone := &task.Milestone{
			ID:     "m-new",
			Name:   "New Milestone",
			GoalID: "g1",
		}

		mockSvc.On("CreateMilestone", mock.Anything, "user-123", input).
			Return(expectedMilestone, nil)

		result, err := mockSvc.CreateMilestone(context.Background(), "user-123", input)

		assert.NoError(t, err)
		assert.Equal(t, "New Milestone", result.Name)
		mockSvc.AssertExpectations(t)
	})
}

func TestDeleteMilestone_Handler(t *testing.T) {
	t.Run("deletes milestone successfully", func(t *testing.T) {
		mockSvc := new(MockService)

		mockSvc.On("DeleteMilestone", mock.Anything, "user-123", "m1").
			Return(nil)

		err := mockSvc.DeleteMilestone(context.Background(), "user-123", "m1")

		assert.NoError(t, err)
		mockSvc.AssertExpectations(t)
	})
}

// ========== Tag Handler Tests ==========

func TestListTags_Handler(t *testing.T) {
	t.Run("returns list of tags", func(t *testing.T) {
		mockSvc := new(MockService)

		tags := []task.Tag{
			{ID: "tag1", Name: "Tag 1"},
			{ID: "tag2", Name: "Tag 2"},
		}

		mockSvc.On("ListTags", mock.Anything, "user-123").
			Return(tags, nil)

		result, err := mockSvc.ListTags(context.Background(), "user-123")

		assert.NoError(t, err)
		assert.Len(t, result, 2)
		mockSvc.AssertExpectations(t)
	})
}

func TestCreateTag_Handler(t *testing.T) {
	t.Run("creates tag successfully", func(t *testing.T) {
		mockSvc := new(MockService)

		input := task.CreateTagInput{
			Name:  "New Tag",
			Color: "#FF0000",
		}

		expectedTag := &task.Tag{
			ID:    "tag-new",
			Name:  "New Tag",
			Color: "#FF0000",
		}

		mockSvc.On("CreateTag", mock.Anything, "user-123", input).
			Return(expectedTag, nil)

		result, err := mockSvc.CreateTag(context.Background(), "user-123", input)

		assert.NoError(t, err)
		assert.Equal(t, "New Tag", result.Name)
		mockSvc.AssertExpectations(t)
	})
}

func TestDeleteTag_Handler(t *testing.T) {
	t.Run("deletes tag successfully", func(t *testing.T) {
		mockSvc := new(MockService)

		mockSvc.On("DeleteTag", mock.Anything, "user-123", "tag1").
			Return(nil)

		err := mockSvc.DeleteTag(context.Background(), "user-123", "tag1")

		assert.NoError(t, err)
		mockSvc.AssertExpectations(t)
	})
}

// ========== JSON Serialization Tests ==========

func TestGoal_JSONSerialization(t *testing.T) {
	goal := task.Goal{
		ID:     "g1",
		UserID: "u1",
		Name:   "Test Goal",
		Color:  "#FF0000",
		Status: task.GoalStatusActive,
	}

	data, err := json.Marshal(goal)
	assert.NoError(t, err)

	var decoded task.Goal
	err = json.Unmarshal(data, &decoded)
	assert.NoError(t, err)

	assert.Equal(t, goal.ID, decoded.ID)
	assert.Equal(t, goal.Name, decoded.Name)
	assert.Equal(t, goal.Color, decoded.Color)
}

func TestTask_JSONSerialization(t *testing.T) {
	estimatedMinutes := 30
	tk := task.Task{
		ID:               "t1",
		UserID:           "u1",
		Name:             "Test Task",
		EstimatedMinutes: &estimatedMinutes,
		Completed:        false,
	}

	data, err := json.Marshal(tk)
	assert.NoError(t, err)

	var decoded task.Task
	err = json.Unmarshal(data, &decoded)
	assert.NoError(t, err)

	assert.Equal(t, tk.ID, decoded.ID)
	assert.Equal(t, tk.Name, decoded.Name)
	assert.Equal(t, *tk.EstimatedMinutes, *decoded.EstimatedMinutes)
}

func TestMilestone_JSONSerialization(t *testing.T) {
	milestone := task.Milestone{
		ID:     "m1",
		UserID: "u1",
		GoalID: "g1",
		Name:   "Test Milestone",
		Status: task.MilestoneStatusActive,
	}

	data, err := json.Marshal(milestone)
	assert.NoError(t, err)

	var decoded task.Milestone
	err = json.Unmarshal(data, &decoded)
	assert.NoError(t, err)

	assert.Equal(t, milestone.ID, decoded.ID)
	assert.Equal(t, milestone.Name, decoded.Name)
	assert.Equal(t, milestone.Status, decoded.Status)
}

func TestTag_JSONSerialization(t *testing.T) {
	tag := task.Tag{
		ID:     "tag1",
		UserID: "u1",
		Name:   "Test Tag",
		Color:  "#00FF00",
	}

	data, err := json.Marshal(tag)
	assert.NoError(t, err)

	var decoded task.Tag
	err = json.Unmarshal(data, &decoded)
	assert.NoError(t, err)

	assert.Equal(t, tag.ID, decoded.ID)
	assert.Equal(t, tag.Name, decoded.Name)
	assert.Equal(t, tag.Color, decoded.Color)
}
