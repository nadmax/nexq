package dashboard

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/nadmax/nexq/internal/queue"
	"github.com/nadmax/nexq/internal/repository"
	"github.com/nadmax/nexq/internal/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestDashboard(t *testing.T) (*Dashboard, *queue.Queue, *miniredis.Miniredis) {
	mr, err := miniredis.Run()
	require.NoError(t, err)

	q, err := queue.NewQueue(mr.Addr(), nil)
	require.NoError(t, err)

	dash := NewDashboard(q)

	return dash, q, mr
}

func setupTestDashboardWithMockRepo(t *testing.T) (*Dashboard, *queue.Queue, *repository.MockPostgresRepository, *miniredis.Miniredis) {
	mr, err := miniredis.Run()
	require.NoError(t, err)

	mockRepo := repository.NewMockPostgresRepository()
	q, err := queue.NewQueue(mr.Addr(), mockRepo)
	require.NoError(t, err)

	dash := NewDashboard(q)

	return dash, q, mockRepo, mr
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

	pending := task.NewTask("pending_task", nil, task.MediumPriority)
	pending.Status = task.PendingStatus
	require.NoError(t, q.Enqueue(pending))
	require.NoError(t, q.UpdateTask(pending))

	running := task.NewTask("running_task", nil, task.MediumPriority)
	running.Status = task.RunningStatus
	now := time.Now()
	running.StartedAt = &now
	require.NoError(t, q.Enqueue(running))
	require.NoError(t, q.UpdateTask(running))

	completed := task.NewTask("completed_task", nil, task.MediumPriority)
	completed.Status = task.CompletedStatus
	startTime := time.Now().Add(-2 * time.Second)
	completedTime := time.Now()
	completed.StartedAt = &startTime
	completed.CompletedAt = &completedTime
	require.NoError(t, q.Enqueue(completed))
	require.NoError(t, q.UpdateTask(completed))

	failed := task.NewTask("failed_task", nil, task.MediumPriority)
	failed.Status = task.FailedStatus
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

	email1 := task.NewTask("send_email", map[string]any{"to": "user1@test.com"}, task.MediumPriority)
	email2 := task.NewTask("send_email", map[string]any{"to": "user2@test.com"}, task.MediumPriority)
	email3 := task.NewTask("send_email", map[string]any{"to": "user3@test.com"}, task.MediumPriority)
	image1 := task.NewTask("process_image", map[string]any{"url": "img1.jpg"}, task.MediumPriority)
	image2 := task.NewTask("process_image", map[string]any{"url": "img2.jpg"}, task.MediumPriority)
	report := task.NewTask("generate_report", map[string]any{"type": "monthly"}, task.MediumPriority)

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

	task1 := task.NewTask("test1", nil, task.MediumPriority)
	task1.CreatedAt = time.Now().Add(-10 * time.Second)
	startTime1 := time.Now().Add(-5 * time.Second)
	task1.StartedAt = &startTime1
	task1.Status = task.CompletedStatus
	require.NoError(t, q.Enqueue(task1))
	require.NoError(t, q.UpdateTask(task1))

	task2 := task.NewTask("test2", nil, task.MediumPriority)
	task2.CreatedAt = time.Now().Add(-8 * time.Second)
	startTime2 := time.Now().Add(-3 * time.Second)
	task2.StartedAt = &startTime2
	task2.Status = task.CompletedStatus
	require.NoError(t, q.Enqueue(task2))
	require.NoError(t, q.UpdateTask(task2))

	req := httptest.NewRequest("GET", "/api/dashboard/stats", nil)
	w := httptest.NewRecorder()

	dash.GetStats(w, req)

	var stats Stats
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &stats))

	assert.NotEqual(t, "N/A", stats.AverageWaitTime)
	assert.Contains(t, stats.AverageWaitTime, "s")
}

func TestGetStats_NoStartedTasks(t *testing.T) {
	dash, q, mr := setupTestDashboard(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	task1 := task.NewTask("pending1", nil, task.MediumPriority)
	task1.Status = task.PendingStatus
	require.NoError(t, q.Enqueue(task1))
	require.NoError(t, q.UpdateTask(task1))

	task2 := task.NewTask("pending2", nil, task.MediumPriority)
	task2.Status = task.PendingStatus
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

	tsk := task.NewTask("completed_task", map[string]any{"data": "test"}, task.MediumPriority)
	tsk.Status = task.CompletedStatus
	startTime := time.Now().Add(-5 * time.Second)
	completedTime := time.Now()
	tsk.StartedAt = &startTime
	tsk.CompletedAt = &completedTime
	require.NoError(t, q.Enqueue(tsk))
	require.NoError(t, q.UpdateTask(tsk))

	req := httptest.NewRequest("GET", "/api/dashboard/history", nil)
	w := httptest.NewRecorder()

	dash.GetRecentTasks(w, req)

	assert.Equal(t, 200, w.Code)

	var history []TaskHistory
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &history))

	assert.Len(t, history, 1)
	assert.Equal(t, tsk.ID, history[0].TaskID)
	assert.Equal(t, tsk.Type, history[0].Type)
	assert.Equal(t, tsk.Status, history[0].Status)
	assert.NotEmpty(t, history[0].Duration)
}

func TestGetRecentTasks_OnlyCompletedOrFailed(t *testing.T) {
	dash, q, mr := setupTestDashboard(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	pending := task.NewTask("pending", nil, task.MediumPriority)
	pending.Status = task.PendingStatus
	require.NoError(t, q.Enqueue(pending))
	require.NoError(t, q.UpdateTask(pending))

	running := task.NewTask("running", nil, task.MediumPriority)
	running.Status = task.RunningStatus
	require.NoError(t, q.Enqueue(running))
	require.NoError(t, q.UpdateTask(running))

	completed := task.NewTask("completed", nil, task.MediumPriority)
	completed.Status = task.CompletedStatus
	now := time.Now()
	completed.CompletedAt = &now
	require.NoError(t, q.Enqueue(completed))
	require.NoError(t, q.UpdateTask(completed))

	failed := task.NewTask("failed", nil, task.MediumPriority)
	failed.Status = task.FailedStatus
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

	old := task.NewTask("old_task", nil, task.MediumPriority)
	old.Status = task.CompletedStatus
	oldTime := time.Now().Add(-25 * time.Hour)
	old.CompletedAt = &oldTime
	require.NoError(t, q.Enqueue(old))
	require.NoError(t, q.UpdateTask(old))

	recent := task.NewTask("recent_task", nil, task.MediumPriority)
	recent.Status = task.CompletedStatus
	recentTime := time.Now().Add(-1 * time.Hour)
	recent.CompletedAt = &recentTime
	require.NoError(t, q.Enqueue(recent))
	require.NoError(t, q.UpdateTask(recent))

	veryRecent := task.NewTask("very_recent", nil, task.MediumPriority)
	veryRecent.Status = task.CompletedStatus
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

	tsk := task.NewTask("timed_task", nil, task.MediumPriority)
	tsk.Status = task.CompletedStatus
	tsk.CreatedAt = time.Now().Add(-10 * time.Second)
	startTime := time.Now().Add(-8 * time.Second)
	completedTime := time.Now().Add(-3 * time.Second)
	tsk.StartedAt = &startTime
	tsk.CompletedAt = &completedTime
	require.NoError(t, q.Enqueue(tsk))
	require.NoError(t, q.UpdateTask(tsk))

	req := httptest.NewRequest("GET", "/api/dashboard/history", nil)
	w := httptest.NewRecorder()

	dash.GetRecentTasks(w, req)

	var history []TaskHistory
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &history))

	require.Len(t, history, 1)
	assert.NotEmpty(t, history[0].Duration)
	assert.Contains(t, history[0].Duration, "s")
}

func TestGetRecentTasks_NoDuration_WhenNotStarted(t *testing.T) {
	dash, q, mr := setupTestDashboard(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	tsk := task.NewTask("no_start", nil, task.MediumPriority)
	tsk.Status = task.CompletedStatus
	now := time.Now()
	tsk.CompletedAt = &now
	require.NoError(t, q.Enqueue(tsk))
	require.NoError(t, q.UpdateTask(tsk))

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
		tsk := task.NewTask("task", map[string]any{"id": i}, task.MediumPriority)
		tsk.Status = task.CompletedStatus
		completedTime := now.Add(-time.Duration(i) * time.Hour)
		tsk.CompletedAt = &completedTime
		require.NoError(t, q.Enqueue(tsk))
		require.NoError(t, q.UpdateTask(tsk))
	}

	req := httptest.NewRequest("GET", "/api/dashboard/history", nil)
	w := httptest.NewRecorder()

	dash.GetRecentTasks(w, req)

	var history []TaskHistory
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &history))

	assert.Len(t, history, 5)

	for _, h := range history {
		assert.Equal(t, task.CompletedStatus, h.Status)
		assert.NotEmpty(t, h.TaskID)
		assert.NotZero(t, h.CreatedAt)
	}
}

func TestGetStats_MixedStatusCounts(t *testing.T) {
	dash, q, mr := setupTestDashboard(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	for range 10 {
		tsk := task.NewTask("pending", nil, task.MediumPriority)
		tsk.Status = task.PendingStatus
		require.NoError(t, q.Enqueue(tsk))
		require.NoError(t, q.UpdateTask(tsk))
	}

	for range 5 {
		tsk := task.NewTask("running", nil, task.MediumPriority)
		tsk.Status = task.RunningStatus
		require.NoError(t, q.Enqueue(tsk))
		require.NoError(t, q.UpdateTask(tsk))
	}

	for range 3 {
		tsk := task.NewTask("completed", nil, task.MediumPriority)
		tsk.Status = task.CompletedStatus
		require.NoError(t, q.Enqueue(tsk))
		require.NoError(t, q.UpdateTask(tsk))
	}

	for range 2 {
		tsk := task.NewTask("failed", nil, task.MediumPriority)
		tsk.Status = task.FailedStatus
		require.NoError(t, q.Enqueue(tsk))
		require.NoError(t, q.UpdateTask(tsk))
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

func TestGetStatsWithRepository(t *testing.T) {
	dash, q, mockRepo, mr := setupTestDashboardWithMockRepo(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	mockRepo.TaskStats = []repository.TaskStats{
		{
			Type:          "send_email",
			Status:        "completed",
			Count:         50,
			AvgDurationMs: 234.5,
			MaxDurationMs: 1000,
			MinDurationMs: 50,
			AvgRetries:    0.3,
		},
		{
			Type:          "send_email",
			Status:        "failed",
			Count:         5,
			AvgDurationMs: 180.0,
			AvgRetries:    2.1,
		},
		{
			Type:          "process_payment",
			Status:        "completed",
			Count:         100,
			AvgDurationMs: 450.2,
			MaxDurationMs: 2000,
			MinDurationMs: 100,
			AvgRetries:    0.1,
		},
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/dashboard/stats", nil)

	dash.GetStats(w, r)

	assert.Equal(t, 200, w.Code)

	var stats map[string]any
	err := json.NewDecoder(w.Body).Decode(&stats)
	require.NoError(t, err)

	assert.NotEmpty(t, stats)
}

func TestGetRecentTasksWithRepository(t *testing.T) {
	dash, q, mockRepo, mr := setupTestDashboardWithMockRepo(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	now := time.Now()
	mockRepo.RecentTasks = []repository.RecentTask{
		{
			TaskID:      "task-1",
			Type:        "send_email",
			Status:      "completed",
			CreatedAt:   now.Add(-5 * time.Minute),
			CompletedAt: ptrTime(now.Add(-4 * time.Minute)),
			DurationMs:  ptrInt(60000),
			RetryCount:  0,
		},
		{
			TaskID:        "task-2",
			Type:          "process_payment",
			Status:        "failed",
			CreatedAt:     now.Add(-10 * time.Minute),
			CompletedAt:   ptrTime(now.Add(-9 * time.Minute)),
			DurationMs:    ptrInt(45000),
			RetryCount:    2,
			FailureReason: "Payment gateway timeout",
		},
		{
			TaskID:     "task-3",
			Type:       "send_email",
			Status:     "pending",
			CreatedAt:  now.Add(-2 * time.Minute),
			RetryCount: 0,
		},
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/dashboard/history", nil)

	dash.GetRecentTasks(w, r)

	assert.Equal(t, 200, w.Code)

	var tasks []any
	err := json.NewDecoder(w.Body).Decode(&tasks)
	require.NoError(t, err)
}

func TestGetStatsWithTasksByType(t *testing.T) {
	dash, q, mockRepo, mr := setupTestDashboardWithMockRepo(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	for range 3 {
		tsk := task.NewTask("send_email", map[string]any{}, task.MediumPriority)
		err := q.Enqueue(tsk)
		require.NoError(t, err)
	}

	for range 2 {
		tsk := task.NewTask("process_payment", map[string]any{}, task.HighPriority)
		err := q.Enqueue(tsk)
		require.NoError(t, err)
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/dashboard/stats", nil)

	dash.GetStats(w, r)

	assert.Equal(t, 200, w.Code)

	assert.Equal(t, 5, mockRepo.GetSaveTaskCallCount())
}

func TestGetStatsWithCompletedTasks(t *testing.T) {
	dash, q, mockRepo, mr := setupTestDashboardWithMockRepo(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	mockRepo.TaskStats = []repository.TaskStats{
		{
			Type:          "send_email",
			Status:        "completed",
			Count:         10,
			AvgDurationMs: 250.0,
			MaxDurationMs: 500,
			MinDurationMs: 100,
			AvgRetries:    0.2,
		},
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/dashboard/stats", nil)

	dash.GetStats(w, r)

	assert.Equal(t, 200, w.Code)
}

func TestGetRecentTasksWithVariedStatuses(t *testing.T) {
	dash, q, mockRepo, mr := setupTestDashboardWithMockRepo(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	now := time.Now()

	mockRepo.RecentTasks = []repository.RecentTask{
		{
			TaskID:      "completed-task",
			Type:        "test_task",
			Status:      "completed",
			CreatedAt:   now.Add(-5 * time.Minute),
			CompletedAt: ptrTime(now.Add(-4 * time.Minute)),
			DurationMs:  ptrInt(60000),
		},
		{
			TaskID:    "pending-task",
			Type:      "test_task",
			Status:    "pending",
			CreatedAt: now.Add(-1 * time.Minute),
		},
		{
			TaskID:    "running-task",
			Type:      "test_task",
			Status:    "running",
			CreatedAt: now.Add(-30 * time.Second),
		},
		{
			TaskID:        "failed-task",
			Type:          "test_task",
			Status:        "failed",
			CreatedAt:     now.Add(-10 * time.Minute),
			CompletedAt:   ptrTime(now.Add(-9 * time.Minute)),
			DurationMs:    ptrInt(30000),
			RetryCount:    3,
			FailureReason: "Max retries exceeded",
		},
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/dashboard/history", nil)

	dash.GetRecentTasks(w, r)

	assert.Equal(t, 200, w.Code)
}

func TestGetStatsPerformanceMetrics(t *testing.T) {
	dash, q, mockRepo, mr := setupTestDashboardWithMockRepo(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	// Setup performance data
	mockRepo.TaskStats = []repository.TaskStats{
		{
			Type:          "fast_task",
			Status:        "completed",
			Count:         100,
			AvgDurationMs: 50.0,
			MaxDurationMs: 100,
			MinDurationMs: 20,
			AvgRetries:    0.0,
		},
		{
			Type:          "slow_task",
			Status:        "completed",
			Count:         50,
			AvgDurationMs: 5000.0,
			MaxDurationMs: 10000,
			MinDurationMs: 2000,
			AvgRetries:    0.5,
		},
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/dashboard/stats", nil)

	dash.GetStats(w, r)

	assert.Equal(t, 200, w.Code)

	var stats map[string]any
	err := json.NewDecoder(w.Body).Decode(&stats)
	require.NoError(t, err)
	assert.NotEmpty(t, stats)
}

func TestGetRecentTasksOrdering(t *testing.T) {
	dash, q, mockRepo, mr := setupTestDashboardWithMockRepo(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	now := time.Now()

	mockRepo.RecentTasks = []repository.RecentTask{
		{
			TaskID:    "newest",
			Type:      "test",
			Status:    "pending",
			CreatedAt: now,
		},
		{
			TaskID:    "middle",
			Type:      "test",
			Status:    "completed",
			CreatedAt: now.Add(-5 * time.Minute),
		},
		{
			TaskID:    "oldest",
			Type:      "test",
			Status:    "completed",
			CreatedAt: now.Add(-10 * time.Minute),
		},
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/dashboard/history", nil)

	dash.GetRecentTasks(w, r)

	assert.Equal(t, 200, w.Code)
}

func TestGetStatsWithRetryMetrics(t *testing.T) {
	dash, q, mockRepo, mr := setupTestDashboardWithMockRepo(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	mockRepo.TaskStats = []repository.TaskStats{
		{
			Type:          "reliable_task",
			Status:        "completed",
			Count:         100,
			AvgDurationMs: 200.0,
			AvgRetries:    0.1,
		},
		{
			Type:          "flaky_task",
			Status:        "completed",
			Count:         50,
			AvgDurationMs: 300.0,
			AvgRetries:    2.5,
		},
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/dashboard/stats", nil)

	dash.GetStats(w, r)

	assert.Equal(t, 200, w.Code)
}

func TestGetRecentTasksWithFailureReasons(t *testing.T) {
	dash, q, mockRepo, mr := setupTestDashboardWithMockRepo(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	now := time.Now()

	mockRepo.RecentTasks = []repository.RecentTask{
		{
			TaskID:        "failed-1",
			Type:          "test",
			Status:        "failed",
			CreatedAt:     now.Add(-5 * time.Minute),
			CompletedAt:   ptrTime(now.Add(-4 * time.Minute)),
			RetryCount:    3,
			FailureReason: "Database connection timeout",
		},
		{
			TaskID:        "failed-2",
			Type:          "test",
			Status:        "dead_letter",
			CreatedAt:     now.Add(-10 * time.Minute),
			CompletedAt:   ptrTime(now.Add(-9 * time.Minute)),
			RetryCount:    5,
			FailureReason: "Max retries exceeded: Invalid credentials",
		},
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/dashboard/history", nil)

	dash.GetRecentTasks(w, r)

	assert.Equal(t, 200, w.Code)
}

func ptrTime(t time.Time) *time.Time {
	return &t
}

func ptrInt(i int) *int {
	return &i
}
