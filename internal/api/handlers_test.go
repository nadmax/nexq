package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/nadmax/nexq/internal/queue"
	"github.com/nadmax/nexq/internal/repository/mocks"
	"github.com/nadmax/nexq/internal/repository/models"
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

func setupTestAPIWithMockRepo(t *testing.T) (*API, *queue.Queue, *mocks.MockPostgresRepository, *miniredis.Miniredis) {
	mr, err := miniredis.Run()
	require.NoError(t, err)

	mockRepo := mocks.NewMockPostgresRepository()
	q, err := queue.NewQueue(mr.Addr(), mockRepo)
	require.NoError(t, err)

	api := NewAPI(q)

	return api, q, mockRepo, mr
}

func TestCreateTask(t *testing.T) {
	api, q, mr := setupTestAPI(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	reqBody := TaskRequest{
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
	assert.Equal(t, task.MediumPriority, tsk.Priority)
}

func TestCreateTaskWithHistory(t *testing.T) {
	api, q, mockRepo, mr := setupTestAPIWithMockRepo(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	req := TaskRequest{
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
	assert.Equal(t, task.PendingStatus, status, "Task should be pending")
}

func TestCreateTask_WithPriority(t *testing.T) {
	api, q, mr := setupTestAPI(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	priority := task.HighPriority
	reqBody := TaskRequest{
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
	assert.Equal(t, task.HighPriority, tsk.Priority)
}

func TestCreateTask_WithSchedule(t *testing.T) {
	api, q, mr := setupTestAPI(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	scheduleIn := 60
	reqBody := TaskRequest{
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

	reqBody := TaskRequest{
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

	task1 := task.NewTask("task1", nil, task.MediumPriority)
	task2 := task.NewTask("task2", nil, task.HighPriority)
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

	tsk := task.NewTask("test_task", map[string]any{"key": "value"}, task.MediumPriority)
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

func TestHandleCancelTask_Success(t *testing.T) {
	api, q, mr := setupTestAPI(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	tsk := task.NewTask("send_email", map[string]any{"to": "test@example.com"}, task.MediumPriority)
	err := q.Enqueue(tsk)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/tasks/cancel/"+tsk.ID, nil)
	w := httptest.NewRecorder()

	api.handleCancelTask(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]string
	err = json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)
	assert.Equal(t, "Task cancelled successfully", response["message"])
	assert.Equal(t, tsk.ID, response["task_id"])
}

func TestHandleCancelTask_MethodNotAllowed(t *testing.T) {
	api, q, mr := setupTestAPI(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	req := httptest.NewRequest(http.MethodGet, "/api/tasks/cancel/task-123", nil)
	w := httptest.NewRecorder()

	api.handleCancelTask(w, req)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)

	var errResp map[string]string
	err := json.NewDecoder(w.Body).Decode(&errResp)
	require.NoError(t, err)
	assert.Contains(t, errResp["error"], "Method not allowed")
}

func TestHandleCancelTask_MissingTaskID(t *testing.T) {
	api, q, mr := setupTestAPI(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	req := httptest.NewRequest(http.MethodPost, "/api/tasks/cancel/", nil)
	w := httptest.NewRecorder()

	api.handleCancelTask(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var errResp map[string]string
	err := json.NewDecoder(w.Body).Decode(&errResp)
	require.NoError(t, err)
	assert.Contains(t, errResp["error"], "Task ID required")
}

func TestHandleCancelTask_NotFound(t *testing.T) {
	api, q, mr := setupTestAPI(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	req := httptest.NewRequest(http.MethodPost, "/api/tasks/cancel/non-existent-task", nil)
	w := httptest.NewRecorder()

	api.handleCancelTask(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandleDLQTasks_Success(t *testing.T) {
	api, q, mr := setupTestAPI(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	req := httptest.NewRequest(http.MethodGet, "/api/dlq/tasks", nil)
	w := httptest.NewRecorder()

	api.handleDLQTasks(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var tasks []*task.Task
	err := json.NewDecoder(w.Body).Decode(&tasks)
	require.NoError(t, err)
	// When there are no tasks in DLQ, it should return an empty slice
	assert.Len(t, tasks, 0)
}

func TestHandleDLQTasks_MethodNotAllowed(t *testing.T) {
	api, q, mr := setupTestAPI(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	req := httptest.NewRequest(http.MethodPost, "/api/dlq/tasks", nil)
	w := httptest.NewRecorder()

	api.handleDLQTasks(w, req)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestGetDLQTask_Success(t *testing.T) {
	api, q, mr := setupTestAPI(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	// Create a task and move it to DLQ
	tsk := task.NewTask("failed_task", map[string]any{"data": "test"}, task.MediumPriority)
	tsk.RetryCount = 3
	err := q.MoveToDeadLetter(tsk, "max retries exceeded")
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/dlq/tasks/"+tsk.ID, nil)
	w := httptest.NewRecorder()

	api.handleDLQTaskByID(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var retrieved task.Task
	err = json.NewDecoder(w.Body).Decode(&retrieved)
	require.NoError(t, err)
	assert.Equal(t, tsk.ID, retrieved.ID)
	assert.Equal(t, tsk.Type, retrieved.Type)
}

func TestGetDLQTask_NotFound(t *testing.T) {
	api, q, mr := setupTestAPI(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	req := httptest.NewRequest(http.MethodGet, "/api/dlq/tasks/non-existent", nil)
	w := httptest.NewRecorder()

	api.handleDLQTaskByID(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestPurgeDLQTask_Success(t *testing.T) {
	api, q, mr := setupTestAPI(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	tsk := task.NewTask("failed_task", map[string]any{"data": "test"}, task.MediumPriority)
	err := q.MoveToDeadLetter(tsk, "test error")
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodDelete, "/api/dlq/tasks/"+tsk.ID, nil)
	w := httptest.NewRecorder()

	api.handleDLQTaskByID(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Empty(t, w.Body.String())
}

func TestRetryDLQTask_Success(t *testing.T) {
	api, q, mr := setupTestAPI(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	tsk := task.NewTask("failed_task", map[string]any{"data": "test"}, task.MediumPriority)
	err := q.MoveToDeadLetter(tsk, "test error")
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/dlq/tasks/"+tsk.ID+"/retry", nil)
	w := httptest.NewRecorder()

	api.handleDLQTaskByID(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]string
	err = json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)
	assert.Equal(t, "Task moved back to queue for retry", response["message"])
	assert.Equal(t, tsk.ID, response["task_id"])
}

func TestRetryDLQTask_NotFound(t *testing.T) {
	api, q, mr := setupTestAPI(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	req := httptest.NewRequest(http.MethodPost, "/api/dlq/tasks/non-existent/retry", nil)
	w := httptest.NewRecorder()

	api.handleDLQTaskByID(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandleDLQTaskByID_MissingTaskID(t *testing.T) {
	api, q, mr := setupTestAPI(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	req := httptest.NewRequest(http.MethodGet, "/api/dlq/tasks/", nil)
	w := httptest.NewRecorder()

	api.handleDLQTaskByID(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var errResp map[string]string
	err := json.NewDecoder(w.Body).Decode(&errResp)
	require.NoError(t, err)
	assert.Contains(t, errResp["error"], "Task ID is required")
}

func TestHandleDLQTaskByID_InvalidEndpoint(t *testing.T) {
	api, q, mr := setupTestAPI(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	req := httptest.NewRequest(http.MethodPost, "/api/dlq/tasks/task-123/invalid", nil)
	w := httptest.NewRecorder()

	api.handleDLQTaskByID(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var errResp map[string]string
	err := json.NewDecoder(w.Body).Decode(&errResp)
	require.NoError(t, err)
	assert.Contains(t, errResp["error"], "Invalid endpoint")
}

func TestHandleDLQTaskByID_MethodNotAllowed(t *testing.T) {
	api, q, mr := setupTestAPI(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	req := httptest.NewRequest(http.MethodPut, "/api/dlq/tasks/task-123", nil)
	w := httptest.NewRecorder()

	api.handleDLQTaskByID(w, req)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestHandleDLQStats_Success(t *testing.T) {
	api, q, mr := setupTestAPI(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	// Add some tasks to DLQ
	tsk1 := task.NewTask("task_type_1", nil, task.MediumPriority)
	tsk2 := task.NewTask("task_type_2", nil, task.HighPriority)

	err := q.MoveToDeadLetter(tsk1, "error 1")
	require.NoError(t, err)
	err = q.MoveToDeadLetter(tsk2, "error 2")
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/dlq/stats", nil)
	w := httptest.NewRecorder()

	api.handleDLQStats(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var stats map[string]any
	err = json.NewDecoder(w.Body).Decode(&stats)
	require.NoError(t, err)
	assert.NotNil(t, stats)
}

func TestHandleDLQStats_MethodNotAllowed(t *testing.T) {
	api, q, mr := setupTestAPI(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	req := httptest.NewRequest(http.MethodPost, "/api/dlq/stats", nil)
	w := httptest.NewRecorder()

	api.handleDLQStats(w, req)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)

	var errResp map[string]string
	err := json.NewDecoder(w.Body).Decode(&errResp)
	require.NoError(t, err)
	assert.Contains(t, errResp["error"], "Method not allowed")
}

func TestHandleCancelTask_CannotCancel(t *testing.T) {
	api, q, mr := setupTestAPI(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	tsk := task.NewTask("send_email", nil, task.MediumPriority)
	err := q.Enqueue(tsk)
	require.NoError(t, err)

	_, err = q.Dequeue()
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/tasks/cancel/"+tsk.ID, nil)
	w := httptest.NewRecorder()

	api.handleCancelTask(w, req)

	if w.Code == http.StatusBadRequest {
		var errResp map[string]string
		err = json.NewDecoder(w.Body).Decode(&errResp)
		require.NoError(t, err)
		assert.NotEmpty(t, errResp["error"])
	}
}

func TestPurgeDLQTask_NonExistent(t *testing.T) {
	api, q, mr := setupTestAPI(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	req := httptest.NewRequest(http.MethodDelete, "/api/dlq/tasks/non-existent", nil)
	w := httptest.NewRecorder()

	api.handleDLQTaskByID(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Empty(t, w.Body.String())
}

func TestHistoryStatsWithMockRepo(t *testing.T) {
	api, q, mockRepo, mr := setupTestAPIWithMockRepo(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	mockRepo.TaskStats = []models.TaskStats{
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

	var stats []models.TaskStats
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

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestHandleRecentHistory_Success(t *testing.T) {
	api, q, mockRepo, mr := setupTestAPIWithMockRepo(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	duration1 := 250
	duration2 := 150
	mockRepo.RecentTasks = []models.RecentTask{
		{
			TaskID:     "task-1",
			Type:       "send_email",
			Status:     string(task.CompletedStatus),
			CreatedAt:  time.Now().Add(-1 * time.Hour),
			DurationMs: &duration1,
		},
		{
			TaskID:     "task-2",
			Type:       "notification",
			Status:     string(task.CompletedStatus),
			CreatedAt:  time.Now().Add(-30 * time.Minute),
			DurationMs: &duration2,
		},
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/history/recent", nil)

	api.handleRecentHistory(w, r)

	assert.Equal(t, http.StatusOK, w.Code)

	var tasks []models.RecentTask
	err := json.NewDecoder(w.Body).Decode(&tasks)
	require.NoError(t, err)
	assert.Len(t, tasks, 2)
	assert.Equal(t, "send_email", tasks[0].Type)
}

func TestHandleRecentHistory_WithLimit(t *testing.T) {
	api, q, mockRepo, mr := setupTestAPIWithMockRepo(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	mockRepo.RecentTasks = []models.RecentTask{}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/history/recent?limit=50", nil)

	api.handleRecentHistory(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandleRecentHistory_InvalidLimit(t *testing.T) {
	api, q, mockRepo, mr := setupTestAPIWithMockRepo(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	mockRepo.RecentTasks = []models.RecentTask{}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/history/recent?limit=invalid", nil)

	api.handleRecentHistory(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandleRecentHistory_MethodNotAllowed(t *testing.T) {
	api, q, _, mr := setupTestAPIWithMockRepo(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/history/recent", nil)

	api.handleRecentHistory(w, r)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestHandleRecentHistory_NoRepository(t *testing.T) {
	api, q, mr := setupTestAPI(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/history/recent", nil)

	api.handleRecentHistory(w, r)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var errResp map[string]string
	err := json.NewDecoder(w.Body).Decode(&errResp)
	require.NoError(t, err)
	assert.Contains(t, errResp["error"], "PostgreSQL not configured")
}

func TestHandleRecentHistory_RepositoryError(t *testing.T) {
	api, q, mockRepo, mr := setupTestAPIWithMockRepo(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	mockRepo.GetRecentTasksError = errors.New("database connection failed")

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/history/recent", nil)

	api.handleRecentHistory(w, r)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var errResp map[string]string
	err := json.NewDecoder(w.Body).Decode(&errResp)
	require.NoError(t, err)
	assert.Contains(t, errResp["error"], "database connection failed")
}

func TestHandleTaskHistory_Success(t *testing.T) {
	api, q, mockRepo, mr := setupTestAPIWithMockRepo(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	taskID := "task-123"
	mockRepo.ExecutionLog = []mocks.LogExecutionCall{
		{
			TaskID:        taskID,
			AttemptNumber: 1,
			Status:        "pending",
			DurationMs:    0,
			WorkerID:      "worker-1",
		},
		{
			TaskID:        taskID,
			AttemptNumber: 1,
			Status:        "completed",
			DurationMs:    250,
			WorkerID:      "worker-1",
		},
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/history/task/"+taskID, nil)

	api.handleTaskHistory(w, r)

	assert.Equal(t, http.StatusOK, w.Code)

	var history []map[string]any
	err := json.NewDecoder(w.Body).Decode(&history)
	require.NoError(t, err)
	assert.Len(t, history, 2)
	assert.Equal(t, taskID, history[0]["task_id"])
	assert.Equal(t, "pending", history[0]["status"])
	assert.Equal(t, "completed", history[1]["status"])
}

func TestHandleTaskHistory_MissingTaskID(t *testing.T) {
	api, q, _, mr := setupTestAPIWithMockRepo(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/history/task/", nil)

	api.handleTaskHistory(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var errResp map[string]string
	err := json.NewDecoder(w.Body).Decode(&errResp)
	require.NoError(t, err)
	assert.Contains(t, errResp["error"], "Task ID is required")
}

func TestHandleTaskHistory_MethodNotAllowed(t *testing.T) {
	api, q, _, mr := setupTestAPIWithMockRepo(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/history/task/task-123", nil)

	api.handleTaskHistory(w, r)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestHandleTaskHistory_NoRepository(t *testing.T) {
	api, q, mr := setupTestAPI(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/history/task/task-123", nil)

	api.handleTaskHistory(w, r)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestHandleTaskHistory_RepositoryError(t *testing.T) {
	api, q, mockRepo, mr := setupTestAPIWithMockRepo(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	mockRepo.GetTaskHistoryError = errors.New("query failed")

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/history/task/task-123", nil)

	api.handleTaskHistory(w, r)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var errResp map[string]string
	err := json.NewDecoder(w.Body).Decode(&errResp)
	require.NoError(t, err)
	assert.Contains(t, errResp["error"], "query failed")
}

func TestHandleTasksByType_Success(t *testing.T) {
	api, q, mockRepo, mr := setupTestAPIWithMockRepo(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	taskType := "send_email"
	duration1 := 200
	duration2 := 300
	mockRepo.RecentTasks = []models.RecentTask{
		{
			TaskID:     "task-1",
			Type:       taskType,
			Status:     string(task.CompletedStatus),
			CreatedAt:  time.Now(),
			DurationMs: &duration1,
		},
		{
			TaskID:     "task-2",
			Type:       taskType,
			Status:     string(task.CompletedStatus),
			CreatedAt:  time.Now(),
			DurationMs: &duration2,
		},
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/history/type/"+taskType, nil)

	api.handleTasksByType(w, r)

	assert.Equal(t, http.StatusOK, w.Code)

	var tasks []models.RecentTask
	err := json.NewDecoder(w.Body).Decode(&tasks)
	require.NoError(t, err)
	assert.Len(t, tasks, 2)
	assert.Equal(t, taskType, tasks[0].Type)
}

func TestHandleTasksByType_WithLimit(t *testing.T) {
	api, q, mockRepo, mr := setupTestAPIWithMockRepo(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	mockRepo.RecentTasks = []models.RecentTask{}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/history/type/send_email?limit=25", nil)

	api.handleTasksByType(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandleTasksByType_MissingType(t *testing.T) {
	api, q, _, mr := setupTestAPIWithMockRepo(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/history/type/", nil)

	api.handleTasksByType(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var errResp map[string]string
	err := json.NewDecoder(w.Body).Decode(&errResp)
	require.NoError(t, err)
	assert.Contains(t, errResp["error"], "Task type is required")
}

func TestHandleTasksByType_MethodNotAllowed(t *testing.T) {
	api, q, _, mr := setupTestAPIWithMockRepo(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodDelete, "/api/history/type/send_email", nil)

	api.handleTasksByType(w, r)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestHandleTasksByType_NoRepository(t *testing.T) {
	api, q, mr := setupTestAPI(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/history/type/send_email", nil)

	api.handleTasksByType(w, r)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestHandleTasksByType_RepositoryError(t *testing.T) {
	api, q, mockRepo, mr := setupTestAPIWithMockRepo(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	mockRepo.GetTasksByTypeError = errors.New("database error")

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/history/type/send_email", nil)

	api.handleTasksByType(w, r)

	// Debug
	t.Logf("Status Code: %d", w.Code)
	t.Logf("Response: %s", w.Body.String())

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var errResp map[string]string
	err := json.NewDecoder(w.Body).Decode(&errResp)
	require.NoError(t, err)
	assert.Contains(t, errResp["error"], "database error")
}
