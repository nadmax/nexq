package queue

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type (
	TaskStatus   string
	TaskPriority int
)

type Task struct {
	ID          string         `json:"id"`
	Type        string         `json:"type"`
	Payload     map[string]any `json:"payload"`
	Priority    TaskPriority   `json:"priority"`
	Status      TaskStatus     `json:"status"`
	MaxRetries  int            `json:"max_retries"`
	Retries     int            `json:"retries"`
	CreatedAt   time.Time      `json:"created_at"`
	ScheduledAt time.Time      `json:"scheduled_at"`
	StartedAt   *time.Time     `json:"started_at,omitempty"`
	CompletedAt *time.Time     `json:"completed_at,omitempty"`
	Error       string         `json:"error,omitempty"`
}

const (
	StatusPending   TaskStatus = "pending"
	StatusRunning   TaskStatus = "running"
	StatusCompleted TaskStatus = "completed"
	StatusFailed    TaskStatus = "failed"
)

const (
	PriorityLow    TaskPriority = 0
	PriorityNormal TaskPriority = 5
	PriorityHigh   TaskPriority = 10
)

func NewTask(taskType string, payload map[string]any) *Task {
	return &Task{
		ID:          uuid.New().String(),
		Type:        taskType,
		Payload:     payload,
		Priority:    PriorityNormal,
		Status:      StatusPending,
		MaxRetries:  3,
		Retries:     0,
		CreatedAt:   time.Now(),
		ScheduledAt: time.Now(),
	}
}

func (t *Task) ToJSON() (string, error) {
	data, err := json.Marshal(t)
	return string(data), err
}

func TaskFromJSON(data string) (*Task, error) {
	var task Task
	err := json.Unmarshal([]byte(data), &task)
	return &task, err
}
