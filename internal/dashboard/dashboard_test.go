package dashboard

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/nadmax/nexq/internal/queue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestDashboard(t *testing.T) (*Dashboard, *queue.Queue, *miniredis.Miniredis) {
	mr, err := miniredis.Run()
	require.NoError(t, err)

	q, err := queue.NewQueue(mr.Addr())
	require.NoError(t, err)

	dash := NewDashboard(q)

	return dash, q, mr
}

func TestNewDashboard(t *testing.T) {
	dash, q, mr := setupTestDashboard(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	assert.NotNil(t, dash)
	assert.NotNil(t, dash.queue)
}

func TestGetStats_Empty(t *testing.T) {
	dash, q, mr := setupTestDashboard(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	req := httptest.NewRequest("GET", "/api/dashboard/stats", nil)
	w := httptest.NewRecorder()

	dash.GetStats(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var stats Stats
	err := json.Unmarshal(w.Body.Bytes(), &stats)
	require.NoError(t, err)

	assert.Equal(t, 0, stats.TotalTasks)
	assert.Equal(t, 0, stats.PendingTasks)
	assert.Equal(t, 0, stats.RunningTasks)
	assert.Equal(t, 0, stats.CompletedTasks)
	assert.Equal(t, 0, stats.FailedTasks)
	assert.Equal(t, "N/A", stats.AverageWaitTime)
	assert.NotZero(t, stats.LastUpdated)
}

func TestGetStats_WithTasks(t *testing.T) {
	dash, q, mr := setupTestDashboard(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	pending := queue.NewTask("pending_task", nil)
	pending.Status = queue.StatusPending
	require.NoError(t, q.Enqueue(pending))
	require.NoError(t, q.UpdateTask(pending))

	running := queue.NewTask("running_task", nil)
	running.Status = queue.StatusRunning
	now := time.Now()
	running.StartedAt = &now
	require.NoError(t, q.Enqueue(running))
	require.NoError(t, q.UpdateTask(running))

	completed := queue.NewTask("completed_task", nil)
	completed.Status = queue.StatusCompleted
	startTime := time.Now().Add(-2 * time.Second)
	completedTime := time.Now()
	completed.StartedAt = &startTime
	completed.CompletedAt = &completedTime
	require.NoError(t, q.Enqueue(completed))
	require.NoError(t, q.UpdateTask(completed))

	failed := queue.NewTask("failed_task", nil)
	failed.Status = queue.StatusFailed
	require.NoError(t, q.Enqueue(failed))
	require.NoError(t, q.UpdateTask(failed))

	req := httptest.NewRequest("GET", "/api/dashboard/stats", nil)
	w := httptest.NewRecorder()

	dash.GetStats(w, req)

	assert.Equal(t, 200, w.Code)

	var stats Stats
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &stats))

	assert.Equal(t, 4, stats.TotalTasks)
	assert.Equal(t, 1, stats.PendingTasks)
	assert.Equal(t, 1, stats.RunningTasks)
	assert.Equal(t, 1, stats.CompletedTasks)
	assert.Equal(t, 1, stats.FailedTasks)
}

func TestGetStats_TasksByType(t *testing.T) {
	dash, q, mr := setupTestDashboard(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	email1 := queue.NewTask("send_email", map[string]any{"to": "user1@test.com"})
	email2 := queue.NewTask("send_email", map[string]any{"to": "user2@test.com"})
	email3 := queue.NewTask("send_email", map[string]any{"to": "user3@test.com"})
	image1 := queue.NewTask("process_image", map[string]any{"url": "img1.jpg"})
	image2 := queue.NewTask("process_image", map[string]any{"url": "img2.jpg"})
	report := queue.NewTask("generate_report", map[string]any{"type": "monthly"})

	require.NoError(t, q.Enqueue(email1))
	require.NoError(t, q.Enqueue(email2))
	require.NoError(t, q.Enqueue(email3))
	require.NoError(t, q.Enqueue(image1))
	require.NoError(t, q.Enqueue(image2))
	require.NoError(t, q.Enqueue(report))

	req := httptest.NewRequest("GET", "/api/dashboard/stats", nil)
	w := httptest.NewRecorder()

	dash.GetStats(w, req)

	var stats Stats
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &stats))

	assert.Equal(t, 6, stats.TotalTasks)
	assert.Equal(t, 3, stats.TasksByType["send_email"])
	assert.Equal(t, 2, stats.TasksByType["process_image"])
	assert.Equal(t, 1, stats.TasksByType["generate_report"])
}

func TestGetStats_AverageWaitTime(t *testing.T) {
	dash, q, mr := setupTestDashboard(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	task1 := queue.NewTask("test1", nil)
	task1.CreatedAt = time.Now().Add(-10 * time.Second)
	startTime1 := time.Now().Add(-5 * time.Second)
	task1.StartedAt = &startTime1
	task1.Status = queue.StatusCompleted
	require.NoError(t, q.Enqueue(task1))
	require.NoError(t, q.UpdateTask(task1))

	// Create another task
	task2 := queue.NewTask("test2", nil)
	task2.CreatedAt = time.Now().Add(-8 * time.Second)
	startTime2 := time.Now().Add(-3 * time.Second)
	task2.StartedAt = &startTime2
	task2.Status = queue.StatusCompleted
	require.NoError(t, q.Enqueue(task2))
	require.NoError(t, q.UpdateTask(task2))

	req := httptest.NewRequest("GET", "/api/dashboard/stats", nil)
	w := httptest.NewRecorder()

	dash.GetStats(w, req)

	var stats Stats
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &stats))

	assert.NotEqual(t, "N/A", stats.AverageWaitTime)
	assert.Contains(t, stats.AverageWaitTime, "s") // Should contain seconds
}

func TestGetStats_NoStartedTasks(t *testing.T) {
	dash, q, mr := setupTestDashboard(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	task1 := queue.NewTask("pending1", nil)
	task1.Status = queue.StatusPending
	require.NoError(t, q.Enqueue(task1))
	require.NoError(t, q.UpdateTask(task1))

	task2 := queue.NewTask("pending2", nil)
	task2.Status = queue.StatusPending
	require.NoError(t, q.Enqueue(task2))
	require.NoError(t, q.UpdateTask(task2))

	req := httptest.NewRequest("GET", "/api/dashboard/stats", nil)
	w := httptest.NewRecorder()

	dash.GetStats(w, req)

	var stats Stats
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &stats))

	assert.Equal(t, "N/A", stats.AverageWaitTime)
	assert.Equal(t, 2, stats.TotalTasks)
	assert.Equal(t, 2, stats.PendingTasks)
}

func TestGetRecentTasks_Empty(t *testing.T) {
	dash, q, mr := setupTestDashboard(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	req := httptest.NewRequest("GET", "/api/dashboard/history", nil)
	w := httptest.NewRecorder()

	dash.GetRecentTasks(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var history []TaskHistory
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &history))

	assert.Len(t, history, 0)
}

func TestGetRecentTasks_WithCompletedTasks(t *testing.T) {
	dash, q, mr := setupTestDashboard(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	task := queue.NewTask("completed_task", map[string]any{"data": "test"})
	task.Status = queue.StatusCompleted
	startTime := time.Now().Add(-5 * time.Second)
	completedTime := time.Now()
	task.StartedAt = &startTime
	task.CompletedAt = &completedTime
	require.NoError(t, q.Enqueue(task))
	require.NoError(t, q.UpdateTask(task))

	req := httptest.NewRequest("GET", "/api/dashboard/history", nil)
	w := httptest.NewRecorder()

	dash.GetRecentTasks(w, req)

	assert.Equal(t, 200, w.Code)

	var history []TaskHistory
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &history))

	assert.Len(t, history, 1)
	assert.Equal(t, task.ID, history[0].TaskID)
	assert.Equal(t, task.Type, history[0].Type)
	assert.Equal(t, task.Status, history[0].Status)
	assert.NotEmpty(t, history[0].Duration)
}

func TestGetRecentTasks_OnlyCompletedOrFailed(t *testing.T) {
	dash, q, mr := setupTestDashboard(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	pending := queue.NewTask("pending", nil)
	pending.Status = queue.StatusPending
	require.NoError(t, q.Enqueue(pending))
	require.NoError(t, q.UpdateTask(pending))

	running := queue.NewTask("running", nil)
	running.Status = queue.StatusRunning
	require.NoError(t, q.Enqueue(running))
	require.NoError(t, q.UpdateTask(running))

	completed := queue.NewTask("completed", nil)
	completed.Status = queue.StatusCompleted
	now := time.Now()
	completed.CompletedAt = &now
	require.NoError(t, q.Enqueue(completed))
	require.NoError(t, q.UpdateTask(completed))

	failed := queue.NewTask("failed", nil)
	failed.Status = queue.StatusFailed
	failed.CompletedAt = &now
	require.NoError(t, q.Enqueue(failed))
	require.NoError(t, q.UpdateTask(failed))

	req := httptest.NewRequest("GET", "/api/dashboard/history", nil)
	w := httptest.NewRecorder()

	dash.GetRecentTasks(w, req)

	var history []TaskHistory
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &history))

	assert.Len(t, history, 2)

	types := []string{history[0].Type, history[1].Type}
	assert.Contains(t, types, "completed")
	assert.Contains(t, types, "failed")
	assert.NotContains(t, types, "pending")
	assert.NotContains(t, types, "running")
}

func TestGetRecentTasks_Last24HoursOnly(t *testing.T) {
	dash, q, mr := setupTestDashboard(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	old := queue.NewTask("old_task", nil)
	old.Status = queue.StatusCompleted
	oldTime := time.Now().Add(-25 * time.Hour)
	old.CompletedAt = &oldTime
	require.NoError(t, q.Enqueue(old))
	require.NoError(t, q.UpdateTask(old))

	recent := queue.NewTask("recent_task", nil)
	recent.Status = queue.StatusCompleted
	recentTime := time.Now().Add(-1 * time.Hour)
	recent.CompletedAt = &recentTime
	require.NoError(t, q.Enqueue(recent))
	require.NoError(t, q.UpdateTask(recent))

	veryRecent := queue.NewTask("very_recent", nil)
	veryRecent.Status = queue.StatusCompleted
	now := time.Now()
	veryRecent.CompletedAt = &now
	require.NoError(t, q.Enqueue(veryRecent))
	require.NoError(t, q.UpdateTask(veryRecent))

	req := httptest.NewRequest("GET", "/api/dashboard/history", nil)
	w := httptest.NewRecorder()

	dash.GetRecentTasks(w, req)

	var history []TaskHistory
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &history))

	assert.Len(t, history, 2)

	types := []string{history[0].Type, history[1].Type}
	assert.Contains(t, types, "recent_task")
	assert.Contains(t, types, "very_recent")
	assert.NotContains(t, types, "old_task")
}

func TestGetRecentTasks_WithDuration(t *testing.T) {
	dash, q, mr := setupTestDashboard(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	task := queue.NewTask("timed_task", nil)
	task.Status = queue.StatusCompleted
	task.CreatedAt = time.Now().Add(-10 * time.Second)
	startTime := time.Now().Add(-8 * time.Second)
	completedTime := time.Now().Add(-3 * time.Second)
	task.StartedAt = &startTime
	task.CompletedAt = &completedTime
	require.NoError(t, q.Enqueue(task))
	require.NoError(t, q.UpdateTask(task))

	req := httptest.NewRequest("GET", "/api/dashboard/history", nil)
	w := httptest.NewRecorder()

	dash.GetRecentTasks(w, req)

	var history []TaskHistory
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &history))

	require.Len(t, history, 1)
	assert.NotEmpty(t, history[0].Duration)
	assert.Contains(t, history[0].Duration, "s") // Should have seconds
}

func TestGetRecentTasks_NoDuration_WhenNotStarted(t *testing.T) {
	dash, q, mr := setupTestDashboard(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	task := queue.NewTask("no_start", nil)
	task.Status = queue.StatusCompleted
	now := time.Now()
	task.CompletedAt = &now
	require.NoError(t, q.Enqueue(task))
	require.NoError(t, q.UpdateTask(task))

	req := httptest.NewRequest("GET", "/api/dashboard/history", nil)
	w := httptest.NewRecorder()

	dash.GetRecentTasks(w, req)

	var history []TaskHistory
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &history))

	require.Len(t, history, 1)
	assert.Empty(t, history[0].Duration)
}

func TestGetRecentTasks_MultipleTasks(t *testing.T) {
	dash, q, mr := setupTestDashboard(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	now := time.Now()

	for i := 1; i <= 5; i++ {
		task := queue.NewTask("task", map[string]any{"id": i})
		task.Status = queue.StatusCompleted
		completedTime := now.Add(-time.Duration(i) * time.Hour)
		task.CompletedAt = &completedTime
		require.NoError(t, q.Enqueue(task))
		require.NoError(t, q.UpdateTask(task))
	}

	req := httptest.NewRequest("GET", "/api/dashboard/history", nil)
	w := httptest.NewRecorder()

	dash.GetRecentTasks(w, req)

	var history []TaskHistory
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &history))

	assert.Len(t, history, 5)

	for _, h := range history {
		assert.Equal(t, queue.StatusCompleted, h.Status)
		assert.NotEmpty(t, h.TaskID)
		assert.NotZero(t, h.CreatedAt)
	}
}

func TestGetStats_MixedStatusCounts(t *testing.T) {
	dash, q, mr := setupTestDashboard(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	for range 10 {
		task := queue.NewTask("pending", nil)
		task.Status = queue.StatusPending
		require.NoError(t, q.Enqueue(task))
		require.NoError(t, q.UpdateTask(task))
	}

	for range 5 {
		task := queue.NewTask("running", nil)
		task.Status = queue.StatusRunning
		require.NoError(t, q.Enqueue(task))
		require.NoError(t, q.UpdateTask(task))
	}

	for range 3 {
		task := queue.NewTask("completed", nil)
		task.Status = queue.StatusCompleted
		require.NoError(t, q.Enqueue(task))
		require.NoError(t, q.UpdateTask(task))
	}

	for range 2 {
		task := queue.NewTask("failed", nil)
		task.Status = queue.StatusFailed
		require.NoError(t, q.Enqueue(task))
		require.NoError(t, q.UpdateTask(task))
	}

	req := httptest.NewRequest("GET", "/api/dashboard/stats", nil)
	w := httptest.NewRecorder()

	dash.GetStats(w, req)

	var stats Stats
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &stats))

	assert.Equal(t, 20, stats.TotalTasks)
	assert.Equal(t, 10, stats.PendingTasks)
	assert.Equal(t, 5, stats.RunningTasks)
	assert.Equal(t, 3, stats.CompletedTasks)
	assert.Equal(t, 2, stats.FailedTasks)
}
