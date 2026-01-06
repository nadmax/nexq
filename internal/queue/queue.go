// Package queue implements a Pogocache-backed distributed job queue with support for task scheduling and retries.
package queue

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/nadmax/nexq/internal/metrics"
	"github.com/nadmax/nexq/internal/repository"
	"github.com/nadmax/nexq/internal/task"
	"github.com/redis/go-redis/v9"
)

type Queue struct {
	client *redis.Client
	repo   repository.TaskRepository
	ctx    context.Context
}

func NewQueue(redisAddr string, repo repository.TaskRepository) (*Queue, error) {
	client := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})

	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &Queue{
		client: client,
		repo:   repo,
		ctx:    ctx,
	}, nil
}

func (q *Queue) Enqueue(t *task.Task) error {
	if q.repo != nil {
		t.Status = task.PendingStatus
		if err := q.repo.SaveTask(q.ctx, t); err != nil {
			log.Printf("Warning: failed to save task in database: %v", err)
		}
	}

	data, err := t.ToJSON()
	if err != nil {
		return err
	}

	seq, err := q.client.Incr(q.ctx, "queue:tail").Result()
	if err != nil {
		return err
	}

	if err := q.client.Set(
		q.ctx,
		fmt.Sprintf("queue:item:%d", seq),
		t.ID,
		0,
	).Err(); err != nil {
		return err
	}

	if err := q.client.Set(
		q.ctx,
		"task:"+t.ID,
		data,
		0,
	).Err(); err != nil {
		return err
	}

	metrics.RecordTaskEnqueued(t.Type, t.Priority)

	return nil
}

func (q *Queue) Dequeue() (*task.Task, error) {
	headStr, _ := q.client.Get(q.ctx, "queue:head").Result()
	tailStr, _ := q.client.Get(q.ctx, "queue:tail").Result()
	head := int64(0)
	tail := int64(0)
	if headStr != "" {
		head, _ = strconv.ParseInt(headStr, 10, 64)
	}
	if tailStr != "" {
		tail, _ = strconv.ParseInt(tailStr, 10, 64)
	}
	if head >= tail {
		return nil, nil
	}

	newHead, err := q.client.Incr(q.ctx, "queue:head").Result()
	if err != nil {
		return nil, err
	}

	itemKey := fmt.Sprintf("queue:item:%d", newHead)
	taskID, err := q.client.Get(q.ctx, itemKey).Result()
	if err != nil {
		return nil, nil
	}

	data, err := q.client.Get(q.ctx, "task:"+taskID).Result()
	if err != nil {
		return nil, err
	}

	t, err := task.TaskFromJSON(data)
	if err != nil {
		return nil, err
	}

	waitTime := time.Since(t.CreatedAt)
	metrics.RecordTaskWaitTime(t.Type, t.Priority, waitTime)
	if q.repo != nil {
		t.Status = task.RunningStatus
		if err := q.repo.UpdateTaskStatus(q.ctx, t.ID, task.RunningStatus, ""); err != nil {
			log.Printf("Warning: failed to update task status: %v", err)
		}
	}

	q.client.Del(q.ctx, itemKey)
	q.client.Del(q.ctx, "task:"+taskID)

	return t, nil
}

func (q *Queue) CompleteTask(t *task.Task, durationMs int) error {
	duration := time.Duration(durationMs) * time.Millisecond
	metrics.RecordTaskCompleted(t.Type, duration)

	if q.repo != nil {
		return q.repo.CompleteTask(q.ctx, t.ID, durationMs)
	}

	return nil
}

func (q *Queue) FailTask(t *task.Task, reason string, durationMs int) error {
	duration := time.Duration(durationMs) * time.Millisecond
	metrics.RecordTaskFailed(t.Type, duration)

	if q.repo != nil {
		return q.repo.FailTask(q.ctx, t.ID, reason, durationMs)
	}

	return nil
}

func (q *Queue) UpdateTask(task *task.Task) error {
	data, err := task.ToJSON()
	if err != nil {
		return err
	}

	if q.repo != nil {
		if err := q.repo.SaveTask(q.ctx, task); err != nil {
			log.Printf("Warning: failed to update task in database: %v", err)
		}
	}

	return q.client.Set(
		q.ctx,
		"task:"+task.ID,
		data,
		0,
	).Err()
}

func (q *Queue) GetTask(taskID string) (*task.Task, error) {
	data, err := q.client.Get(
		q.ctx,
		"task:"+taskID,
	).Result()
	if err != nil {
		return nil, err
	}

	return task.TaskFromJSON(data)
}

func (q *Queue) GetAllTasks() ([]*task.Task, error) {
	var tasks []*task.Task

	iter := q.client.Scan(q.ctx, 0, "task:*", 100).Iterator()
	for iter.Next(q.ctx) {
		key := iter.Val()

		data, err := q.client.Get(q.ctx, key).Result()
		if err != nil {
			continue
		}

		task, err := task.TaskFromJSON(data)
		if err != nil {
			continue
		}

		tasks = append(tasks, task)
	}

	if err := iter.Err(); err != nil {
		return nil, err
	}

	return tasks, nil
}

func (q *Queue) MoveToDeadLetter(t *task.Task, reason string) error {
	t.FailureReason = reason
	now := time.Now()
	t.MoveToDLQAt = &now
	t.Status = task.DeadLetterStatus

	if q.repo != nil {
		if err := q.repo.MoveTaskToDLQ(q.ctx, t.ID, reason); err != nil {
			log.Printf("Warning: failed to move task to DLQ in database: %v", err)
		}
	}

	data, err := t.ToJSON()
	if err != nil {
		return err
	}

	seq, err := q.client.Incr(q.ctx, "dlq:tail").Result()
	if err != nil {
		return err
	}

	if err := q.client.Set(
		q.ctx,
		fmt.Sprintf("dlq:item:%d", seq),
		t.ID,
		0,
	).Err(); err != nil {
		return err
	}

	if err := q.client.Set(
		q.ctx,
		"dlq:task:"+t.ID,
		data,
		0,
	).Err(); err != nil {
		return err
	}

	metrics.RecordTaskDeadLettered(t.Type)

	return nil
}

func (q *Queue) GetDeadLetterTasks() ([]*task.Task, error) {
	var tasks []*task.Task

	iter := q.client.Scan(q.ctx, 0, "dlq:task:*", 100).Iterator()
	for iter.Next(q.ctx) {
		key := iter.Val()

		data, err := q.client.Get(q.ctx, key).Result()
		if err != nil {
			continue
		}

		t, err := task.TaskFromJSON(data)
		if err != nil {
			continue
		}

		tasks = append(tasks, t)
	}

	if err := iter.Err(); err != nil {
		return nil, err
	}

	return tasks, nil
}

func (q *Queue) GetDeadLetterTask(taskID string) (*task.Task, error) {
	data, err := q.client.Get(
		q.ctx,
		"dlq:task:"+taskID,
	).Result()
	if err != nil {
		return nil, err
	}

	return task.TaskFromJSON(data)
}

func (q *Queue) RetryDeadLetterTask(taskID string) error {
	data, err := q.client.Get(q.ctx, "dlq:task:"+taskID).Result()
	if err != nil {
		return err
	}

	t, err := task.TaskFromJSON(data)
	if err != nil {
		return err
	}

	t.RetryCount = 0
	t.FailureReason = ""
	t.MoveToDLQAt = nil
	t.ScheduledAt = time.Now()
	t.Status = task.PendingStatus

	if err := q.Enqueue(t); err != nil {
		return err
	}

	q.client.Del(q.ctx, "dlq:task:"+taskID)
	return nil
}

func (q *Queue) PurgeDeadLetterTask(taskID string) error {
	return q.client.Del(
		q.ctx,
		"dlq:task:"+taskID,
	).Err()
}

func (q *Queue) GetDeadLetterStats() (map[string]any, error) {
	var count int

	iter := q.client.Scan(q.ctx, 0, "dlq:task:*", 100).Iterator()
	for iter.Next(q.ctx) {
		count++
	}

	if err := iter.Err(); err != nil {
		return nil, err
	}

	return map[string]any{
		"total_tasks": count,
	}, nil
}

func (q *Queue) IncrementRetryCount(taskID string) error {
	if q.repo != nil {
		return q.repo.IncrementRetryCount(q.ctx, taskID)
	}

	return nil
}

func (q *Queue) LogExecution(taskID string, attemptNumber int, status string, durationMs int, errorMsg string, workerID string) error {
	if q.repo != nil {
		return q.repo.LogExecution(q.ctx, taskID, attemptNumber, status, durationMs, errorMsg, workerID)
	}

	return nil
}

func (q *Queue) GetRepository() repository.TaskRepository {
	return q.repo
}

func (q *Queue) Close() error {
	return q.client.Close()
}

func (q *Queue) UpdateMetrics() error {
	tasks, err := q.GetAllTasks()
	if err != nil {
		return err
	}

	tasksByStatus := make(map[task.TaskStatus]map[string]int)
	for _, t := range tasks {
		if tasksByStatus[t.Status] == nil {
			tasksByStatus[t.Status] = make(map[string]int)
		}
		tasksByStatus[t.Status][t.Type]++
	}

	metrics.UpdateTaskGauges(tasksByStatus)
	metrics.UpdateQueueDepth(len(tasks))

	dlqTasks, err := q.GetDeadLetterTasks()
	if err == nil {
		metrics.UpdateDeadLetterQueueDepth(len(dlqTasks))
	}

	return nil
}
