package worker

import (
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/nadmax/nexq/internal/queue"
	"github.com/nadmax/nexq/internal/repository"
	"github.com/nadmax/nexq/internal/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestWorker(t *testing.T) (*Worker, *queue.Queue, *miniredis.Miniredis) {
	mr, err := miniredis.Run()
	require.NoError(t, err)

	q, err := queue.NewQueue(mr.Addr(), nil)
	require.NoError(t, err)

	w := NewWorker("test-worker", q)

	return w, q, mr
}

func setupTestWorkerWithMockRepo(t *testing.T) (*Worker, *queue.Queue, *repository.MockPostgresRepository, *miniredis.Miniredis) {
	mr, err := miniredis.Run()
	require.NoError(t, err)

	mockRepo := repository.NewMockPostgresRepository()
	q, err := queue.NewQueue(mr.Addr(), mockRepo)
	require.NoError(t, err)

	w := NewWorker("test-worker", q)
	w.SetPollInterval(10 * time.Millisecond)

	return w, q, mockRepo, mr
}

func TestNewWorker(t *testing.T) {
	w, q, mr := setupTestWorker(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	assert.NotNil(t, w)
	assert.Equal(t, "test-worker", w.id)
	assert.NotNil(t, w.handlers)
	assert.NotNil(t, w.stop)
}

func TestRegisterHandler(t *testing.T) {
	w, q, mr := setupTestWorker(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	handler := func(t *task.Task) error {
		return nil
	}

	w.RegisterHandler("test_task", handler)

	assert.Contains(t, w.handlers, "test_task")
}

func TestProcessTask_Success(t *testing.T) {
	w, q, mr := setupTestWorker(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	executed := false
	w.RegisterHandler("test_task", func(tsk *task.Task) error {
		executed = true
		return nil
	})

	tsk := task.NewTask("test_task", nil, task.PriorityMedium)
	err := q.Enqueue(tsk)
	assert.NoError(t, err)

	w.processTask(tsk)

	assert.True(t, executed)

	updated, _ := q.GetTask(tsk.ID)
	assert.Equal(t, task.StatusCompleted, updated.Status)
	assert.NotNil(t, updated.CompletedAt)
}

func TestProcessTask_Failure(t *testing.T) {
	w, q, mr := setupTestWorker(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	w.RegisterHandler("test_task", func(tsk *task.Task) error {
		return errors.New("task failed")
	})

	tsk := task.NewTask("test_task", nil, task.PriorityMedium)
	tsk.MaxRetries = 1
	err := q.Enqueue(tsk)
	assert.NoError(t, err)

	w.processTask(tsk)

	updated, _ := q.GetTask(tsk.ID)
	assert.Equal(t, 1, updated.RetryCount)
}

func TestProcessTask_MaxRetriesExceeded(t *testing.T) {
	w, q, mr := setupTestWorker(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	w.RegisterHandler("test_task", func(tsk *task.Task) error {
		return errors.New("task failed")
	})

	tsk := task.NewTask("test_task", nil, task.PriorityMedium)
	tsk.MaxRetries = 2
	tsk.RetryCount = 2
	err := q.Enqueue(tsk)
	assert.NoError(t, err)

	w.processTask(tsk)

	updated, _ := q.GetTask(tsk.ID)
	assert.Equal(t, task.StatusFailed, updated.Status)
	assert.Contains(t, updated.Error, "task failed")
}

func TestProcessTask_NoHandler(t *testing.T) {
	w, q, mr := setupTestWorker(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	tsk := task.NewTask("unknown_task", nil, task.PriorityMedium)
	err := q.Enqueue(tsk)
	assert.NoError(t, err)

	w.processTask(tsk)

	updated, _ := q.GetTask(tsk.ID)
	assert.Equal(t, task.StatusPending, updated.Status)
	assert.Contains(t, updated.Error, "no handler")
}

func TestWorkerStartStop(t *testing.T) {
	w, q, mr := setupTestWorker(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	w.SetPollInterval(10 * time.Millisecond)

	processed := make(chan bool, 1)
	w.RegisterHandler("test_task", func(tsk *task.Task) error {
		processed <- true
		return nil
	})

	go w.Start()

	time.Sleep(50 * time.Millisecond)

	tsk := task.NewTask("test_task", nil, task.PriorityMedium)
	err := q.Enqueue(tsk)
	assert.NoError(t, err)

	select {
	case <-processed:
	case <-time.After(5 * time.Second):
		t.Fatal("Task was not processed")
	}

	w.Stop()
	time.Sleep(50 * time.Millisecond)
}

func TestWorkerProcessMultipleTasks(t *testing.T) {
	w, q, mr := setupTestWorker(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	count := 0
	w.RegisterHandler("test_task", func(tsk *task.Task) error {
		count++
		return nil
	})

	for range 5 {
		tsk := task.NewTask("test_task", nil, task.PriorityMedium)
		_ = q.Enqueue(tsk)
	}

	for range 5 {
		tsk, _ := q.Dequeue()
		if tsk != nil {
			w.processTask(tsk)
		}
	}

	assert.Equal(t, 5, count)
}

func TestWorkerProcessTaskSuccessWithHistory(t *testing.T) {
	w, q, mockRepo, mr := setupTestWorkerWithMockRepo(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	w.RegisterHandler("test_task", func(tsk *task.Task) error {
		time.Sleep(50 * time.Millisecond)
		return nil
	})

	tsk := task.NewTask("test_task", map[string]any{"key": "value"}, task.PriorityMedium)
	err := q.Enqueue(tsk)
	require.NoError(t, err)

	assert.Equal(t, 1, mockRepo.GetSaveTaskCallCount())

	retrievedTask, err := q.Dequeue()
	require.NoError(t, err)
	require.NotNil(t, retrievedTask)

	w.processTask(retrievedTask)

	execLogs := mockRepo.GetExecutionLogForTask(tsk.ID)
	assert.Len(t, execLogs, 2, "Should have start and completion logs")

	assert.Equal(t, string(task.StatusRunning), execLogs[0].Status)
	assert.Equal(t, "test-worker", execLogs[0].WorkerID)
	assert.Equal(t, string(task.StatusCompleted), execLogs[1].Status)
	assert.Greater(t, execLogs[1].DurationMs, 0, "Duration should be recorded")
	assert.Equal(t, 1, mockRepo.GetCompleteTaskCallCount())
}

func TestWorkerProcessTaskFailureWithRetry(t *testing.T) {
	w, q, mockRepo, mr := setupTestWorkerWithMockRepo(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	w.RegisterHandler("test_task", func(tsk *task.Task) error {
		return errors.New("task failed")
	})

	tsk := task.NewTask("test_task", map[string]any{"key": "value"}, task.PriorityMedium)
	tsk.MaxRetries = 3
	err := q.Enqueue(tsk)
	require.NoError(t, err)

	retrievedTask, err := q.Dequeue()
	require.NoError(t, err)
	require.NotNil(t, retrievedTask)

	w.processTask(retrievedTask)

	execLogs := mockRepo.GetExecutionLogForTask(tsk.ID)
	assert.Len(t, execLogs, 2, "Should have start and failure logs")

	assert.Equal(t, string(task.StatusFailed), execLogs[1].Status)
	assert.Equal(t, "task failed", execLogs[1].ErrorMsg)
	assert.Equal(t, 1, mockRepo.GetFailTaskCallCount())
	assert.Equal(t, 1, mockRepo.GetIncrementRetryCallCount())
}

func TestWorkerProcessTaskFailurePermanent(t *testing.T) {
	w, q, mockRepo, mr := setupTestWorkerWithMockRepo(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	w.RegisterHandler("test_task", func(task *task.Task) error {
		return errors.New("permanent failure")
	})

	tsk := task.NewTask("test_task", map[string]any{"key": "value"}, task.PriorityMedium)
	tsk.MaxRetries = 1
	tsk.RetryCount = 0
	err := q.Enqueue(tsk)
	require.NoError(t, err)

	retrievedTask, err := q.Dequeue()
	require.NoError(t, err)
	require.NotNil(t, retrievedTask)

	w.processTask(retrievedTask)

	assert.Equal(t, 0, mockRepo.GetFailTaskCallCount())
}

func TestWorkerProcessTaskNoHandler(t *testing.T) {
	w, q, mockRepo, mr := setupTestWorkerWithMockRepo(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	tsk := task.NewTask("unknown_task", map[string]any{"key": "value"}, task.PriorityMedium)
	err := q.Enqueue(tsk)
	require.NoError(t, err)

	// Process the task
	retrievedTask, err := q.Dequeue()
	require.NoError(t, err)
	require.NotNil(t, retrievedTask)

	w.processTask(retrievedTask)

	execLogs := mockRepo.GetExecutionLogForTask(tsk.ID)
	assert.Len(t, execLogs, 2, "Should have start and failure logs")
	assert.Contains(t, execLogs[1].ErrorMsg, "no handler for task type")

	assert.Equal(t, 1, mockRepo.GetFailTaskCallCount())
}

func TestWorkerMultipleTasks(t *testing.T) {
	w, q, mockRepo, mr := setupTestWorkerWithMockRepo(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	processedTasks := 0
	w.RegisterHandler("test_task", func(tsk *task.Task) error {
		processedTasks++
		return nil
	})

	for i := range 5 {
		tsk := task.NewTask("test_task", map[string]any{"index": i}, task.PriorityMedium)
		err := q.Enqueue(tsk)
		require.NoError(t, err)
	}

	for range 5 {
		retrievedTask, err := q.Dequeue()
		require.NoError(t, err)
		if retrievedTask != nil {
			w.processTask(retrievedTask)
		}
	}

	assert.Equal(t, 5, processedTasks, "All tasks should be processed")
	assert.Equal(t, 15, mockRepo.GetSaveTaskCallCount(), "All tasks should be saved")
	assert.Equal(t, 5, mockRepo.GetCompleteTaskCallCount(), "All tasks should be completed")
}

func TestWorkerExecutionDurationTracking(t *testing.T) {
	w, q, mockRepo, mr := setupTestWorkerWithMockRepo(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	w.RegisterHandler("test_task", func(tsk *task.Task) error {
		time.Sleep(100 * time.Millisecond)
		return nil
	})

	tsk := task.NewTask("test_task", map[string]any{}, task.PriorityMedium)
	err := q.Enqueue(tsk)
	require.NoError(t, err)

	retrievedTask, err := q.Dequeue()
	require.NoError(t, err)

	w.processTask(retrievedTask)

	execLogs := mockRepo.GetExecutionLogForTask(tsk.ID)
	completionLog := execLogs[1] // Second log is completion

	assert.Greater(t, completionLog.DurationMs, 90, "Duration should be at least 90ms")
	assert.Less(t, completionLog.DurationMs, 200, "Duration should be less than 200ms")
}

func TestWorkerIDTracking(t *testing.T) {
	w, q, mockRepo, mr := setupTestWorkerWithMockRepo(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	w.RegisterHandler("test_task", func(tsk *task.Task) error {
		return nil
	})

	tsk := task.NewTask("test_task", map[string]any{}, task.PriorityMedium)
	err := q.Enqueue(tsk)
	require.NoError(t, err)

	retrievedTask, err := q.Dequeue()
	require.NoError(t, err)

	w.processTask(retrievedTask)

	execLogs := mockRepo.GetExecutionLogForTask(tsk.ID)
	for _, log := range execLogs {
		assert.Equal(t, "test-worker", log.WorkerID, "Worker ID should be tracked")
	}
}
