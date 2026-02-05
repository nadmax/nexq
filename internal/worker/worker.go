// Package worker provides the background job processor that consumes and executes tasks from the queue.
package worker

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/nadmax/nexq/internal/queue"
	"github.com/nadmax/nexq/internal/task"
)

type TaskHandler func(context.Context, *task.Task) error

type Worker struct {
	id           string
	queue        *queue.Queue
	handlers     map[string]TaskHandler
	stop         chan bool
	pollInterval time.Duration
}

func NewWorker(id string, q *queue.Queue) *Worker {
	return &Worker{
		id:       id,
		queue:    q,
		handlers: make(map[string]TaskHandler),
		stop:     make(chan bool),
	}
}

func (w *Worker) RegisterHandler(taskType string, handler TaskHandler) {
	w.handlers[taskType] = handler
}

func (w *Worker) SetPollInterval(d time.Duration) {
	w.pollInterval = d
}

func (w *Worker) Start() {
	log.Printf("Worker %s started", w.id)

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-w.stop:
			log.Printf("Worker %s stopped", w.id)
			return
		case <-ticker.C:
			w.processNextTask()
		}
	}
}

func (w *Worker) processNextTask() {
	task, err := w.queue.Dequeue()
	if err != nil || task == nil {
		return
	}

	w.processTask(task)
}

func (w *Worker) processTask(t *task.Task) {
	log.Printf("Worker %s processing task %s (type: %s)", w.id, t.ID, t.Type)

	cancelled, err := w.queue.IsCancelled(t.ID)
	if err == nil && cancelled {
		log.Printf("Task %s was cancelled, skipping execution", t.ID)
		return
	}

	startTime := time.Now()
	t.Status = task.RunningStatus
	t.StartedAt = &startTime
	if err := w.queue.UpdateTask(t); err != nil {
		log.Printf("Failed to update task status to running: %v", err)
	}

	if err := w.queue.LogExecution(
		t.ID,
		t.RetryCount+1,
		string(task.RunningStatus),
		0,
		"",
		w.id,
	); err != nil {
		log.Printf("Warning: failed to log execution start: %v", err)
	}

	handler, exists := w.handlers[t.Type]
	if !exists {
		w.handleTaskFailure(t, fmt.Errorf("no handler for task type: %s", t.Type), startTime)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	err = handler(ctx, t)

	log.Printf("Handler returned for task %s, err=%v, ctx.Err()=%v", t.ID, err, ctx.Err())

	if ctx.Err() == context.Canceled {
		log.Printf("Task %s was cancelled during execution", t.ID)
		completedAt := time.Now()
		t.CompletedAt = &completedAt
		t.Status = task.CancelledStatus // Assuming you have this status

		durationMs := int(completedAt.Sub(startTime).Milliseconds())

		if err := w.queue.UpdateTask(t); err != nil {
			log.Printf("Failed to update cancelled task: %v", err)
		}

		if err := w.queue.LogExecution(
			t.ID,
			t.RetryCount+1,
			string(task.CancelledStatus),
			durationMs,
			"Task cancelled during execution",
			w.id,
		); err != nil {
			log.Printf("Warning: failed to log cancelled execution: %v", err)
		}

		return
	}

	completedAt := time.Now()
	t.CompletedAt = &completedAt
	durationMs := int(completedAt.Sub(startTime).Milliseconds())

	if err != nil {
		w.handleTaskFailure(t, err, startTime)
	} else {
		w.handleTaskSuccess(t, durationMs)
	}
}

func (w *Worker) handleTaskSuccess(t *task.Task, durationMs int) {
	t.Status = task.CompletedStatus
	if err := w.queue.UpdateTask(t); err != nil {
		log.Printf("Failed to update completed task: %v", err)
	}
	if err := w.queue.CompleteTask(t, durationMs); err != nil {
		log.Printf("Warning: failed to mark task as completed in history: %v", err)
	}
	if err := w.queue.LogExecution(
		t.ID,
		t.RetryCount+1,
		string(task.CompletedStatus),
		durationMs,
		"",
		w.id,
	); err != nil {
		log.Printf("Warning: failed to log execution: %v", err)
	}

	log.Printf("Worker %s completed task %s successfully in %dms", w.id, t.ID, durationMs)
}

func (w *Worker) handleTaskFailure(t *task.Task, taskErr error, startTime time.Time) {
	durationMs := int(time.Since(startTime).Milliseconds())
	t.RetryCount++
	t.Error = taskErr.Error()

	if err := w.queue.LogExecution(
		t.ID,
		t.RetryCount,
		string(task.FailedStatus),
		durationMs,
		taskErr.Error(),
		w.id,
	); err != nil {
		log.Printf("Warning: failed to log execution: %v", err)
	}

	if t.RetryCount < t.MaxRetries {
		t.Status = task.PendingStatus
		backoffDuration := time.Duration(t.RetryCount) * 10 * time.Second
		t.ScheduledAt = time.Now().Add(backoffDuration)

		if err := w.queue.Enqueue(t); err != nil {
			log.Printf("Failed to re-enqueue task: %v", err)
		}
		if err := w.queue.IncrementRetryCount(t.ID); err != nil {
			log.Printf("Warning: failed to increment retry count: %v", err)
		}
		if err := w.queue.FailTask(t, taskErr.Error(), durationMs); err != nil {
			log.Printf("Warning: failed to record task failure: %v", err)
		}

		log.Printf("Worker %s: Task %s failed, will retry (%d/%d) in %s",
			w.id, t.ID, t.RetryCount, t.MaxRetries, backoffDuration)
	} else {
		t.Status = task.FailedStatus
		if err := w.queue.UpdateTask(t); err != nil {
			log.Printf("Failed to update failed task: %v", err)
		}
		if err := w.queue.MoveToDeadLetter(t, taskErr.Error()); err != nil {
			log.Printf("Failed to move task to DLQ: %v", err)
		}

		log.Printf("Worker %s: Task %s failed permanently after %d attempts: %v",
			w.id, t.ID, t.RetryCount, taskErr)
	}
}

func (w *Worker) Stop() {
	w.stop <- true
}
