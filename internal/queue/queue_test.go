package queue

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/nadmax/nexq/internal/repository/mocks"
	"github.com/nadmax/nexq/internal/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestQueue(t *testing.T) (*Queue, *miniredis.Miniredis) {
	mr, err := miniredis.Run()
	require.NoError(t, err)

	q, err := NewQueue(mr.Addr(), nil)
	require.NoError(t, err)

	return q, mr
}

func setupTestQueueWithMockRepo(t *testing.T) (*Queue, *mocks.MockPostgresRepository, *miniredis.Miniredis) {
	mr, err := miniredis.Run()
	require.NoError(t, err)

	mockRepo := mocks.NewMockPostgresRepository()
	q, err := NewQueue(mr.Addr(), mockRepo)
	require.NoError(t, err)

	return q, mockRepo, mr
}

func TestNewQueue(t *testing.T) {
	q, mr := setupTestQueue(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	assert.NotNil(t, q)
	assert.NotNil(t, q.client)
}

func TestNewQueue_InvalidAddress(t *testing.T) {
	_, err := NewQueue("invalid:99999", nil)
	assert.Error(t, err)
}

func TestEnqueue(t *testing.T) {
	q, mr := setupTestQueue(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	tsk := task.NewTask("test_task", map[string]any{"key": "value"}, task.MediumPriority)
	err := q.Enqueue(tsk)

	assert.NoError(t, err)
}

func TestEnqueueWithRepository(t *testing.T) {
	q, mockRepo, mr := setupTestQueueWithMockRepo(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	tsk := task.NewTask("test_task", map[string]any{"key": "value"}, task.MediumPriority)
	err := q.Enqueue(tsk)
	require.NoError(t, err)

	assert.Equal(t, 1, mockRepo.GetSaveTaskCallCount())
	assert.True(t, mockRepo.WasTaskSaved(tsk.ID))

	status, exists := mockRepo.GetTaskStatus(tsk.ID)
	assert.True(t, exists)
	assert.Equal(t, task.PendingStatus, status)
}

func TestEnqueueAndDequeue(t *testing.T) {
	q, mr := setupTestQueue(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	original := task.NewTask("test_task", map[string]any{"key": "value"}, task.MediumPriority)
	err := q.Enqueue(original)
	require.NoError(t, err)

	dequeued, err := q.Dequeue()
	require.NoError(t, err)
	require.NotNil(t, dequeued)

	assert.Equal(t, original.ID, dequeued.ID)
	assert.Equal(t, original.Type, dequeued.Type)
	assert.Equal(t, original.Status, dequeued.Status)
}

func TestDequeue_EmptyQueue(t *testing.T) {
	q, mr := setupTestQueue(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	task, err := q.Dequeue()

	assert.NoError(t, err)
	assert.Nil(t, task)
}

func TestDequeueWithRepository(t *testing.T) {
	q, mockRepo, mr := setupTestQueueWithMockRepo(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	tsk := task.NewTask("test_task", map[string]any{"key": "value"}, task.MediumPriority)
	err := q.Enqueue(tsk)
	require.NoError(t, err)

	dequeuedTask, err := q.Dequeue()
	require.NoError(t, err)
	require.NotNil(t, dequeuedTask)

	assert.Equal(t, 1, mockRepo.GetUpdateTaskStatusCallCount())
	status, _ := mockRepo.GetTaskStatus(tsk.ID)
	assert.Equal(t, task.RunningStatus, status)
}

func TestPriorityOrdering(t *testing.T) {
	q, mr := setupTestQueue(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	lowPriorityTask := task.NewTask("low", nil, task.LowPriority)
	mediumPriorityTask := task.NewTask("medium", nil, task.MediumPriority)
	highPriorityTask := task.NewTask("high", nil, task.HighPriority)

	err := q.Enqueue(highPriorityTask)
	assert.NoError(t, err)
	err = q.Enqueue(mediumPriorityTask)
	assert.NoError(t, err)
	err = q.Enqueue(lowPriorityTask)
	assert.NoError(t, err)

	first, err := q.Dequeue()
	assert.NoError(t, err)
	assert.Equal(t, "high", first.Type)

	second, err := q.Dequeue()
	assert.NoError(t, err)
	assert.Equal(t, "medium", second.Type)

	third, err := q.Dequeue()
	assert.NoError(t, err)
	assert.Equal(t, "low", third.Type)
}

func TestScheduledTasks(t *testing.T) {
	q, mr := setupTestQueue(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	futureTask := task.NewTask("future", nil, task.LowPriority)
	futureTask.ScheduledAt = time.Now().Add(10 * time.Second)

	nowTask := task.NewTask("now", nil, task.MediumPriority)
	nowTask.ScheduledAt = time.Now()

	err := q.Enqueue(nowTask)
	assert.NoError(t, err)
	err = q.Enqueue(futureTask)
	assert.NoError(t, err)

	dequeued, err := q.Dequeue()
	assert.NoError(t, err)
	assert.NotNil(t, dequeued)
	assert.Equal(t, "now", dequeued.Type)

	dequeued2, err := q.Dequeue()
	assert.NoError(t, err)
	assert.NotNil(t, dequeued2)
}

func TestUpdateTask(t *testing.T) {
	q, mr := setupTestQueue(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	tsk := task.NewTask("test", nil, task.MediumPriority)
	err := q.Enqueue(tsk)
	assert.NoError(t, err)

	tsk.Status = task.CompletedStatus
	err = q.UpdateTask(tsk)
	assert.NoError(t, err)

	retrieved, err := q.GetTask(tsk.ID)
	require.NoError(t, err)
	assert.Equal(t, task.CompletedStatus, retrieved.Status)
}

func TestGetTask(t *testing.T) {
	q, mr := setupTestQueue(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	tsk := task.NewTask("test", map[string]any{"key": "value"}, task.MediumPriority)
	err := q.Enqueue(tsk)
	assert.NoError(t, err)

	retrieved, err := q.GetTask(tsk.ID)

	require.NoError(t, err)
	assert.Equal(t, tsk.ID, retrieved.ID)
	assert.Equal(t, tsk.Type, retrieved.Type)
}

func TestGetTask_NotFound(t *testing.T) {
	q, mr := setupTestQueue(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	_, err := q.GetTask("non-existent-id")

	assert.Error(t, err)
}

func TestGetAllTasks(t *testing.T) {
	q, mr := setupTestQueue(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	task1 := task.NewTask("task1", nil, task.MediumPriority)
	task2 := task.NewTask("task2", nil, task.MediumPriority)
	task3 := task.NewTask("task3", nil, task.MediumPriority)

	err := q.Enqueue(task1)
	assert.NoError(t, err)
	err = q.Enqueue(task2)
	assert.NoError(t, err)
	err = q.Enqueue(task3)
	assert.NoError(t, err)

	tasks, err := q.GetAllTasks()

	require.NoError(t, err)
	assert.Len(t, tasks, 3)
}

func TestGetAllTasks_Empty(t *testing.T) {
	q, mr := setupTestQueue(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	tasks, err := q.GetAllTasks()

	require.NoError(t, err)
	assert.Len(t, tasks, 0)
}

func TestClose(t *testing.T) {
	q, mr := setupTestQueue(t)
	defer mr.Close()

	err := q.Close()
	assert.NoError(t, err)
}

func TestCompleteTaskWithRepository(t *testing.T) {
	q, mockRepo, mr := setupTestQueueWithMockRepo(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	tsk := task.NewTask("test_task", map[string]any{}, task.MediumPriority)
	err := q.Enqueue(tsk)
	require.NoError(t, err)

	durationMs := 250
	err = q.CompleteTask(tsk, durationMs)
	require.NoError(t, err)

	assert.Equal(t, 1, mockRepo.GetCompleteTaskCallCount())

	completeCall := mockRepo.CompleteTaskCalls[0]
	assert.Equal(t, tsk.ID, completeCall.TaskID)
	assert.Equal(t, durationMs, completeCall.DurationMs)
}

func TestFailTaskWithRepository(t *testing.T) {
	q, mockRepo, mr := setupTestQueueWithMockRepo(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	tsk := task.NewTask("test_task", map[string]any{}, task.MediumPriority)
	err := q.Enqueue(tsk)
	require.NoError(t, err)

	reason := "Connection timeout"
	durationMs := 1500
	err = q.FailTask(tsk, reason, durationMs)
	require.NoError(t, err)

	assert.Equal(t, 1, mockRepo.GetFailTaskCallCount())

	failCall := mockRepo.FailTaskCalls[0]
	assert.Equal(t, tsk.ID, failCall.TaskID)
	assert.Equal(t, reason, failCall.Reason)
	assert.Equal(t, durationMs, failCall.DurationMs)
}

func TestMoveToDeadLetterWithRepository(t *testing.T) {
	q, mockRepo, mr := setupTestQueueWithMockRepo(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	tsk := task.NewTask("test_task", map[string]any{}, task.MediumPriority)
	err := q.Enqueue(tsk)
	require.NoError(t, err)

	reason := "Max retries exceeded"
	err = q.MoveToDeadLetter(tsk, reason)
	require.NoError(t, err)

	assert.Equal(t, 1, mockRepo.GetMoveToDLQCallCount())

	dlqCall := mockRepo.MoveTaskToDLQCalls[0]
	assert.Equal(t, tsk.ID, dlqCall.TaskID)
	assert.Equal(t, reason, dlqCall.Reason)
}

func TestIncrementRetryCountWithRepository(t *testing.T) {
	q, mockRepo, mr := setupTestQueueWithMockRepo(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	tsk := task.NewTask("test_task", map[string]any{}, task.MediumPriority)
	err := q.Enqueue(tsk)
	require.NoError(t, err)

	err = q.IncrementRetryCount(tsk.ID)
	require.NoError(t, err)

	assert.Len(t, mockRepo.IncrementRetryCalls, 1)
	assert.Equal(t, tsk.ID, mockRepo.IncrementRetryCalls[0])
}

func TestLogExecutionWithRepository(t *testing.T) {
	q, mockRepo, mr := setupTestQueueWithMockRepo(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	taskID := "test-task-123"
	attemptNumber := 2
	status := "running"
	durationMs := 350
	errorMsg := "some error"
	workerID := "worker-1"

	err := q.LogExecution(taskID, attemptNumber, status, durationMs, errorMsg, workerID)
	require.NoError(t, err)

	// Verify execution was logged
	assert.Equal(t, 1, mockRepo.GetLogExecutionCallCount())

	execCall := mockRepo.LogExecutionCalls[0]
	assert.Equal(t, taskID, execCall.TaskID)
	assert.Equal(t, attemptNumber, execCall.AttemptNumber)
	assert.Equal(t, status, execCall.Status)
	assert.Equal(t, durationMs, execCall.DurationMs)
	assert.Equal(t, errorMsg, execCall.ErrorMsg)
	assert.Equal(t, workerID, execCall.WorkerID)
}

func TestQueueWithNilRepository(t *testing.T) {
	q, mr := setupTestQueue(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	tsk := task.NewTask("test_task", map[string]any{}, task.MediumPriority)

	err := q.Enqueue(tsk)
	require.NoError(t, err)

	err = q.CompleteTask(tsk, 100)
	require.NoError(t, err)

	err = q.FailTask(tsk, "error", 100)
	require.NoError(t, err)

	err = q.IncrementRetryCount(tsk.ID)
	require.NoError(t, err)

	err = q.LogExecution(tsk.ID, 1, "running", 100, "", "worker-1")
	require.NoError(t, err)
}

func TestUpdateTaskWithRepository(t *testing.T) {
	q, mockRepo, mr := setupTestQueueWithMockRepo(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	tsk := task.NewTask("test_task", map[string]any{}, task.MediumPriority)
	err := q.Enqueue(tsk)
	require.NoError(t, err)

	tsk.Status = task.RunningStatus
	tsk.RetryCount = 1
	err = q.UpdateTask(tsk)
	require.NoError(t, err)

	assert.Equal(t, 2, mockRepo.GetSaveTaskCallCount())
}

func TestMultipleTasksWithRepository(t *testing.T) {
	q, mockRepo, mr := setupTestQueueWithMockRepo(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	taskIDs := []string{}
	for i := range 5 {
		tsk := task.NewTask("test_task", map[string]any{"index": i}, task.MediumPriority)
		err := q.Enqueue(tsk)
		require.NoError(t, err)
		taskIDs = append(taskIDs, tsk.ID)
	}

	assert.Equal(t, 5, mockRepo.GetSaveTaskCallCount())

	for _, taskID := range taskIDs {
		assert.True(t, mockRepo.WasTaskSaved(taskID))
	}
}

func TestGetRepository(t *testing.T) {
	q, mockRepo, mr := setupTestQueueWithMockRepo(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	repo := q.GetRepository()
	assert.NotNil(t, repo)
	assert.Equal(t, mockRepo, repo)
}

func TestGetRepositoryWithNilRepo(t *testing.T) {
	q, mr := setupTestQueue(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	repo := q.GetRepository()
	assert.Nil(t, repo)
}

func TestCancelTask(t *testing.T) {
	q, mr := setupTestQueue(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	tsk := task.NewTask("test_task", map[string]any{}, task.MediumPriority)
	tsk.Status = task.PendingStatus
	err := q.Enqueue(tsk)
	require.NoError(t, err)

	err = q.CancelTask(tsk.ID)
	require.NoError(t, err)

	retrieved, err := q.GetTask(tsk.ID)
	require.NoError(t, err)
	assert.Equal(t, task.CancelledStatus, retrieved.Status)
	assert.NotNil(t, retrieved.CompletedAt)
}

func TestCancelTask_NotFound(t *testing.T) {
	q, mr := setupTestQueue(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	err := q.CancelTask("non-existent-id")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "task not found")
}

func TestCancelTask_CompletedTask(t *testing.T) {
	q, mr := setupTestQueue(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	tsk := task.NewTask("test_task", map[string]any{}, task.MediumPriority)
	tsk.Status = task.CompletedStatus
	err := q.Enqueue(tsk)
	require.NoError(t, err)

	err = q.CancelTask(tsk.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot cancel task with status")
}

func TestCancelTask_RunningTask(t *testing.T) {
	q, mr := setupTestQueue(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	tsk := task.NewTask("test_task", map[string]any{}, task.MediumPriority)
	tsk.Status = task.RunningStatus
	err := q.Enqueue(tsk)
	require.NoError(t, err)

	err = q.CancelTask(tsk.ID)
	require.NoError(t, err)

	retrieved, err := q.GetTask(tsk.ID)
	require.NoError(t, err)
	assert.Equal(t, task.CancelledStatus, retrieved.Status)
}

func TestCancelTaskWithRepository(t *testing.T) {
	q, mockRepo, mr := setupTestQueueWithMockRepo(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	tsk := task.NewTask("test_task", map[string]any{}, task.MediumPriority)
	tsk.Status = task.PendingStatus
	err := q.Enqueue(tsk)
	require.NoError(t, err)

	err = q.CancelTask(tsk.ID)
	require.NoError(t, err)

	assert.GreaterOrEqual(t, mockRepo.GetUpdateTaskStatusCallCount(), 1)
}

func TestIsCancelled(t *testing.T) {
	q, mr := setupTestQueue(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	tsk := task.NewTask("test_task", map[string]any{}, task.MediumPriority)
	tsk.Status = task.PendingStatus
	err := q.Enqueue(tsk)
	require.NoError(t, err)

	cancelled, err := q.IsCancelled(tsk.ID)
	require.NoError(t, err)
	assert.False(t, cancelled)

	err = q.CancelTask(tsk.ID)
	require.NoError(t, err)

	cancelled, err = q.IsCancelled(tsk.ID)
	require.NoError(t, err)
	assert.True(t, cancelled)
}

func TestIsCancelled_NotFound(t *testing.T) {
	q, mr := setupTestQueue(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	_, err := q.IsCancelled("non-existent-id")
	assert.Error(t, err)
}

func TestGetDeadLetterTasks_Empty(t *testing.T) {
	q, mr := setupTestQueue(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	tasks, err := q.GetDeadLetterTasks()
	require.NoError(t, err)
	assert.Len(t, tasks, 0)
}

func TestGetDeadLetterTasks(t *testing.T) {
	q, mr := setupTestQueue(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	tsk1 := task.NewTask("task1", map[string]any{}, task.MediumPriority)
	err := q.Enqueue(tsk1)
	require.NoError(t, err)
	err = q.MoveToDeadLetter(tsk1, "test reason 1")
	require.NoError(t, err)

	tsk2 := task.NewTask("task2", map[string]any{}, task.MediumPriority)
	err = q.Enqueue(tsk2)
	require.NoError(t, err)
	err = q.MoveToDeadLetter(tsk2, "test reason 2")
	require.NoError(t, err)

	dlqTasks, err := q.GetDeadLetterTasks()
	require.NoError(t, err)
	assert.Len(t, dlqTasks, 2)
}

func TestGetDeadLetterTask(t *testing.T) {
	q, mr := setupTestQueue(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	tsk := task.NewTask("test_task", map[string]any{"key": "value"}, task.MediumPriority)
	err := q.Enqueue(tsk)
	require.NoError(t, err)

	reason := "max retries exceeded"
	err = q.MoveToDeadLetter(tsk, reason)
	require.NoError(t, err)

	retrieved, err := q.GetDeadLetterTask(tsk.ID)
	require.NoError(t, err)
	assert.Equal(t, tsk.ID, retrieved.ID)
	assert.Equal(t, tsk.Type, retrieved.Type)
}

func TestGetDeadLetterTask_NotFound(t *testing.T) {
	q, mr := setupTestQueue(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	_, err := q.GetDeadLetterTask("non-existent-id")
	assert.Error(t, err)
}

func TestRetryDeadLetterTask(t *testing.T) {
	q, mr := setupTestQueue(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	tsk := task.NewTask("test_task", map[string]any{}, task.MediumPriority)
	tsk.RetryCount = 3
	tsk.FailureReason = "some error"
	err := q.Enqueue(tsk)
	require.NoError(t, err)

	err = q.MoveToDeadLetter(tsk, "max retries")
	require.NoError(t, err)

	err = q.RetryDeadLetterTask(tsk.ID)
	require.NoError(t, err)

	_, err = q.GetDeadLetterTask(tsk.ID)
	assert.Error(t, err)

	retrieved, err := q.GetTask(tsk.ID)
	require.NoError(t, err)
	assert.Equal(t, task.PendingStatus, retrieved.Status)
	assert.Equal(t, 0, retrieved.RetryCount)
	assert.Equal(t, "", retrieved.FailureReason)
}

func TestRetryDeadLetterTask_NotFound(t *testing.T) {
	q, mr := setupTestQueue(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	err := q.RetryDeadLetterTask("non-existent-id")
	assert.Error(t, err)
}

func TestPurgeDeadLetterTask(t *testing.T) {
	q, mr := setupTestQueue(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	tsk := task.NewTask("test_task", map[string]any{}, task.MediumPriority)
	err := q.Enqueue(tsk)
	require.NoError(t, err)

	err = q.MoveToDeadLetter(tsk, "test reason")
	require.NoError(t, err)

	// Verify task is in DLQ
	_, err = q.GetDeadLetterTask(tsk.ID)
	require.NoError(t, err)

	err = q.PurgeDeadLetterTask(tsk.ID)
	require.NoError(t, err)

	_, err = q.GetDeadLetterTask(tsk.ID)
	assert.Error(t, err)
}

func TestPurgeDeadLetterTask_NotFound(t *testing.T) {
	q, mr := setupTestQueue(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	err := q.PurgeDeadLetterTask("non-existent-id")
	assert.NoError(t, err)
}

func TestGetDeadLetterStats_Empty(t *testing.T) {
	q, mr := setupTestQueue(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	stats, err := q.GetDeadLetterStats()
	require.NoError(t, err)
	assert.Equal(t, 0, stats["total_tasks"])
}

func TestGetDeadLetterStats(t *testing.T) {
	q, mr := setupTestQueue(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	for i := range 5 {
		tsk := task.NewTask("test_task", map[string]any{"index": i}, task.MediumPriority)
		err := q.Enqueue(tsk)
		require.NoError(t, err)
		err = q.MoveToDeadLetter(tsk, "test reason")
		require.NoError(t, err)
	}

	stats, err := q.GetDeadLetterStats()
	require.NoError(t, err)
	assert.Equal(t, 5, stats["total_tasks"])
}

func TestUpdateMetrics(t *testing.T) {
	q, mr := setupTestQueue(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	pendingTask := task.NewTask("pending_task", map[string]any{}, task.MediumPriority)
	pendingTask.Status = task.PendingStatus
	err := q.Enqueue(pendingTask)
	require.NoError(t, err)

	runningTask := task.NewTask("running_task", map[string]any{}, task.MediumPriority)
	runningTask.Status = task.RunningStatus
	err = q.Enqueue(runningTask)
	require.NoError(t, err)

	dlqTask := task.NewTask("dlq_task", map[string]any{}, task.MediumPriority)
	err = q.Enqueue(dlqTask)
	require.NoError(t, err)
	err = q.MoveToDeadLetter(dlqTask, "test")
	require.NoError(t, err)

	err = q.UpdateMetrics()
	assert.NoError(t, err)
}

func TestUpdateMetrics_EmptyQueue(t *testing.T) {
	q, mr := setupTestQueue(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	err := q.UpdateMetrics()
	assert.NoError(t, err)
}

func TestDeadLetterWorkflow(t *testing.T) {
	q, mr := setupTestQueue(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	tsk := task.NewTask("workflow_task", map[string]any{"data": "test"}, task.HighPriority)
	err := q.Enqueue(tsk)
	require.NoError(t, err)

	err = q.MoveToDeadLetter(tsk, "max retries exceeded")
	require.NoError(t, err)

	dlqTasks, err := q.GetDeadLetterTasks()
	require.NoError(t, err)
	assert.Len(t, dlqTasks, 1)

	stats, err := q.GetDeadLetterStats()
	require.NoError(t, err)
	assert.Equal(t, 1, stats["total_tasks"])

	err = q.RetryDeadLetterTask(tsk.ID)
	require.NoError(t, err)

	dlqTasks, err = q.GetDeadLetterTasks()
	require.NoError(t, err)
	assert.Len(t, dlqTasks, 0)

	retrieved, err := q.GetTask(tsk.ID)
	require.NoError(t, err)
	assert.Equal(t, task.PendingStatus, retrieved.Status)
}

func TestCancelAndRetryWorkflow(t *testing.T) {
	q, mr := setupTestQueue(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	tsk := task.NewTask("cancel_test", map[string]any{}, task.MediumPriority)
	err := q.Enqueue(tsk)
	require.NoError(t, err)

	err = q.CancelTask(tsk.ID)
	require.NoError(t, err)

	cancelled, err := q.IsCancelled(tsk.ID)
	require.NoError(t, err)
	assert.True(t, cancelled)

	err = q.CancelTask(tsk.ID)
	assert.Error(t, err)
}
