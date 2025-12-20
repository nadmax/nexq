package queue

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestQueue(t *testing.T) (*Queue, *miniredis.Miniredis) {
	mr, err := miniredis.Run()
	require.NoError(t, err)

	q, err := NewQueue(mr.Addr())
	require.NoError(t, err)

	return q, mr
}

func TestNewQueue(t *testing.T) {
	q, mr := setupTestQueue(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	assert.NotNil(t, q)
	assert.NotNil(t, q.client)
}

func TestNewQueue_InvalidAddress(t *testing.T) {
	_, err := NewQueue("invalid:99999")
	assert.Error(t, err)
}

func TestEnqueue(t *testing.T) {
	q, mr := setupTestQueue(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	task := NewTask("test_task", map[string]any{"key": "value"}, PriorityMedium)
	err := q.Enqueue(task)

	assert.NoError(t, err)
}

func TestEnqueueAndDequeue(t *testing.T) {
	q, mr := setupTestQueue(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	original := NewTask("test_task", map[string]any{"key": "value"}, PriorityMedium)
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

func TestPriorityOrdering(t *testing.T) {
	q, mr := setupTestQueue(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	lowPriorityTask := NewTask("low", nil, PriorityLow)
	mediumPriorityTask := NewTask("medium", nil, PriorityMedium)
	highPriorityTask := NewTask("high", nil, PriorityHigh)

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

	futureTask := NewTask("future", nil, PriorityLow)
	futureTask.ScheduledAt = time.Now().Add(10 * time.Second)

	nowTask := NewTask("now", nil, PriorityMedium)
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

	task := NewTask("test", nil, PriorityMedium)
	err := q.Enqueue(task)
	assert.NoError(t, err)

	task.Status = StatusCompleted
	err = q.UpdateTask(task)
	assert.NoError(t, err)

	retrieved, err := q.GetTask(task.ID)
	require.NoError(t, err)
	assert.Equal(t, StatusCompleted, retrieved.Status)
}

func TestGetTask(t *testing.T) {
	q, mr := setupTestQueue(t)
	defer mr.Close()
	defer func() { _ = q.Close() }()

	task := NewTask("test", map[string]any{"key": "value"}, PriorityMedium)
	err := q.Enqueue(task)
	assert.NoError(t, err)

	retrieved, err := q.GetTask(task.ID)

	require.NoError(t, err)
	assert.Equal(t, task.ID, retrieved.ID)
	assert.Equal(t, task.Type, retrieved.Type)
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

	task1 := NewTask("task1", nil, PriorityMedium)
	task2 := NewTask("task2", nil, PriorityMedium)
	task3 := NewTask("task3", nil, PriorityMedium)

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
