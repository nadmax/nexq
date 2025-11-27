package queue

import (
	"context"
	"fmt"
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
	taskJSON, err := task.ToJSON()
	if err != nil {
		return err
	}

	if err := q.client.HSet(q.ctx, "tasks", task.ID, taskJSON).Err(); err != nil {
		return err
	}

	invertedPriority := float64(PriorityHigh - task.Priority)
	score := float64(task.ScheduledAt.Unix())*1000 + invertedPriority
	return q.client.ZAdd(q.ctx, "task_queue", redis.Z{
		Score:  score,
		Member: task.ID,
	}).Err()
}

func (q *Queue) Dequeue() (*Task, error) {
	now := time.Now().Unix()
	maxScore := float64(now)*1000 + float64(PriorityHigh-PriorityLow)

	results, err := q.client.ZRangeByScore(q.ctx, "task_queue", &redis.ZRangeBy{
		Min:   "-inf",
		Max:   fmt.Sprintf("%f", maxScore),
		Count: 1,
	}).Result()

	if err != nil || len(results) == 0 {
		return nil, err
	}

	taskID := results[0]

	q.client.ZRem(q.ctx, "task_queue", taskID)

	taskJSON, err := q.client.HGet(q.ctx, "tasks", taskID).Result()
	if err != nil {
		return nil, err
	}

	return TaskFromJSON(taskJSON)
}

func (q *Queue) UpdateTask(task *Task) error {
	taskJSON, err := task.ToJSON()
	if err != nil {
		return err
	}
	return q.client.HSet(q.ctx, "tasks", task.ID, taskJSON).Err()
}

func (q *Queue) GetTask(taskID string) (*Task, error) {
	taskJSON, err := q.client.HGet(q.ctx, "tasks", taskID).Result()
	if err != nil {
		return nil, err
	}
	return TaskFromJSON(taskJSON)
}

func (q *Queue) GetAllTasks() ([]*Task, error) {
	taskMap, err := q.client.HGetAll(q.ctx, "tasks").Result()
	if err != nil {
		return nil, err
	}

	tasks := make([]*Task, 0, len(taskMap))
	for _, taskJSON := range taskMap {
		task, err := TaskFromJSON(taskJSON)
		if err != nil {
			continue
		}
		tasks = append(tasks, task)
	}

	return tasks, nil
}

func (q *Queue) Close() error {
	return q.client.Close()
}
