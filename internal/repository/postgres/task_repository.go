// Package postgres provides PostgreSQL-backed implementations of repository interfaces.
package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	_ "github.com/lib/pq"
	"github.com/nadmax/nexq/internal/repository/models"
	"github.com/nadmax/nexq/internal/task"
)

type PostgresTaskRepository struct {
	db *sql.DB
}

func NewPostgresTaskRepository(connectionString string) (*PostgresTaskRepository, error) {
	db, err := sql.Open("postgres", connectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to PostgreSQL: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping PostgreSQL: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	return &PostgresTaskRepository{db: db}, nil
}

func (r *PostgresTaskRepository) GetTask(ctx context.Context, taskID string) (*task.Task, error) {
	query := `
		SELECT 
			task_id, type, payload, priority, status, 
			retry_count, failure_reason, created_at, 
			scheduled_at, started_at, completed_at,
			duration_ms, worker_id, moved_to_dlq_at
		FROM task_history
		WHERE task_id = $1
	`

	var t task.Task
	var payload []byte
	var scheduledAt, startedAt, completedAt, movedToDLQAt sql.NullTime
	var durationMs sql.NullInt64
	var workerID, failureReason sql.NullString

	err := r.db.QueryRowContext(ctx, query, taskID).Scan(
		&t.ID,
		&t.Type,
		&payload,
		&t.Priority,
		&t.Status,
		&t.RetryCount,
		&failureReason,
		&t.CreatedAt,
		&scheduledAt,
		&startedAt,
		&completedAt,
		&durationMs,
		&workerID,
		&movedToDLQAt,
	)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(payload, &t.Payload); err != nil {
		return nil, fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	if scheduledAt.Valid {
		t.ScheduledAt = scheduledAt.Time
	}
	if startedAt.Valid {
		t.StartedAt = &startedAt.Time
	}
	if completedAt.Valid {
		t.CompletedAt = &completedAt.Time
	}
	if movedToDLQAt.Valid {
		t.MoveToDLQAt = &movedToDLQAt.Time
	}
	if failureReason.Valid {
		t.FailureReason = failureReason.String
	}

	return &t, nil
}

func (r *PostgresTaskRepository) SaveTask(ctx context.Context, t *task.Task) error {
	payload, err := json.Marshal(t.Payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	query := `
		INSERT INTO task_history (
			task_id, type, payload, priority, status, 
			retry_count, failure_reason, created_at, scheduled_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (task_id) DO UPDATE SET
			status = EXCLUDED.status,
			retry_count = EXCLUDED.retry_count,
			failure_reason = EXCLUDED.failure_reason,
			scheduled_at = EXCLUDED.scheduled_at
	`

	var scheduledAt any
	if t.ScheduledAt.IsZero() {
		scheduledAt = nil
	} else {
		scheduledAt = t.ScheduledAt
	}

	_, err = r.db.ExecContext(
		ctx,
		query,
		t.ID,
		t.Type,
		payload,
		t.Priority,
		t.Status,
		t.RetryCount,
		t.FailureReason,
		t.CreatedAt,
		scheduledAt,
	)

	return err
}

func (r *PostgresTaskRepository) UpdateTaskStatus(ctx context.Context, taskID string, status task.TaskStatus, workerID string) error {
	statusStr := string(status)
	query := `
		UPDATE task_history 
		SET status = $1,
		    started_at = CASE WHEN $4::text = 'running' THEN NOW() ELSE started_at END,
		    worker_id = $2
		WHERE task_id = $3
	`

	_, err := r.db.ExecContext(ctx, query, statusStr, workerID, taskID, statusStr)
	return err
}

func (r *PostgresTaskRepository) CompleteTask(ctx context.Context, taskID string, durationMs int) error {
	query := `
		UPDATE task_history 
		SET status = 'completed',
		    completed_at = NOW(),
		    duration_ms = $1
		WHERE task_id = $2
	`
	_, err := r.db.ExecContext(ctx, query, durationMs, taskID)

	return err
}

func (r *PostgresTaskRepository) FailTask(ctx context.Context, taskID string, reason string, durationMs int) error {
	query := `
		UPDATE task_history 
		SET status = 'failed',
		    completed_at = NOW(),
		    failure_reason = $1,
		    duration_ms = $2,
		    last_error = $1
		WHERE task_id = $3
	`
	_, err := r.db.ExecContext(ctx, query, reason, durationMs, taskID)

	return err
}

func (r *PostgresTaskRepository) MoveTaskToDLQ(ctx context.Context, taskID string, reason string) error {
	query := `
		UPDATE task_history 
		SET status = 'dead_letter',
		    failure_reason = $1,
		    moved_to_dlq_at = NOW()
		WHERE task_id = $2
	`
	_, err := r.db.ExecContext(ctx, query, reason, taskID)

	return err
}

func (r *PostgresTaskRepository) IncrementRetryCount(ctx context.Context, taskID string) error {
	query := `
		UPDATE task_history 
		SET retry_count = retry_count + 1
		WHERE task_id = $1
	`
	_, err := r.db.ExecContext(ctx, query, taskID)

	return err
}

func (r *PostgresTaskRepository) LogExecution(ctx context.Context, taskID string, attemptNumber int, status string, durationMs int, msgErr string, workerID string) error {
	query := `
		INSERT INTO task_execution_log (
			task_id, attempt_number, status, completed_at, 
			duration_ms, error_message, worker_id
		) VALUES ($1, $2, $3, NOW(), $4, $5, $6)
	`

	var durationMsVal any
	if durationMs == 0 {
		durationMsVal = nil
	} else {
		durationMsVal = durationMs
	}

	var msgErrVal any
	if msgErr == "" {
		msgErrVal = nil
	} else {
		msgErrVal = msgErr
	}

	_, err := r.db.ExecContext(
		ctx,
		query,
		taskID,
		attemptNumber,
		status,
		durationMsVal,
		msgErrVal,
		workerID,
	)

	return err
}

func (r *PostgresTaskRepository) GetTaskStats(ctx context.Context, hours int) ([]models.TaskStats, error) {
	query := `
		SELECT 
			type, status, COUNT(*) as count,
			COALESCE(AVG(duration_ms), 0) as avg_duration_ms,
			COALESCE(MAX(duration_ms), 0) as max_duration_ms,
			COALESCE(MIN(duration_ms), 0) as min_duration_ms,
			COALESCE(AVG(retry_count), 0) as avg_retries
		FROM task_history
		WHERE created_at > NOW() - INTERVAL '1 hour' * $1
		GROUP BY type, status
		ORDER BY type, status
	`
	rows, err := r.db.QueryContext(ctx, query, hours)
	if err != nil {
		return nil, err
	}

	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("failed to close rows: %v", err)
		}
	}()

	var stats []models.TaskStats
	for rows.Next() {
		var s models.TaskStats
		if err := rows.Scan(
			&s.Type,
			&s.Status,
			&s.Count,
			&s.AvgDurationMs,
			&s.MaxDurationMs,
			&s.MinDurationMs,
			&s.AvgRetries,
		); err != nil {
			return nil, err
		}

		stats = append(stats, s)
	}

	return stats, rows.Err()
}

func (r *PostgresTaskRepository) GetRecentTasks(ctx context.Context, limit int) ([]models.RecentTask, error) {
	query := `
		SELECT 
			task_id, type, status, created_at, completed_at,
			duration_ms, retry_count, COALESCE(failure_reason, '')
		FROM task_history
		ORDER BY created_at DESC
		LIMIT $1
	`
	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, err
	}

	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("failed to close rows: %v", err)
		}
	}()

	var tasks []models.RecentTask
	for rows.Next() {
		var t models.RecentTask
		if err := rows.Scan(
			&t.TaskID,
			&t.Type,
			&t.Status,
			&t.CreatedAt,
			&t.CompletedAt,
			&t.DurationMs,
			&t.RetryCount,
			&t.FailureReason,
		); err != nil {
			return nil, err
		}

		tasks = append(tasks, t)
	}

	return tasks, rows.Err()
}

func (r *PostgresTaskRepository) GetTasksByType(ctx context.Context, taskType string, limit int) ([]models.RecentTask, error) {
	query := `
		SELECT 
			task_id, type, status, created_at, completed_at,
			duration_ms, retry_count, COALESCE(failure_reason, '')
		FROM task_history
		WHERE type = $1
		ORDER BY created_at DESC
		LIMIT $2
	`
	rows, err := r.db.QueryContext(ctx, query, taskType, limit)
	if err != nil {
		return nil, err
	}

	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("failed to close rows: %v", err)
		}
	}()

	var tasks []models.RecentTask
	for rows.Next() {
		var t models.RecentTask
		if err := rows.Scan(
			&t.TaskID,
			&t.Type,
			&t.Status,
			&t.CreatedAt,
			&t.CompletedAt,
			&t.DurationMs,
			&t.RetryCount,
			&t.FailureReason,
		); err != nil {
			return nil, err
		}

		tasks = append(tasks, t)
	}

	return tasks, rows.Err()
}

func (r *PostgresTaskRepository) GetTaskHistory(ctx context.Context, taskID string) ([]map[string]any, error) {
	query := `
		SELECT 
			attempt_number, status, started_at, completed_at,
			duration_ms, error_message, worker_id
		FROM task_execution_log
		WHERE task_id = $1
		ORDER BY started_at ASC
	`
	rows, err := r.db.QueryContext(ctx, query, taskID)
	if err != nil {
		return nil, err
	}

	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("failed to close rows: %v", err)
		}
	}()

	var history []map[string]any
	for rows.Next() {
		var attemptNum int
		var status, workerID string
		var startedAt, completedAt sql.NullTime
		var durationMs sql.NullInt64
		var msgErr sql.NullString

		if err := rows.Scan(
			&attemptNum,
			&status,
			&startedAt,
			&completedAt,
			&durationMs,
			&msgErr,
			&workerID,
		); err != nil {
			return nil, err
		}

		entry := map[string]any{
			"attempt_number": attemptNum,
			"status":         status,
			"worker_id":      workerID,
		}

		if startedAt.Valid {
			entry["started_at"] = startedAt.Time
		}
		if completedAt.Valid {
			entry["completed_at"] = completedAt.Time
		}
		if durationMs.Valid {
			entry["duration_ms"] = durationMs.Int64
		}
		if msgErr.Valid {
			entry["error_message"] = msgErr.String
		}

		history = append(history, entry)
	}

	return history, rows.Err()
}

func (r *PostgresTaskRepository) DB() *sql.DB {
	return r.db
}

func (r *PostgresTaskRepository) Close() error {
	return r.db.Close()
}
