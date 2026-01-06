package repository

import (
	"context"

	"github.com/nadmax/nexq/internal/task"
)

type TaskRepository interface {
	GetTask(ctx context.Context, taskID string) (*task.Task, error)
	SaveTask(ctx context.Context, t *task.Task) error
	UpdateTaskStatus(ctx context.Context, taskID string, status task.TaskStatus, workerID string) error
	CompleteTask(ctx context.Context, taskID string, durationMs int) error
	FailTask(ctx context.Context, taskID string, reason string, durationMs int) error
	MoveTaskToDLQ(ctx context.Context, taskID string, reason string) error
	IncrementRetryCount(ctx context.Context, taskID string) error
	LogExecution(ctx context.Context, taskID string, attemptNumber int, status string, durationMs int, msgErr string, workerID string) error
	GetTaskStats(ctx context.Context, hours int) ([]TaskStats, error)
	GetRecentTasks(ctx context.Context, limit int) ([]RecentTask, error)
	GetTasksByType(ctx context.Context, taskType string, limit int) ([]RecentTask, error)
	GetTaskHistory(ctx context.Context, taskID string) ([]map[string]any, error)
	Close() error
}
