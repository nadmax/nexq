package main

import (
	"log"
	"time"

	"github.com/nadmax/nexq/internal/metrics"
	"github.com/nadmax/nexq/internal/queue"
	"github.com/nadmax/nexq/internal/task"
)

func startMetricsCollector(q *queue.Queue) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		updateQueueMetrics(q)
	}
}

func updateQueueMetrics(q *queue.Queue) {
	tasks, err := q.GetAllTasks()
	if err != nil {
		log.Printf("Failed to get tasks for metrics: %v", err)
		return
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
}
