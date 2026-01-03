// Package task defines the core task domain model used by the queue and persistence layers.
// It contains task metadata, status and priority definitions, and serialization helpers.
package task

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type (
	TaskStatus   string
	TaskPriority int
	Task         struct {
		ID            string         `json:"id"`
		Type          string         `json:"type"`
		Payload       map[string]any `json:"payload"`
		Priority      TaskPriority   `json:"priority"`
		Status        TaskStatus     `json:"status"`
		RetryCount    int            `json:"retry_count"`
		MaxRetries    int            `json:"max_retries"`
		CreatedAt     time.Time      `json:"created_at"`
		ScheduledAt   time.Time      `json:"scheduled_at"`
		StartedAt     *time.Time     `json:"started_at,omitempty"`
		CompletedAt   *time.Time     `json:"completed_at,omitempty"`
		Error         string         `json:"error,omitempty"`
		FailureReason string         `json:"failure_reason,omitempty"`
		MoveToDLQAt   time.Time      `json:"moved_to_dlq_at,omitempty"`
	}
)

const (
	StatusPending    TaskStatus = "pending"
	StatusRunning    TaskStatus = "running"
	StatusCompleted  TaskStatus = "completed"
	StatusFailed     TaskStatus = "failed"
	StatusDeadLetter TaskStatus = "dead_letter"
)

const (
	PriorityLow TaskPriority = iota
	PriorityMedium
	PriorityHigh
)

func NewTask(taskType string, payload map[string]any, priority TaskPriority) *Task {
	return &Task{
		ID:          uuid.New().String(),
		Type:        taskType,
		Payload:     payload,
		Priority:    priority,
		Status:      StatusPending,
		MaxRetries:  3,
		RetryCount:  0,
		CreatedAt:   time.Now(),
		ScheduledAt: time.Now(),
	}
}

func (t *Task) ToJSON() (string, error) {
	data, err := json.Marshal(t)
	if err != nil {
		return "", err
	}

	return string(data), err
}

func (t *Task) ShouldMoveToDeadLetter() bool {
	return t.RetryCount >= t.MaxRetries && t.Status == StatusFailed
}

func TaskFromJSON(data string) (*Task, error) {
	var task Task
	if err := json.Unmarshal([]byte(data), &task); err != nil {
		return nil, err
	}

	return &task, nil
}
