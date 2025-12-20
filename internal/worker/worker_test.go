package worker

import (
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/nadmax/nexq/internal/queue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestWorker(t *testing.T) (*Worker, *queue.Queue, *miniredis.Miniredis) {
	mr, err := miniredis.Run()
	require.NoError(t, err)

	q, err := queue.NewQueue(mr.Addr())
	require.NoError(t, err)

	w := NewWorker("test-worker", q)

	return w, q, mr
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

	handler := func(task *queue.Task) error {
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
	w.RegisterHandler("test_task", func(task *queue.Task) error {
		executed = true
		return nil
	})

	task := queue.NewTask("test_task", nil, queue.PriorityMedium)
	err := q.Enqueue(task)
	assert.NoError(t, err)

	w.processTask(task)

	assert.True(t, executed)

	updated, _ := q.GetTask(task.ID)
	assert.Equal(t, queue.StatusCompleted, updated.Status)
	assert.NotNil(t, updated.CompletedAt)
}

func TestProcessTask_Failure(t *testing.T) {
	w, q, mr := setupTestWorker(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	w.RegisterHandler("test_task", func(task *queue.Task) error {
		return errors.New("task failed")
	})

	task := queue.NewTask("test_task", nil, queue.PriorityMedium)
	task.MaxRetries = 1
	err := q.Enqueue(task)
	assert.NoError(t, err)

	w.processTask(task)

	updated, _ := q.GetTask(task.ID)
	assert.Equal(t, 1, updated.RetryCount)
}

func TestProcessTask_MaxRetriesExceeded(t *testing.T) {
	w, q, mr := setupTestWorker(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	w.RegisterHandler("test_task", func(task *queue.Task) error {
		return errors.New("task failed")
	})

	task := queue.NewTask("test_task", nil, queue.PriorityMedium)
	task.MaxRetries = 2
	task.RetryCount = 2
	err := q.Enqueue(task)
	assert.NoError(t, err)

	w.processTask(task)

	updated, _ := q.GetTask(task.ID)
	assert.Equal(t, queue.StatusFailed, updated.Status)
	assert.Contains(t, updated.Error, "task failed")
}

func TestProcessTask_NoHandler(t *testing.T) {
	w, q, mr := setupTestWorker(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	task := queue.NewTask("unknown_task", nil, queue.PriorityMedium)
	err := q.Enqueue(task)
	assert.NoError(t, err)

	w.processTask(task)

	updated, _ := q.GetTask(task.ID)
	assert.Equal(t, queue.StatusFailed, updated.Status)
	assert.Contains(t, updated.Error, "no handler")
}

func TestWorkerStartStop(t *testing.T) {
	w, q, mr := setupTestWorker(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	w.SetPollInterval(10 * time.Millisecond)

	processed := make(chan bool, 1)
	w.RegisterHandler("test_task", func(task *queue.Task) error {
		processed <- true
		return nil
	})

	go w.Start()

	time.Sleep(50 * time.Millisecond)

	task := queue.NewTask("test_task", nil, queue.PriorityMedium)
	err := q.Enqueue(task)
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
	w.RegisterHandler("test_task", func(task *queue.Task) error {
		count++
		return nil
	})

	for range 5 {
		task := queue.NewTask("test_task", nil, queue.PriorityMedium)
		_ = q.Enqueue(task)
	}

	for range 5 {
		task, _ := q.Dequeue()
		if task != nil {
			w.processTask(task)
		}
	}

	assert.Equal(t, 5, count)
}
