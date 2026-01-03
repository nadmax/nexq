package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/nadmax/nexq/internal/queue"
	"github.com/nadmax/nexq/internal/repository"
	"github.com/nadmax/nexq/internal/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestAPI(t *testing.T) (*API, *queue.Queue, *miniredis.Miniredis) {
	mr, err := miniredis.Run()
	require.NoError(t, err)

	q, err := queue.NewQueue(mr.Addr(), nil)
	require.NoError(t, err)

	api := NewAPI(q)

	return api, q, mr
}

func setupTestAPIWithMockRepo(t *testing.T) (*API, *queue.Queue, *repository.MockPostgresRepository, *miniredis.Miniredis) {
	mr, err := miniredis.Run()
	require.NoError(t, err)

	mockRepo := repository.NewMockPostgresRepository()
	q, err := queue.NewQueue(mr.Addr(), mockRepo)
	require.NoError(t, err)

	api := NewAPI(q)

	return api, q, mockRepo, mr
}

func TestCreateTask(t *testing.T) {
	api, q, mr := setupTestAPI(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	reqBody := CreateTaskRequest{
		Type:    "send_email",
		Payload: map[string]any{"to": "test@example.com"},
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/tasks", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	api.createTask(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var tsk task.Task
	err := json.Unmarshal(w.Body.Bytes(), &tsk)
	require.NoError(t, err)
	assert.Equal(t, "send_email", tsk.Type)
	assert.NotEmpty(t, tsk.ID)
	assert.Equal(t, task.PriorityMedium, tsk.Priority)
}

func TestCreateTaskWithHistory(t *testing.T) {
	api, q, mockRepo, mr := setupTestAPIWithMockRepo(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	req := CreateTaskRequest{
		Type:    "send_email",
		Payload: map[string]any{"to": "test@example.com"},
	}
	body, err := json.Marshal(req)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/tasks", bytes.NewReader(body))

	api.ServeHTTP(w, r)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, 1, mockRepo.GetSaveTaskCallCount(), "Task should be saved to repository")

	var tsk task.Task
	err = json.NewDecoder(w.Body).Decode(&tsk)
	require.NoError(t, err)

	assert.True(t, mockRepo.WasTaskSaved(tsk.ID), "Task should exist in repository")

	status, exists := mockRepo.GetTaskStatus(tsk.ID)
	assert.True(t, exists, "Task status should be retrievable")
	assert.Equal(t, task.StatusPending, status, "Task should be pending")
}

func TestCreateTask_WithPriority(t *testing.T) {
	api, q, mr := setupTestAPI(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	priority := task.PriorityHigh
	reqBody := CreateTaskRequest{
		Type:     "send_email",
		Payload:  map[string]any{"to": "test@example.com"},
		Priority: &priority,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/tasks", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	api.createTask(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var tsk task.Task
	err := json.Unmarshal(w.Body.Bytes(), &tsk)
	require.NoError(t, err)
	assert.Equal(t, task.PriorityHigh, tsk.Priority)
}

func TestCreateTask_WithSchedule(t *testing.T) {
	api, q, mr := setupTestAPI(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	scheduleIn := 60
	reqBody := CreateTaskRequest{
		Type:       "send_email",
		Payload:    map[string]any{"to": "test@example.com"},
		ScheduleIn: &scheduleIn,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/tasks", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	api.createTask(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var tsk task.Task
	err := json.Unmarshal(w.Body.Bytes(), &tsk)
	require.NoError(t, err)
	assert.True(t, tsk.ScheduledAt.After(tsk.CreatedAt))
}

func TestCreateTask_InvalidJSON(t *testing.T) {
	api, q, mr := setupTestAPI(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	req := httptest.NewRequest(http.MethodPost, "/api/tasks", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	api.createTask(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateTask_MissingType(t *testing.T) {
	api, q, mr := setupTestAPI(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	reqBody := CreateTaskRequest{
		Payload: map[string]any{"to": "test@example.com"},
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/tasks", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	api.createTask(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestListTasks(t *testing.T) {
	api, q, mr := setupTestAPI(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	task1 := task.NewTask("task1", nil, task.PriorityMedium)
	task2 := task.NewTask("task2", nil, task.PriorityHigh)
	err := q.Enqueue(task1)
	assert.NoError(t, err)
	err = q.Enqueue(task2)
	assert.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/tasks", nil)
	w := httptest.NewRecorder()

	api.listTasks(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var tasks []*task.Task
	err = json.Unmarshal(w.Body.Bytes(), &tasks)
	require.NoError(t, err)
	assert.Len(t, tasks, 2)
}

func TestListTasks_Empty(t *testing.T) {
	api, q, mr := setupTestAPI(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	req := httptest.NewRequest(http.MethodGet, "/api/tasks", nil)
	w := httptest.NewRecorder()

	api.listTasks(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var tasks []*task.Task
	err := json.Unmarshal(w.Body.Bytes(), &tasks)
	require.NoError(t, err)
	assert.Len(t, tasks, 0)
}

func TestGetTaskByID(t *testing.T) {
	api, q, mr := setupTestAPI(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	tsk := task.NewTask("test_task", map[string]any{"key": "value"}, task.PriorityMedium)
	err := q.Enqueue(tsk)
	assert.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/tasks/"+tsk.ID, nil)
	w := httptest.NewRecorder()

	api.handleTaskByID(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var retrieved task.Task
	err = json.Unmarshal(w.Body.Bytes(), &retrieved)
	require.NoError(t, err)
	assert.Equal(t, tsk.ID, retrieved.ID)
	assert.Equal(t, tsk.Type, retrieved.Type)
}

func TestGetTaskByID_NotFound(t *testing.T) {
	api, q, mr := setupTestAPI(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	req := httptest.NewRequest(http.MethodGet, "/api/tasks/non-existent", nil)
	w := httptest.NewRecorder()

	api.handleTaskByID(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandleTasks_MethodNotAllowed(t *testing.T) {
	api, q, mr := setupTestAPI(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	req := httptest.NewRequest(http.MethodDelete, "/api/tasks", nil)
	w := httptest.NewRecorder()

	api.handleTasks(w, req)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestHandleTaskByID_MethodNotAllowed(t *testing.T) {
	api, q, mr := setupTestAPI(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	req := httptest.NewRequest(http.MethodPost, "/api/tasks/123", nil)
	w := httptest.NewRecorder()

	api.handleTaskByID(w, req)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestServeHTTP(t *testing.T) {
	api, q, mr := setupTestAPI(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	req := httptest.NewRequest(http.MethodGet, "/api/tasks", nil)
	w := httptest.NewRecorder()

	api.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHistoryStatsWithMockRepo(t *testing.T) {
	api, q, mockRepo, mr := setupTestAPIWithMockRepo(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	mockRepo.TaskStats = []repository.TaskStats{
		{
			Type:          "send_email",
			Status:        "completed",
			Count:         10,
			AvgDurationMs: 250.5,
			MaxDurationMs: 500,
			MinDurationMs: 100,
			AvgRetries:    0.2,
		},
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/history/stats", nil)

	api.ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code)

	var stats []repository.TaskStats
	err := json.NewDecoder(w.Body).Decode(&stats)
	require.NoError(t, err)
	assert.Len(t, stats, 1)
	assert.Equal(t, "send_email", stats[0].Type)
	assert.Equal(t, 10, stats[0].Count)
}

func TestHistoryStatsWithoutRepo(t *testing.T) {
	api, q, mr := setupTestAPI(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/history/stats", nil)

	api.ServeHTTP(w, r)

	// Should return 503 when PostgreSQL is not configured
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}
