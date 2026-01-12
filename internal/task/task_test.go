package task

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

	task := NewTask("send_email", payload, MediumPriority)

	assert.NotEmpty(t, task.ID)
	assert.Equal(t, "send_email", task.Type)
	assert.Equal(t, payload, task.Payload)
	assert.Equal(t, MediumPriority, task.Priority)
	assert.Equal(t, PendingStatus, task.Status)
	assert.Equal(t, 3, task.MaxRetries)
	assert.Equal(t, 0, task.RetryCount)
	assert.False(t, task.CreatedAt.IsZero())
	assert.False(t, task.ScheduledAt.IsZero())
	assert.Nil(t, task.StartedAt)
	assert.Nil(t, task.CompletedAt)
}

func TestTaskToJSON(t *testing.T) {
	task := NewTask("test_task", map[string]any{"key": "value"}, MediumPriority)

	jsonStr, err := task.ToJSON()

	assert.NoError(t, err)
	assert.NotEmpty(t, jsonStr)
	assert.Contains(t, jsonStr, "test_task")
	assert.Contains(t, jsonStr, "key")
}

func TestTaskFromJSON(t *testing.T) {
	original := NewTask("test_task", map[string]any{"key": "value"}, MediumPriority)
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
	assert.Equal(t, TaskStatus("pending"), PendingStatus)
	assert.Equal(t, TaskStatus("running"), RunningStatus)
	assert.Equal(t, TaskStatus("completed"), CompletedStatus)
	assert.Equal(t, TaskStatus("failed"), FailedStatus)
}

func TestTaskPriorities(t *testing.T) {
	assert.Equal(t, TaskPriority(0), LowPriority)
	assert.Equal(t, TaskPriority(1), MediumPriority)
	assert.Equal(t, TaskPriority(2), HighPriority)
}

func TestTaskJSONRoundTrip(t *testing.T) {
	now := time.Now()
	task := &Task{
		ID:          "test-123",
		Type:        "email",
		Payload:     map[string]any{"to": "test@example.com"},
		Priority:    HighPriority,
		Status:      RunningStatus,
		MaxRetries:  5,
		RetryCount:  2,
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
	assert.Equal(t, task.RetryCount, restored.RetryCount)
	assert.Equal(t, task.Error, restored.Error)
}

func TestTask_ShouldMoveToDeadLetter(t *testing.T) {
	tests := []struct {
		name       string
		retryCount int
		maxRetries int
		status     TaskStatus
		expected   bool
	}{
		{
			name:       "should move when retries exceeded and failed",
			retryCount: 3,
			maxRetries: 3,
			status:     FailedStatus,
			expected:   true,
		},
		{
			name:       "should move when retries exceeded beyond max and failed",
			retryCount: 5,
			maxRetries: 3,
			status:     FailedStatus,
			expected:   true,
		},
		{
			name:       "should not move when retries not exceeded",
			retryCount: 2,
			maxRetries: 3,
			status:     FailedStatus,
			expected:   false,
		},
		{
			name:       "should not move when status is not failed",
			retryCount: 3,
			maxRetries: 3,
			status:     PendingStatus,
			expected:   false,
		},
		{
			name:       "should not move when status is running",
			retryCount: 3,
			maxRetries: 3,
			status:     RunningStatus,
			expected:   false,
		},
		{
			name:       "should not move when status is completed",
			retryCount: 3,
			maxRetries: 3,
			status:     CompletedStatus,
			expected:   false,
		},
		{
			name:       "should not move when retries below max even if failed",
			retryCount: 0,
			maxRetries: 3,
			status:     FailedStatus,
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &Task{
				RetryCount: tt.retryCount,
				MaxRetries: tt.maxRetries,
				Status:     tt.status,
			}

			result := task.ShouldMoveToDeadLetter()

			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTaskPriority_String(t *testing.T) {
	tests := []struct {
		name     string
		priority TaskPriority
		expected string
	}{
		{
			name:     "low priority",
			priority: LowPriority,
			expected: "low",
		},
		{
			name:     "medium priority",
			priority: MediumPriority,
			expected: "medium",
		},
		{
			name:     "high priority",
			priority: HighPriority,
			expected: "high",
		},
		{
			name:     "unknown priority value",
			priority: TaskPriority(99),
			expected: "unknown",
		},
		{
			name:     "negative priority value",
			priority: TaskPriority(-1),
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.priority.String()

			assert.Equal(t, tt.expected, result)
		})
	}
}
