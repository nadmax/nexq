// Package worker provides the background job processor that consumes and executes tasks from the queue.
package worker

import (
	"fmt"
	"log"
	"time"

	"github.com/nadmax/nexq/internal/queue"
)

type TaskHandler func(*queue.Task) error

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

	for {
		select {
		case <-w.stop:
			log.Printf("Worker %s stopped", w.id)
			return
		default:
			task, err := w.queue.Dequeue()
			if err != nil || task == nil {
				time.Sleep(w.pollInterval)
				continue
			}

			w.processTask(task)
		}
	}
}

func (w *Worker) processTask(task *queue.Task) {
	log.Printf("Worker %s processing task %s (type: %s)", w.id, task.ID, task.Type)

	now := time.Now()
	task.Status = queue.StatusRunning
	task.StartedAt = &now
	if err := w.queue.UpdateTask(task); err != nil {
		log.Printf("Failed to update task status to running: %v", err)
	}

	handler, exists := w.handlers[task.Type]
	if !exists {
		task.Status = queue.StatusFailed
		task.Error = fmt.Sprintf("no handler for task type: %s", task.Type)
		if err := w.queue.UpdateTask(task); err != nil {
			log.Printf("Failed to update task: %v", err)
		}
		return
	}

	err := handler(task)
	completedAt := time.Now()
	task.CompletedAt = &completedAt

	if err != nil {
		task.RetryCount++
		if task.RetryCount < task.MaxRetries {
			task.Status = queue.StatusPending
			task.ScheduledAt = time.Now().Add(time.Duration(task.RetryCount) * 10 * time.Second)
			if err := w.queue.Enqueue(task); err != nil {
				log.Printf("Failed to re-enqueue task: %v", err)
			}
			log.Printf("Task %s failed, will retry (%d/%d)", task.ID, task.RetryCount, task.MaxRetries)
		} else {
			task.Status = queue.StatusFailed
			task.Error = err.Error()
			if err := w.queue.UpdateTask(task); err != nil {
				log.Printf("Failed to update failed task: %v", err)
			}
			log.Printf("Task %s failed permanently: %v", task.ID, err)
		}
	} else {
		task.Status = queue.StatusCompleted
		if err := w.queue.UpdateTask(task); err != nil {
			log.Printf("Failed to update completed task: %v", err)
		}
		log.Printf("Task %s completed successfully", task.ID)
	}
}

func (w *Worker) Stop() {
	w.stop <- true
}
