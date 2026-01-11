package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/nadmax/nexq/internal/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock, *PostgresTaskRepository) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	repo := &PostgresTaskRepository{db: db}
	return db, mock, repo
}

func TestNewPostgresTaskRepository(t *testing.T) {
	t.Run("successful connection", func(t *testing.T) {
		t.Skip("Integration test - requires real database")
	})

	t.Run("connection failure", func(t *testing.T) {
		_, err := NewPostgresTaskRepository("invalid connection string")
		assert.Error(t, err)
	})
}

func TestGetTask(t *testing.T) {
	db, mock, repo := setupMockDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	taskID := "task-123"
	now := time.Now()
	startedAt := now.Add(1 * time.Minute)
	completedAt := now.Add(5 * time.Minute)

	t.Run("successful retrieval", func(t *testing.T) {
		payload := map[string]any{"key": "value"}
		payloadBytes, _ := json.Marshal(payload)

		rows := sqlmock.NewRows([]string{
			"task_id", "type", "payload", "priority", "status",
			"retry_count", "failure_reason", "created_at",
			"scheduled_at", "started_at", "completed_at",
			"duration_ms", "worker_id", "moved_to_dlq_at",
		}).AddRow(
			taskID, "email", payloadBytes, 5, "completed",
			0, nil, now,
			now, startedAt, completedAt,
			5000, "worker-1", nil,
		)

		mock.ExpectQuery("SELECT.*FROM task_history WHERE task_id").
			WithArgs(taskID).
			WillReturnRows(rows)

		result, err := repo.GetTask(ctx, taskID)
		require.NoError(t, err)
		assert.Equal(t, taskID, result.ID)
		assert.Equal(t, "email", result.Type)
		assert.Equal(t, task.TaskStatus("completed"), result.Status)
		assert.NotNil(t, result.StartedAt)
		assert.NotNil(t, result.CompletedAt)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("task not found", func(t *testing.T) {
		mock.ExpectQuery("SELECT.*FROM task_history WHERE task_id").
			WithArgs("nonexistent").
			WillReturnError(sql.ErrNoRows)

		_, err := repo.GetTask(ctx, "nonexistent")
		assert.Error(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("invalid payload JSON", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{
			"task_id", "type", "payload", "priority", "status",
			"retry_count", "failure_reason", "created_at",
			"scheduled_at", "started_at", "completed_at",
			"duration_ms", "worker_id", "moved_to_dlq_at",
		}).AddRow(
			taskID, "email", []byte("invalid json"), 5, "completed",
			0, nil, now,
			now, nil, nil,
			nil, nil, nil,
		)

		mock.ExpectQuery("SELECT.*FROM task_history WHERE task_id").
			WithArgs(taskID).
			WillReturnRows(rows)

		_, err := repo.GetTask(ctx, taskID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to unmarshal payload")
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestSaveTask(t *testing.T) {
	db, mock, repo := setupMockDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	now := time.Now()

	t.Run("successful save with scheduled time", func(t *testing.T) {
		tsk := &task.Task{
			ID:          "task-123",
			Type:        "email",
			Payload:     map[string]any{"to": "test@example.com"},
			Priority:    5,
			Status:      task.PendingStatus,
			RetryCount:  0,
			CreatedAt:   now,
			ScheduledAt: now.Add(1 * time.Hour),
		}

		mock.ExpectExec("INSERT INTO task_history").
			WithArgs(
				tsk.ID,
				tsk.Type,
				sqlmock.AnyArg(),
				tsk.Priority,
				tsk.Status,
				tsk.RetryCount,
				tsk.FailureReason,
				tsk.CreatedAt,
				tsk.ScheduledAt,
			).
			WillReturnResult(sqlmock.NewResult(1, 1))

		err := repo.SaveTask(ctx, tsk)
		assert.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("successful save with zero scheduled time", func(t *testing.T) {
		tsk := &task.Task{
			ID:         "task-456",
			Type:       "webhook",
			Payload:    map[string]any{"url": "https://example.com"},
			Priority:   3,
			Status:     task.PendingStatus,
			RetryCount: 0,
			CreatedAt:  now,
		}

		mock.ExpectExec("INSERT INTO task_history").
			WithArgs(
				tsk.ID,
				tsk.Type,
				sqlmock.AnyArg(),
				tsk.Priority,
				tsk.Status,
				tsk.RetryCount,
				tsk.FailureReason,
				tsk.CreatedAt,
				nil,
			).
			WillReturnResult(sqlmock.NewResult(1, 1))

		err := repo.SaveTask(ctx, tsk)
		assert.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("conflict handling - update existing", func(t *testing.T) {
		tsk := &task.Task{
			ID:            "task-789",
			Type:          "email",
			Payload:       map[string]any{"to": "update@example.com"},
			Priority:      5,
			Status:        task.RunningStatus,
			RetryCount:    2,
			FailureReason: "timeout",
			CreatedAt:     now,
			ScheduledAt:   now,
		}

		mock.ExpectExec("INSERT INTO task_history").
			WithArgs(
				tsk.ID,
				tsk.Type,
				sqlmock.AnyArg(),
				tsk.Priority,
				tsk.Status,
				tsk.RetryCount,
				tsk.FailureReason,
				tsk.CreatedAt,
				tsk.ScheduledAt,
			).
			WillReturnResult(sqlmock.NewResult(0, 1))

		err := repo.SaveTask(ctx, tsk)
		assert.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestUpdateTaskStatus(t *testing.T) {
	db, mock, repo := setupMockDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()

	t.Run("update to running status", func(t *testing.T) {
		mock.ExpectExec("UPDATE task_history SET status").
			WithArgs("running", "worker-1", "task-123", "running").
			WillReturnResult(sqlmock.NewResult(0, 1))

		err := repo.UpdateTaskStatus(ctx, "task-123", task.RunningStatus, "worker-1")
		assert.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("update to pending status", func(t *testing.T) {
		mock.ExpectExec("UPDATE task_history SET status").
			WithArgs("pending", "worker-2", "task-456", "pending").
			WillReturnResult(sqlmock.NewResult(0, 1))

		err := repo.UpdateTaskStatus(ctx, "task-456", task.PendingStatus, "worker-2")
		assert.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestCompleteTask(t *testing.T) {
	db, mock, repo := setupMockDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()

	t.Run("successful completion", func(t *testing.T) {
		mock.ExpectExec("UPDATE task_history SET status = 'completed'").
			WithArgs(5000, "task-123").
			WillReturnResult(sqlmock.NewResult(0, 1))

		err := repo.CompleteTask(ctx, "task-123", 5000)
		assert.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestFailTask(t *testing.T) {
	db, mock, repo := setupMockDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()

	t.Run("task failure with reason", func(t *testing.T) {
		reason := "connection timeout"
		mock.ExpectExec("UPDATE task_history SET status = 'failed'").
			WithArgs(reason, 3000, "task-123").
			WillReturnResult(sqlmock.NewResult(0, 1))

		err := repo.FailTask(ctx, "task-123", reason, 3000)
		assert.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestMoveTaskToDLQ(t *testing.T) {
	db, mock, repo := setupMockDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()

	t.Run("move to dead letter queue", func(t *testing.T) {
		reason := "max retries exceeded"
		mock.ExpectExec("UPDATE task_history SET status = 'dead_letter'").
			WithArgs(reason, "task-123").
			WillReturnResult(sqlmock.NewResult(0, 1))

		err := repo.MoveTaskToDLQ(ctx, "task-123", reason)
		assert.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestIncrementRetryCount(t *testing.T) {
	db, mock, repo := setupMockDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()

	t.Run("increment retry count", func(t *testing.T) {
		mock.ExpectExec("UPDATE task_history SET retry_count = retry_count \\+ 1").
			WithArgs("task-123").
			WillReturnResult(sqlmock.NewResult(0, 1))

		err := repo.IncrementRetryCount(ctx, "task-123")
		assert.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestLogExecution(t *testing.T) {
	db, mock, repo := setupMockDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()

	t.Run("log successful execution", func(t *testing.T) {
		mock.ExpectExec("INSERT INTO task_execution_log").
			WithArgs("task-123", 1, "completed", 2500, nil, "worker-1").
			WillReturnResult(sqlmock.NewResult(1, 1))

		err := repo.LogExecution(ctx, "task-123", 1, "completed", 2500, "", "worker-1")
		assert.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("log failed execution with error", func(t *testing.T) {
		errMsg := "database connection failed"
		mock.ExpectExec("INSERT INTO task_execution_log").
			WithArgs("task-456", 2, "failed", nil, errMsg, "worker-2").
			WillReturnResult(sqlmock.NewResult(1, 1))

		err := repo.LogExecution(ctx, "task-456", 2, "failed", 0, errMsg, "worker-2")
		assert.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestGetTaskStats(t *testing.T) {
	db, mock, repo := setupMockDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()

	t.Run("get stats for last 24 hours", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{
			"type", "status", "count", "avg_duration_ms",
			"max_duration_ms", "min_duration_ms", "avg_retries",
		}).
			AddRow("email", "completed", 100, 2500.5, 5000, 1000, 0.5).
			AddRow("email", "failed", 10, 3000.0, 4000, 2000, 2.0).
			AddRow("webhook", "completed", 50, 1500.0, 2000, 1000, 0.1)

		mock.ExpectQuery("SELECT.*FROM task_history WHERE created_at").
			WithArgs(24).
			WillReturnRows(rows)

		stats, err := repo.GetTaskStats(ctx, 24)
		require.NoError(t, err)
		assert.Len(t, stats, 3)
		assert.Equal(t, "email", stats[0].Type)
		assert.Equal(t, "completed", stats[0].Status)
		assert.Equal(t, 100, stats[0].Count)
		assert.Equal(t, 2500.5, stats[0].AvgDurationMs)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("no stats available", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{
			"type", "status", "count", "avg_duration_ms",
			"max_duration_ms", "min_duration_ms", "avg_retries",
		})

		mock.ExpectQuery("SELECT.*FROM task_history WHERE created_at").
			WithArgs(1).
			WillReturnRows(rows)

		stats, err := repo.GetTaskStats(ctx, 1)
		require.NoError(t, err)
		assert.Empty(t, stats)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestGetRecentTasks(t *testing.T) {
	db, mock, repo := setupMockDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	now := time.Now()

	t.Run("get recent tasks", func(t *testing.T) {
		completedAt := now.Add(5 * time.Minute)
		rows := sqlmock.NewRows([]string{
			"task_id", "type", "status", "created_at", "completed_at",
			"duration_ms", "retry_count", "failure_reason",
		}).
			AddRow("task-1", "email", "completed", now, completedAt, 5000, 0, "").
			AddRow("task-2", "webhook", "failed", now, completedAt, 3000, 2, "timeout")

		mock.ExpectQuery("SELECT.*FROM task_history ORDER BY created_at DESC").
			WithArgs(10).
			WillReturnRows(rows)

		tasks, err := repo.GetRecentTasks(ctx, 10)
		require.NoError(t, err)
		assert.Len(t, tasks, 2)
		assert.Equal(t, "task-1", tasks[0].TaskID)
		assert.Equal(t, "email", tasks[0].Type)
		assert.Equal(t, "task-2", tasks[1].TaskID)
		assert.Equal(t, "timeout", tasks[1].FailureReason)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestGetTasksByType(t *testing.T) {
	db, mock, repo := setupMockDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	now := time.Now()

	t.Run("get tasks by type", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{
			"task_id", "type", "status", "created_at", "completed_at",
			"duration_ms", "retry_count", "failure_reason",
		}).
			AddRow("task-1", "email", "completed", now, now, 5000, 0, "").
			AddRow("task-2", "email", "failed", now, now, 3000, 1, "smtp error")

		mock.ExpectQuery("SELECT.*FROM task_history WHERE type").
			WithArgs("email", 50).
			WillReturnRows(rows)

		tasks, err := repo.GetTasksByType(ctx, "email", 50)
		require.NoError(t, err)
		assert.Len(t, tasks, 2)
		assert.Equal(t, "email", tasks[0].Type)
		assert.Equal(t, "email", tasks[1].Type)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestGetTaskHistory(t *testing.T) {
	db, mock, repo := setupMockDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	now := time.Now()

	t.Run("get task execution history", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{
			"attempt_number", "status", "started_at", "completed_at",
			"duration_ms", "error_message", "worker_id",
		}).
			AddRow(1, "completed", now, now.Add(2*time.Second), 2000, nil, "worker-1").
			AddRow(2, "failed", now, now.Add(3*time.Second), 3000, "timeout", "worker-2")

		mock.ExpectQuery("SELECT.*FROM task_execution_log WHERE task_id").
			WithArgs("task-123").
			WillReturnRows(rows)

		history, err := repo.GetTaskHistory(ctx, "task-123")
		require.NoError(t, err)
		assert.Len(t, history, 2)
		assert.Equal(t, 1, history[0]["attempt_number"])
		assert.Equal(t, "completed", history[0]["status"])
		assert.Equal(t, 2, history[1]["attempt_number"])
		assert.Equal(t, "timeout", history[1]["error_message"])
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("task with no execution history", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{
			"attempt_number", "status", "started_at", "completed_at",
			"duration_ms", "error_message", "worker_id",
		})

		mock.ExpectQuery("SELECT.*FROM task_execution_log WHERE task_id").
			WithArgs("task-999").
			WillReturnRows(rows)

		history, err := repo.GetTaskHistory(ctx, "task-999")
		require.NoError(t, err)
		assert.Empty(t, history)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestDBAndClose(t *testing.T) {
	t.Run("DB returns database instance", func(t *testing.T) {
		db, _, repo := setupMockDB(t)
		defer func() { _ = db.Close() }()

		assert.Equal(t, db, repo.DB())
	})

	t.Run("Close closes database connection", func(t *testing.T) {
		_, mock, repo := setupMockDB(t)

		mock.ExpectClose()

		err := repo.Close()
		assert.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}
