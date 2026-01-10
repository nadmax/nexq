package repository

import (
	"context"
	"fmt"
	"sync"

	"github.com/nadmax/nexq/internal/task"
)

type MockPostgresRepository struct {
	mu                    sync.Mutex
	GetTaskCalls          []string
	SaveTaskCalls         []SaveTaskCall
	UpdateTaskStatusCalls []UpdateTaskStatusCall
	CompleteTaskCalls     []CompleteTaskCall
	FailTaskCalls         []FailTaskCall
	MoveTaskToDLQCalls    []MoveTaskToDLQCall
	IncrementRetryCalls   []string
	LogExecutionCalls     []LogExecutionCall
	Tasks                 map[string]*task.Task
	ExecutionLog          []LogExecutionCall
	TaskStats             []TaskStats
	RecentTasks           []RecentTask
	GetTaskError          error
	SaveTaskError         error
	CompleteTaskError     error
	FailTaskError         error
	MoveTaskToDLQError    error
	IncrementRetryError   error
	LogExecutionError     error
	GetTaskStatsError     error
	GetRecentTasksError   error
	GetTaskHistoryError   error
	GetTasksByTypeError   error
}

type SaveTaskCall struct {
	Task *task.Task
}

type UpdateTaskStatusCall struct {
	TaskID   string
	Status   task.TaskStatus
	WorkerID string
}

type CompleteTaskCall struct {
	TaskID     string
	DurationMs int
}

type FailTaskCall struct {
	TaskID     string
	Reason     string
	DurationMs int
}

type MoveTaskToDLQCall struct {
	TaskID string
	Reason string
}

type LogExecutionCall struct {
	TaskID        string
	AttemptNumber int
	Status        string
	DurationMs    int
	ErrorMsg      string
	WorkerID      string
}

func NewMockPostgresRepository() *MockPostgresRepository {
	return &MockPostgresRepository{
		Tasks:        make(map[string]*task.Task),
		ExecutionLog: make([]LogExecutionCall, 0),
		TaskStats:    make([]TaskStats, 0),
		RecentTasks:  make([]RecentTask, 0),
	}
}

func (m *MockPostgresRepository) GetTask(ctx context.Context, taskID string) (*task.Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.GetTaskCalls = append(m.GetTaskCalls, taskID)

	if m.GetTaskError != nil {
		return nil, m.GetTaskError
	}

	t, exists := m.Tasks[taskID]
	if !exists {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}

	taskCopy := *t
	return &taskCopy, nil
}

func (m *MockPostgresRepository) SaveTask(ctx context.Context, t *task.Task) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.SaveTaskCalls = append(m.SaveTaskCalls, SaveTaskCall{Task: t})

	if m.SaveTaskError != nil {
		return m.SaveTaskError
	}

	taskCopy := *t
	m.Tasks[t.ID] = &taskCopy
	return nil
}

func (m *MockPostgresRepository) UpdateTaskStatus(ctx context.Context, taskID string, status task.TaskStatus, workerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.UpdateTaskStatusCalls = append(m.UpdateTaskStatusCalls, UpdateTaskStatusCall{
		TaskID:   taskID,
		Status:   status,
		WorkerID: workerID,
	})

	if t, exists := m.Tasks[taskID]; exists {
		t.Status = status
	}

	return nil
}

func (m *MockPostgresRepository) CompleteTask(ctx context.Context, taskID string, durationMs int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.CompleteTaskCalls = append(m.CompleteTaskCalls, CompleteTaskCall{
		TaskID:     taskID,
		DurationMs: durationMs,
	})

	if m.CompleteTaskError != nil {
		return m.CompleteTaskError
	}

	if t, exists := m.Tasks[taskID]; exists {
		t.Status = task.CompletedStatus
	}

	return nil
}

func (m *MockPostgresRepository) FailTask(ctx context.Context, taskID string, reason string, durationMs int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.FailTaskCalls = append(m.FailTaskCalls, FailTaskCall{
		TaskID:     taskID,
		Reason:     reason,
		DurationMs: durationMs,
	})

	if m.FailTaskError != nil {
		return m.FailTaskError
	}

	if t, exists := m.Tasks[taskID]; exists {
		t.Status = task.FailedStatus
		t.FailureReason = reason
	}

	return nil
}

func (m *MockPostgresRepository) MoveTaskToDLQ(ctx context.Context, taskID string, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.MoveTaskToDLQCalls = append(m.MoveTaskToDLQCalls, MoveTaskToDLQCall{
		TaskID: taskID,
		Reason: reason,
	})

	if m.MoveTaskToDLQError != nil {
		return m.MoveTaskToDLQError
	}

	if t, exists := m.Tasks[taskID]; exists {
		t.Status = task.DeadLetterStatus
		t.FailureReason = reason
	}

	return nil
}

func (m *MockPostgresRepository) IncrementRetryCount(ctx context.Context, taskID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.IncrementRetryCalls = append(m.IncrementRetryCalls, taskID)

	if m.IncrementRetryError != nil {
		return m.IncrementRetryError
	}

	if task, exists := m.Tasks[taskID]; exists {
		task.RetryCount++
	}

	return nil
}

func (m *MockPostgresRepository) LogExecution(ctx context.Context, taskID string, attemptNumber int, status string, durationMs int, errorMsg string, workerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	call := LogExecutionCall{
		TaskID:        taskID,
		AttemptNumber: attemptNumber,
		Status:        status,
		DurationMs:    durationMs,
		ErrorMsg:      errorMsg,
		WorkerID:      workerID,
	}

	m.LogExecutionCalls = append(m.LogExecutionCalls, call)
	m.ExecutionLog = append(m.ExecutionLog, call)

	if m.LogExecutionError != nil {
		return m.LogExecutionError
	}

	return nil
}

func (m *MockPostgresRepository) GetTaskStats(ctx context.Context, hours int) ([]TaskStats, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.GetTaskStatsError != nil {
		return nil, m.GetTaskStatsError
	}

	return m.TaskStats, nil
}

func (m *MockPostgresRepository) GetRecentTasks(ctx context.Context, limit int) ([]RecentTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.GetRecentTasksError != nil {
		return nil, m.GetRecentTasksError
	}

	if len(m.RecentTasks) > limit {
		return m.RecentTasks[:limit], nil
	}

	return m.RecentTasks, nil
}

func (m *MockPostgresRepository) GetTasksByType(ctx context.Context, taskType string, limit int) ([]RecentTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.GetTasksByTypeError != nil {
		return nil, m.GetTasksByTypeError
	}

	var filtered []RecentTask
	for _, task := range m.RecentTasks {
		if task.Type == taskType {
			filtered = append(filtered, task)
			if len(filtered) >= limit {
				break
			}
		}
	}

	return filtered, nil
}

func (m *MockPostgresRepository) GetTaskHistory(ctx context.Context, taskID string) ([]map[string]any, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.GetTaskHistoryError != nil {
		return nil, m.GetTaskHistoryError
	}

	var history []map[string]any
	for _, log := range m.ExecutionLog {
		if log.TaskID == taskID {
			entry := map[string]any{
				"task_id":        log.TaskID,
				"attempt_number": log.AttemptNumber,
				"status":         log.Status,
				"duration_ms":    log.DurationMs,
				"error_message":  log.ErrorMsg,
				"worker_id":      log.WorkerID,
			}
			history = append(history, entry)
		}
	}

	return history, nil
}

func (m *MockPostgresRepository) Close() error {
	return nil
}

func (m *MockPostgresRepository) GetSaveTaskCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return len(m.SaveTaskCalls)
}

func (m *MockPostgresRepository) GetCompleteTaskCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return len(m.CompleteTaskCalls)
}

func (m *MockPostgresRepository) GetFailTaskCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return len(m.FailTaskCalls)
}

func (m *MockPostgresRepository) GetLogExecutionCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return len(m.LogExecutionCalls)
}

func (m *MockPostgresRepository) WasTaskSaved(taskID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, exists := m.Tasks[taskID]
	return exists
}

func (m *MockPostgresRepository) GetTaskStatus(taskID string) (task.TaskStatus, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if task, exists := m.Tasks[taskID]; exists {
		return task.Status, true
	}

	return "", false
}

func (m *MockPostgresRepository) GetExecutionLogForTask(taskID string) []LogExecutionCall {
	m.mu.Lock()
	defer m.mu.Unlock()

	var logs []LogExecutionCall
	for _, exec := range m.ExecutionLog {
		if exec.TaskID == taskID {
			logs = append(logs, exec)
		}
	}

	return logs
}

func (m *MockPostgresRepository) GetIncrementRetryCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return len(m.IncrementRetryCalls)
}

func (m *MockPostgresRepository) GetMoveToDLQCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return len(m.MoveTaskToDLQCalls)
}

func (m *MockPostgresRepository) GetUpdateTaskStatusCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return len(m.UpdateTaskStatusCalls)
}

func (m *MockPostgresRepository) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.SaveTaskCalls = nil
	m.UpdateTaskStatusCalls = nil
	m.CompleteTaskCalls = nil
	m.FailTaskCalls = nil
	m.MoveTaskToDLQCalls = nil
	m.IncrementRetryCalls = nil
	m.LogExecutionCalls = nil
	m.Tasks = make(map[string]*task.Task)
	m.ExecutionLog = nil
}
