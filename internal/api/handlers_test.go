package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/nadmax/nexq/internal/queue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestAPI(t *testing.T) (*API, *queue.Queue, *miniredis.Miniredis) {
	mr, err := miniredis.Run()
	require.NoError(t, err)

	q, err := queue.NewQueue(mr.Addr())
	require.NoError(t, err)

	api := NewAPI(q)

	return api, q, mr
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

	var task queue.Task
	err := json.Unmarshal(w.Body.Bytes(), &task)
	require.NoError(t, err)
	assert.Equal(t, "send_email", task.Type)
	assert.NotEmpty(t, task.ID)
}

func TestCreateTask_WithPriority(t *testing.T) {
	api, q, mr := setupTestAPI(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	priority := queue.PriorityHigh
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

	var task queue.Task
	err := json.Unmarshal(w.Body.Bytes(), &task)
	require.NoError(t, err)
	assert.Equal(t, queue.PriorityHigh, task.Priority)
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

	var task queue.Task
	err := json.Unmarshal(w.Body.Bytes(), &task)
	require.NoError(t, err)
	assert.True(t, task.ScheduledAt.After(task.CreatedAt))
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

	task1 := queue.NewTask("task1", nil)
	task2 := queue.NewTask("task2", nil)
	err := q.Enqueue(task1)
	assert.NoError(t, err)
	err = q.Enqueue(task2)
	assert.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/tasks", nil)
	w := httptest.NewRecorder()

	api.listTasks(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var tasks []*queue.Task
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

	var tasks []*queue.Task
	err := json.Unmarshal(w.Body.Bytes(), &tasks)
	require.NoError(t, err)
	assert.Len(t, tasks, 0)
}

func TestGetTaskByID(t *testing.T) {
	api, q, mr := setupTestAPI(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	task := queue.NewTask("test_task", map[string]any{"key": "value"})
	err := q.Enqueue(task)
	assert.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/tasks/"+task.ID, nil)
	w := httptest.NewRecorder()

	api.handleTaskByID(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var retrieved queue.Task
	err = json.Unmarshal(w.Body.Bytes(), &retrieved)
	require.NoError(t, err)
	assert.Equal(t, task.ID, retrieved.ID)
	assert.Equal(t, task.Type, retrieved.Type)
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
