package metrics

import (
	"testing"
	"time"

	"github.com/nadmax/nexq/internal/task"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecordTaskEnqueued(t *testing.T) {
	// Reset counter before test
	TasksEnqueued.Reset()

	tests := []struct {
		name     string
		taskType string
		priority task.TaskPriority
	}{
		{
			name:     "high priority task",
			taskType: "email",
			priority: task.HighPriority,
		},
		{
			name:     "normal priority task",
			taskType: "notification",
			priority: task.MediumPriority,
		},
		{
			name:     "low priority task",
			taskType: "cleanup",
			priority: task.LowPriority,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RecordTaskEnqueued(tt.taskType, tt.priority)

			metric := getCounterValue(t, TasksEnqueued, tt.taskType, tt.priority.String())
			assert.Greater(t, metric, 0.0, "counter should be incremented")
		})
	}
}

func TestRecordTaskCompleted(t *testing.T) {
	TasksCompleted.Reset()
	TaskDuration.Reset()

	taskType := "test-task"
	duration := 2 * time.Second

	RecordTaskCompleted(taskType, duration)

	completedCount := getCounterValue(t, TasksCompleted, taskType)
	assert.Equal(t, 1.0, completedCount, "completed counter should be 1")

	durationSum := getHistogramSum(t, TaskDuration, taskType, "completed")
	assert.Equal(t, 2.0, durationSum, "duration should be recorded")
}

func TestRecordTaskFailed(t *testing.T) {
	TasksFailed.Reset()
	TaskDuration.Reset()

	taskType := "failing-task"
	duration := 500 * time.Millisecond

	RecordTaskFailed(taskType, duration)

	failedCount := getCounterValue(t, TasksFailed, taskType)
	assert.Equal(t, 1.0, failedCount, "failed counter should be 1")

	durationSum := getHistogramSum(t, TaskDuration, taskType, "failed")
	assert.Equal(t, 0.5, durationSum, "duration should be recorded")
}

func TestRecordTaskCancelled(t *testing.T) {
	TasksCancelled.Reset()

	taskType := "cancelled-task"
	RecordTaskCancelled(taskType)

	count := getCounterValue(t, TasksCancelled, taskType)
	assert.Equal(t, 1.0, count, "cancelled counter should be 1")
}

func TestRecordTaskRetried(t *testing.T) {
	TasksRetried.Reset()

	taskType := "retry-task"
	RecordTaskRetried(taskType)

	count := getCounterValue(t, TasksRetried, taskType)
	assert.Equal(t, 1.0, count, "retried counter should be 1")
}

func TestRecordTaskDeadLettered(t *testing.T) {
	TasksDeadLettered.Reset()

	taskType := "dead-task"
	RecordTaskDeadLettered(taskType)

	count := getCounterValue(t, TasksDeadLettered, taskType)
	assert.Equal(t, 1.0, count, "dead lettered counter should be 1")
}

func TestRecordTaskWaitTime(t *testing.T) {
	TaskWaitTime.Reset()

	tests := []struct {
		name     string
		taskType string
		priority task.TaskPriority
		waitTime time.Duration
	}{
		{
			name:     "short wait",
			taskType: "fast-task",
			priority: task.HighPriority,
			waitTime: 100 * time.Millisecond,
		},
		{
			name:     "long wait",
			taskType: "slow-task",
			priority: task.LowPriority,
			waitTime: 30 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RecordTaskWaitTime(tt.taskType, tt.priority, tt.waitTime)

			sum := getHistogramSum(t, TaskWaitTime, tt.taskType, tt.priority.String())
			assert.Equal(t, tt.waitTime.Seconds(), sum, "wait time should be recorded")
		})
	}
}

func TestUpdateTaskGauges(t *testing.T) {
	TasksInQueue.Reset()

	tasksByStatus := map[task.TaskStatus]map[string]int{
		task.PendingStatus: {
			"email":        5,
			"notification": 3,
		},
		task.RunningStatus: {
			"email": 2,
		},
		task.CompletedStatus: {
			"cleanup": 10,
		},
	}

	UpdateTaskGauges(tasksByStatus)

	emailPending := getGaugeValue(t, TasksInQueue, string(task.PendingStatus), "email")
	assert.Equal(t, 5.0, emailPending)

	notificationPending := getGaugeValue(t, TasksInQueue, string(task.PendingStatus), "notification")
	assert.Equal(t, 3.0, notificationPending)

	emailRunning := getGaugeValue(t, TasksInQueue, string(task.RunningStatus), "email")
	assert.Equal(t, 2.0, emailRunning)

	cleanupCompleted := getGaugeValue(t, TasksInQueue, string(task.CompletedStatus), "cleanup")
	assert.Equal(t, 10.0, cleanupCompleted)
}

func TestUpdateTaskGauges_Reset(t *testing.T) {
	TasksInQueue.Reset()

	initial := map[task.TaskStatus]map[string]int{
		task.PendingStatus: {
			"task1": 5,
		},
	}
	UpdateTaskGauges(initial)

	updated := map[task.TaskStatus]map[string]int{
		task.PendingStatus: {
			"task2": 3,
		},
	}
	UpdateTaskGauges(updated)

	task2Value := getGaugeValue(t, TasksInQueue, string(task.PendingStatus), "task2")
	assert.Equal(t, 3.0, task2Value)
}

func TestUpdateQueueDepth(t *testing.T) {
	depths := []int{0, 10, 100, 1000}

	for _, depth := range depths {
		UpdateQueueDepth(depth)

		metric := &dto.Metric{}
		err := QueueDepth.Write(metric)
		require.NoError(t, err)

		assert.Equal(t, float64(depth), metric.Gauge.GetValue())
	}
}

func TestUpdateDeadLetterQueueDepth(t *testing.T) {
	depths := []int{0, 5, 25, 100}

	for _, depth := range depths {
		UpdateDeadLetterQueueDepth(depth)

		metric := &dto.Metric{}
		err := DeadLetterQueueDepth.Write(metric)
		require.NoError(t, err)

		assert.Equal(t, float64(depth), metric.Gauge.GetValue())
	}
}

func TestUpdateActiveWorkers(t *testing.T) {
	counts := []int{0, 1, 5, 10, 20}

	for _, count := range counts {
		UpdateActiveWorkers(count)

		metric := &dto.Metric{}
		err := WorkersActive.Write(metric)
		require.NoError(t, err)

		assert.Equal(t, float64(count), metric.Gauge.GetValue())
	}
}

func TestRecordHTTPRequest(t *testing.T) {
	HTTPRequestsTotal.Reset()
	HTTPRequestDuration.Reset()

	tests := []struct {
		name     string
		method   string
		endpoint string
		status   string
		duration time.Duration
	}{
		{
			name:     "successful GET",
			method:   "GET",
			endpoint: "/tasks",
			status:   "200",
			duration: 50 * time.Millisecond,
		},
		{
			name:     "failed POST",
			method:   "POST",
			endpoint: "/tasks",
			status:   "500",
			duration: 100 * time.Millisecond,
		},
		{
			name:     "not found",
			method:   "GET",
			endpoint: "/unknown",
			status:   "404",
			duration: 10 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RecordHTTPRequest(tt.method, tt.endpoint, tt.status, tt.duration)

			count := getCounterValue(t, HTTPRequestsTotal, tt.method, tt.endpoint, tt.status)
			assert.Greater(t, count, 0.0, "request counter should be incremented")

			sum := getHistogramSum(t, HTTPRequestDuration, tt.method, tt.endpoint)
			assert.Greater(t, sum, 0.0, "duration should be recorded")
		})
	}
}

func TestTaskDurationHistogramBuckets(t *testing.T) {
	TaskDuration.Reset()

	durations := []time.Duration{
		5 * time.Millisecond,
		100 * time.Millisecond,
		1 * time.Second,
		30 * time.Second,
		2 * time.Minute,
	}

	for _, d := range durations {
		RecordTaskCompleted("bucket-test", d)
	}

	metric := getHistogramMetric(t, TaskDuration, "bucket-test", "completed")
	assert.Equal(t, uint64(len(durations)), metric.Histogram.GetSampleCount())
}

func TestTaskWaitTimeHistogramBuckets(t *testing.T) {
	TaskWaitTime.Reset()

	waitTimes := []time.Duration{
		10 * time.Millisecond,
		1 * time.Second,
		1 * time.Minute,
		10 * time.Minute,
		1 * time.Hour,
	}

	for i, wt := range waitTimes {
		RecordTaskWaitTime("wait-test", task.MediumPriority, wt)

		metric := getHistogramMetric(t, TaskWaitTime, "wait-test", task.MediumPriority.String())
		expectedCount := uint64(len(waitTimes[:i+1]))
		assert.Equal(t, expectedCount, metric.Histogram.GetSampleCount())
	}
}

func getCounterValue(t *testing.T, counter *prometheus.CounterVec, labels ...string) float64 {
	metric := &dto.Metric{}
	observer, err := counter.GetMetricWithLabelValues(labels...)
	require.NoError(t, err)

	c := observer
	err = c.Write(metric)
	require.NoError(t, err)
	return metric.Counter.GetValue()
}

func getGaugeValue(t *testing.T, gauge *prometheus.GaugeVec, labels ...string) float64 {
	metric := &dto.Metric{}
	observer, err := gauge.GetMetricWithLabelValues(labels...)
	require.NoError(t, err)

	g := observer
	err = g.Write(metric)
	require.NoError(t, err)
	return metric.Gauge.GetValue()
}

func getHistogramSum(t *testing.T, histogram *prometheus.HistogramVec, labels ...string) float64 {
	metric := getHistogramMetric(t, histogram, labels...)
	return metric.Histogram.GetSampleSum()
}

func getHistogramMetric(t *testing.T, histogram *prometheus.HistogramVec, labels ...string) *dto.Metric {
	metric := &dto.Metric{}
	observer, err := histogram.GetMetricWithLabelValues(labels...)
	require.NoError(t, err)

	h := observer.(prometheus.Histogram)
	err = h.Write(metric)
	require.NoError(t, err)
	return metric
}
