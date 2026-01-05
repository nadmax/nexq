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
		MoveToDLQAt   *time.Time     `json:"moved_to_dlq_at,omitempty"`
	}
)

const (
	PendingStatus    TaskStatus = "pending"
	RunningStatus    TaskStatus = "running"
	CompletedStatus  TaskStatus = "completed"
	FailedStatus     TaskStatus = "failed"
	DeadLetterStatus TaskStatus = "dead_letter"
)

const (
	LowPriority TaskPriority = iota
	MediumPriority
	HighPriority
)

func NewTask(taskType string, payload map[string]any, priority TaskPriority) *Task {
	return &Task{
		ID:          uuid.New().String(),
		Type:        taskType,
		Payload:     payload,
		Priority:    priority,
		Status:      PendingStatus,
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
	return t.RetryCount >= t.MaxRetries && t.Status == FailedStatus
}

func TaskFromJSON(data string) (*Task, error) {
	var t Task

	if err := json.Unmarshal([]byte(data), &t); err != nil {
		return nil, err
	}

	return &t, nil
}

func (p TaskPriority) String() string {
	switch p {
	case LowPriority:
		return "low"
	case MediumPriority:
		return "medium"
	case HighPriority:
		return "high"
	default:
		return "unknown"
	}
}
