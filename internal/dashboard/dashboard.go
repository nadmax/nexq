// Package dashboard implements the web-based monitoring interface for queue metrics and job status.
package dashboard

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/nadmax/nexq/internal/httputil"
	"github.com/nadmax/nexq/internal/queue"
)

type Dashboard struct {
	queue *queue.Queue
}

type Stats struct {
	TotalTasks      int            `json:"total_tasks"`
	PendingTasks    int            `json:"pending_tasks"`
	RunningTasks    int            `json:"running_tasks"`
	CompletedTasks  int            `json:"completed_tasks"`
	FailedTasks     int            `json:"failed_tasks"`
	DeadLetterTasks int            `json:"dead_letter_tasks"`
	TasksByType     map[string]int `json:"tasks_by_type"`
	AverageWaitTime string         `json:"average_wait_time"`
	LastUpdated     time.Time      `json:"last_updated"`
}

type TaskHistory struct {
	TaskID      string           `json:"task_id"`
	Type        string           `json:"type"`
	Status      queue.TaskStatus `json:"status"`
	CreatedAt   time.Time        `json:"created_at"`
	CompletedAt *time.Time       `json:"completed_at"`
	Duration    string           `json:"duration"`
}

func NewDashboard(q *queue.Queue) *Dashboard {
	return &Dashboard{queue: q}
}

func (d *Dashboard) GetStats(w http.ResponseWriter, r *http.Request) {
	tasks, err := d.queue.GetAllTasks()
	if err != nil {
		httputil.WriteJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	stats := Stats{
		TotalTasks:  len(tasks),
		TasksByType: make(map[string]int),
		LastUpdated: time.Now(),
	}

	var totalWaitTime time.Duration
	waitCount := 0

	for _, task := range tasks {
		switch task.Status {
		case queue.StatusPending:
			stats.PendingTasks++
		case queue.StatusRunning:
			stats.RunningTasks++
		case queue.StatusCompleted:
			stats.CompletedTasks++
		case queue.StatusFailed:
			stats.FailedTasks++
		case queue.StatusDeadLetter:
			stats.DeadLetterTasks++
		}

		stats.TasksByType[task.Type]++

		if task.StartedAt != nil {
			waitTime := task.StartedAt.Sub(task.CreatedAt)
			totalWaitTime += waitTime
			waitCount++
		}
	}

	if waitCount > 0 {
		avgWait := totalWaitTime / time.Duration(waitCount)
		stats.AverageWaitTime = avgWait.Round(time.Millisecond).String()
	} else {
		stats.AverageWaitTime = "N/A"
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(stats); err != nil {
		httputil.WriteJSONError(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

func (d *Dashboard) GetRecentTasks(w http.ResponseWriter, r *http.Request) {
	tasks, err := d.queue.GetAllTasks()
	if err != nil {
		httputil.WriteJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	cutoff := time.Now().Add(-24 * time.Hour)
	history := []TaskHistory{}

	for _, task := range tasks {
		if task.CompletedAt == nil {
			continue
		}
		if task.CompletedAt.Before(cutoff) {
			continue
		}

		var duration string
		if task.StartedAt != nil {
			duration = task.CompletedAt.Sub(*task.StartedAt).Round(time.Millisecond).String()
		}

		history = append(history, TaskHistory{
			TaskID:      task.ID,
			Type:        task.Type,
			Status:      task.Status,
			CreatedAt:   task.CreatedAt,
			CompletedAt: task.CompletedAt,
			Duration:    duration,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(history); err != nil {
		httputil.WriteJSONError(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}
