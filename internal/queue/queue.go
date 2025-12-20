package queue

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

type Queue struct {
	client *redis.Client
	ctx    context.Context
}

func NewQueue(redisAddr string) (*Queue, error) {
	client := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})

	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &Queue{
		client: client,
		ctx:    ctx,
	}, nil
}

func (q *Queue) Enqueue(task *Task) error {
	data, err := task.ToJSON()
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
		task.ID,
		0,
	).Err(); err != nil {
		return err
	}

	return q.client.Set(
		q.ctx,
		"task:"+task.ID,
		data,
		0,
	).Err()
}

func (q *Queue) Dequeue() (*Task, error) {
	// Get current head and tail to check if queue has items
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

	// No items in queue
	if head >= tail {
		return nil, nil
	}

	// Increment head to claim next item
	newHead, err := q.client.Incr(q.ctx, "queue:head").Result()
	if err != nil {
		return nil, err
	}

	// Read the item at the position we just claimed
	itemKey := fmt.Sprintf("queue:item:%d", newHead)
	taskID, err := q.client.Get(q.ctx, itemKey).Result()
	if err != nil {
		return nil, nil // Item doesn't exist
	}

	data, err := q.client.Get(q.ctx, "task:"+taskID).Result()
	if err != nil {
		return nil, err
	}

	q.client.Del(q.ctx, itemKey)
	q.client.Del(q.ctx, "task:"+taskID)

	return TaskFromJSON(data)
}

func (q *Queue) UpdateTask(task *Task) error {
	data, err := task.ToJSON()
	if err != nil {
		return err
	}

	return q.client.Set(
		q.ctx,
		"task:"+task.ID,
		data,
		0,
	).Err()
}

func (q *Queue) GetTask(taskID string) (*Task, error) {
	data, err := q.client.Get(
		q.ctx,
		"task:"+taskID,
	).Result()
	if err != nil {
		return nil, err
	}

	return TaskFromJSON(data)
}

func (q *Queue) GetAllTasks() ([]*Task, error) {
	var tasks []*Task

	iter := q.client.Scan(q.ctx, 0, "task:*", 100).Iterator()
	for iter.Next(q.ctx) {
		key := iter.Val()

		data, err := q.client.Get(q.ctx, key).Result()
		if err != nil {
			continue
		}

		task, err := TaskFromJSON(data)
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

func (q *Queue) MoveToDeadLetter(task *Task, reason string) error {
	task.FailureReason = reason
	task.MoveToDLQAt = time.Now()

	data, err := task.ToJSON()
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
		task.ID,
		0,
	).Err(); err != nil {
		return err
	}

	return q.client.Set(
		q.ctx,
		"dlq:task:"+task.ID,
		data,
		0,
	).Err()
}

func (q *Queue) GetDeadLetterTasks() ([]*Task, error) {
	var tasks []*Task

	iter := q.client.Scan(q.ctx, 0, "dlq:task:*", 100).Iterator()
	for iter.Next(q.ctx) {
		key := iter.Val()

		data, err := q.client.Get(q.ctx, key).Result()
		if err != nil {
			continue
		}

		task, err := TaskFromJSON(data)
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

func (q *Queue) GetDeadLetterTask(taskID string) (*Task, error) {
	data, err := q.client.Get(
		q.ctx,
		"dlq:task:"+taskID,
	).Result()
	if err != nil {
		return nil, err
	}

	return TaskFromJSON(data)
}

func (q *Queue) RetryDeadLetterTask(taskID string) error {
	data, err := q.client.Get(q.ctx, "dlq:task:"+taskID).Result()
	if err != nil {
		return err
	}

	task, err := TaskFromJSON(data)
	if err != nil {
		return err
	}

	task.RetryCount = 0
	task.FailureReason = ""
	task.MoveToDLQAt = time.Time{}
	task.ScheduledAt = time.Now()

	if err := q.Enqueue(task); err != nil {
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

func (q *Queue) Close() error {
	return q.client.Close()
}
