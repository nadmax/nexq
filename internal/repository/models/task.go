// Package models contains data structures used by the task repository layer.
package models

import "time"

type TaskStats struct {
	Type          string  `json:"type"`
	Status        string  `json:"status"`
	Count         int     `json:"count"`
	AvgDurationMs float64 `json:"avg_duration_ms"`
	MaxDurationMs int     `json:"max_duration_ms"`
	MinDurationMs int     `json:"min_duration_ms"`
	AvgRetries    float64 `json:"avg_retries"`
}

type RecentTask struct {
	TaskID        string     `json:"task_id"`
	Type          string     `json:"type"`
	Status        string     `json:"status"`
	CreatedAt     time.Time  `json:"created_at"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
	DurationMs    *int       `json:"duration_ms,omitempty"`
	RetryCount    int        `json:"retry_count"`
	FailureReason string     `json:"failure_reason,omitempty"`
}
