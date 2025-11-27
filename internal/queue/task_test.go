package queue

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewTask(t *testing.T) {
	payload := map[string]any{
		"to":      "test@example.com",
		"subject": "Test",
	}

	task := NewTask("send_email", payload)

	assert.NotEmpty(t, task.ID)
	assert.Equal(t, "send_email", task.Type)
	assert.Equal(t, payload, task.Payload)
	assert.Equal(t, PriorityNormal, task.Priority)
	assert.Equal(t, StatusPending, task.Status)
	assert.Equal(t, 3, task.MaxRetries)
	assert.Equal(t, 0, task.Retries)
	assert.False(t, task.CreatedAt.IsZero())
	assert.False(t, task.ScheduledAt.IsZero())
	assert.Nil(t, task.StartedAt)
	assert.Nil(t, task.CompletedAt)
}

func TestTaskToJSON(t *testing.T) {
	task := NewTask("test_task", map[string]any{"key": "value"})

	jsonStr, err := task.ToJSON()

	assert.NoError(t, err)
	assert.NotEmpty(t, jsonStr)
	assert.Contains(t, jsonStr, "test_task")
	assert.Contains(t, jsonStr, "key")
}

func TestTaskFromJSON(t *testing.T) {
	original := NewTask("test_task", map[string]any{"key": "value"})
	jsonStr, _ := original.ToJSON()

	restored, err := TaskFromJSON(jsonStr)

	assert.NoError(t, err)
	assert.Equal(t, original.ID, restored.ID)
	assert.Equal(t, original.Type, restored.Type)
	assert.Equal(t, original.Status, restored.Status)
	assert.Equal(t, original.Priority, restored.Priority)
}

func TestTaskFromJSON_InvalidJSON(t *testing.T) {
	_, err := TaskFromJSON("invalid json")

	assert.Error(t, err)
}

func TestTaskStatuses(t *testing.T) {
	assert.Equal(t, TaskStatus("pending"), StatusPending)
	assert.Equal(t, TaskStatus("running"), StatusRunning)
	assert.Equal(t, TaskStatus("completed"), StatusCompleted)
	assert.Equal(t, TaskStatus("failed"), StatusFailed)
}

func TestTaskPriorities(t *testing.T) {
	assert.Equal(t, TaskPriority(0), PriorityLow)
	assert.Equal(t, TaskPriority(5), PriorityNormal)
	assert.Equal(t, TaskPriority(10), PriorityHigh)
}

func TestTaskJSONRoundTrip(t *testing.T) {
	now := time.Now()
	task := &Task{
		ID:          "test-123",
		Type:        "email",
		Payload:     map[string]any{"to": "test@example.com"},
		Priority:    PriorityHigh,
		Status:      StatusRunning,
		MaxRetries:  5,
		Retries:     2,
		CreatedAt:   now,
		ScheduledAt: now,
		StartedAt:   &now,
		Error:       "test error",
	}

	jsonStr, err := task.ToJSON()
	assert.NoError(t, err)

	restored, err := TaskFromJSON(jsonStr)
	assert.NoError(t, err)

	assert.Equal(t, task.ID, restored.ID)
	assert.Equal(t, task.Type, restored.Type)
	assert.Equal(t, task.Priority, restored.Priority)
	assert.Equal(t, task.Status, restored.Status)
	assert.Equal(t, task.MaxRetries, restored.MaxRetries)
	assert.Equal(t, task.Retries, restored.Retries)
	assert.Equal(t, task.Error, restored.Error)
}
