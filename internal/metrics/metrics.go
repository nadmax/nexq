// Package metrics provides Prometheus metrics for monitoring the task queue system.
package metrics

import (
	"time"

	"github.com/nadmax/nexq/internal/task"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	TasksEnqueued = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "nexq_tasks_enqueued_total",
			Help: "Total number of tasks enqueued",
		},
		[]string{"type", "priority"},
	)
	TasksCompleted = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "nexq_tasks_completed_total",
			Help: "Total number of tasks completed successfully",
		},
		[]string{"type"},
	)
	TasksFailed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "nexq_tasks_failed_total",
			Help: "Total number of tasks that failed",
		},
		[]string{"type"},
	)
	TasksRetried = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "nexq_tasks_retried_total",
			Help: "Total number of task retries",
		},
		[]string{"type"},
	)
	TasksDeadLettered = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "nexq_tasks_dead_lettered_total",
			Help: "Total number of tasks moved to dead letter queue",
		},
		[]string{"type"},
	)
	TasksInQueue = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "nexq_tasks_in_queue",
			Help: "Current number of tasks in queue by status",
		},
		[]string{"status", "type"},
	)
	TaskDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "nexq_task_duration_seconds",
			Help:    "Task execution duration in seconds",
			Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10, 30, 60, 120, 300},
		},
		[]string{"type", "status"},
	)
	TaskWaitTime = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "nexq_task_wait_time_seconds",
			Help:    "Time tasks spend waiting in queue before execution",
			Buckets: []float64{.01, .05, .1, .5, 1, 5, 10, 30, 60, 300, 600, 1800, 3600},
		},
		[]string{"type", "priority"},
	)
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "nexq_http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "endpoint", "status"},
	)
	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "nexq_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "endpoint"},
	)
	QueueDepth = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "nexq_queue_depth",
			Help: "Current depth of the task queue",
		},
	)
	DeadLetterQueueDepth = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "nexq_dead_letter_queue_depth",
			Help: "Current depth of the dead letter queue",
		},
	)
	WorkersActive = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "nexq_workers_active",
			Help: "Number of currently active workers",
		},
	)
)

func RecordTaskEnqueued(taskType string, priority task.TaskPriority) {
	TasksEnqueued.WithLabelValues(taskType, priority.String()).Inc()
}

func RecordTaskCompleted(taskType string, duration time.Duration) {
	TasksCompleted.WithLabelValues(taskType).Inc()
	TaskDuration.WithLabelValues(taskType, "completed").Observe(duration.Seconds())
}

func RecordTaskFailed(taskType string, duration time.Duration) {
	TasksFailed.WithLabelValues(taskType).Inc()
	TaskDuration.WithLabelValues(taskType, "failed").Observe(duration.Seconds())
}

func RecordTaskRetried(taskType string) {
	TasksRetried.WithLabelValues(taskType).Inc()
}

func RecordTaskDeadLettered(taskType string) {
	TasksDeadLettered.WithLabelValues(taskType).Inc()
}

func RecordTaskWaitTime(taskType string, priority task.TaskPriority, waitTime time.Duration) {
	TaskWaitTime.WithLabelValues(taskType, priority.String()).Observe(waitTime.Seconds())
}

func UpdateTaskGauges(tasksByStatus map[task.TaskStatus]map[string]int) {
	TasksInQueue.Reset()
	for status, typeMap := range tasksByStatus {
		for taskType, count := range typeMap {
			TasksInQueue.WithLabelValues(string(status), taskType).Set(float64(count))
		}
	}
}

func UpdateQueueDepth(depth int) {
	QueueDepth.Set(float64(depth))
}

func UpdateDeadLetterQueueDepth(depth int) {
	DeadLetterQueueDepth.Set(float64(depth))
}

func UpdateActiveWorkers(count int) {
	WorkersActive.Set(float64(count))
}

func RecordHTTPRequest(method, endpoint, status string, duration time.Duration) {
	HTTPRequestsTotal.WithLabelValues(method, endpoint, status).Inc()
	HTTPRequestDuration.WithLabelValues(method, endpoint).Observe(duration.Seconds())
}
