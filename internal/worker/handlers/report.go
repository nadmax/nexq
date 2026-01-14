// Package handlers provides task handlers for the worker.
// Each handler implements the business logic for a specific task type
// and can be registered with the worker to process tasks from the queue.
package handlers

import (
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/nadmax/nexq/internal/task"
)

type ReportPayload struct {
	ReportType string `json:"report_type"`
	StartTime  string `json:"start_time"`
	EndTime    string `json:"end_time"`
	Format     string `json:"format"`
	OutputPath string `json:"output_path"`
	ScheduleIn int    `json:"schedule_in"`
}

type ReportGenerator struct {
	db *sql.DB
}

func NewReportGenerator(db *sql.DB) *ReportGenerator {
	return &ReportGenerator{db: db}
}

func (rg *ReportGenerator) GenerateReportHandler(ctx context.Context, t *task.Task) error {
	payload, err := parsePayload(t.Payload)
	if err != nil {
		return fmt.Errorf("invalid payload: %w", err)
	}

	if payload.ScheduleIn > 0 {
		log.Printf("[Task %s] Delaying report generation by %d seconds", t.ID, payload.ScheduleIn)

		select {
		case <-time.After(time.Duration(payload.ScheduleIn) * time.Second):
		case <-ctx.Done():
			log.Printf("[Task %s] Task cancelled during delay", t.ID)
			return ctx.Err()
		}
	}

	startTime, endTime, err := parseTimeRange(payload)
	if err != nil {
		return fmt.Errorf("invalid time range: %w", err)
	}

	log.Printf("[Task %s] Generating %s report (format: %s, period: %s to %s)",
		t.ID, payload.ReportType, payload.Format, startTime.Format(time.RFC3339), endTime.Format(time.RFC3339))

	var data [][]string
	switch payload.ReportType {
	case "task_summary":
		data, err = rg.generateTaskSummary(ctx, startTime, endTime)
	case "worker_performance":
		data, err = rg.generateWorkerPerformance(ctx, startTime, endTime)
	case "failure_analysis":
		data, err = rg.generateFailureAnalysis(ctx, startTime, endTime)
	case "hourly_breakdown":
		data, err = rg.generateHourlyBreakdown(ctx, startTime, endTime)
	case "retry_analysis":
		data, err = rg.generateRetryAnalysis(ctx, startTime, endTime)
	default:
		return fmt.Errorf("unsupported report type: %s (available: task_summary, worker_performance, failure_analysis, hourly_breakdown, retry_analysis)", payload.ReportType)
	}

	if err != nil {
		return fmt.Errorf("failed to generate report: %w", err)
	}

	if ctx.Err() != nil {
		log.Printf("[Task %s] Task cancelled after data generation", t.ID)
		return ctx.Err()
	}

	outputFile, err := saveReport(payload, data)
	if err != nil {
		return fmt.Errorf("failed to save report: %w", err)
	}

	log.Printf("[Task %s] Report generated successfully: %s (%d rows)", t.ID, outputFile, len(data)-1)
	return nil
}

func parsePayload(payload map[string]any) (*ReportPayload, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	var rp ReportPayload
	if err := json.Unmarshal(data, &rp); err != nil {
		return nil, err
	}

	if rp.ReportType == "" {
		return nil, errors.New("missing required field: report_type")
	}
	if rp.OutputPath == "" {
		rp.OutputPath = "./reports"
	}
	if rp.Format == "" {
		rp.Format = "csv"
	}

	return &rp, nil
}

func parseTimeRange(payload *ReportPayload) (time.Time, time.Time, error) {
	var startTime, endTime time.Time
	var err error

	if payload.StartTime != "" {
		startTime, err = time.Parse(time.RFC3339, payload.StartTime)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid start_time format: %w", err)
		}
	} else {
		startTime = time.Now().Add(-24 * time.Hour)
	}

	if payload.EndTime != "" {
		endTime, err = time.Parse(time.RFC3339, payload.EndTime)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid end_time format: %w", err)
		}
	} else {
		endTime = time.Now()
	}

	return startTime, endTime, nil
}

func (rg *ReportGenerator) generateTaskSummary(ctx context.Context, startTime, endTime time.Time) ([][]string, error) {
	query := `
		SELECT 
			type,
			COUNT(*) as total_tasks,
			COUNT(*) FILTER (WHERE status = 'completed') as completed,
			COUNT(*) FILTER (WHERE status = 'failed') as failed,
			COUNT(*) FILTER (WHERE status = 'moved_to_dlq') as moved_to_dlq,
			AVG(retry_count) as avg_retries,
			AVG(duration_ms) FILTER (WHERE duration_ms IS NOT NULL) as avg_duration_ms,
			MAX(duration_ms) as max_duration_ms,
			MIN(duration_ms) FILTER (WHERE duration_ms > 0) as min_duration_ms,
			ROUND(100.0 * COUNT(*) FILTER (WHERE status = 'completed') / NULLIF(COUNT(*), 0), 2) as success_rate
		FROM task_history
		WHERE created_at BETWEEN $1 AND $2
		GROUP BY type
		ORDER BY total_tasks DESC
	`

	rows, err := rg.db.QueryContext(ctx, query, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			log.Printf("failed to close rows: %v", closeErr)
		}
	}()

	data := [][]string{
		{"Task Type", "Total", "Completed", "Failed", "DLQ", "Avg Retries", "Avg Duration (ms)", "Max Duration (ms)", "Min Duration (ms)", "Success Rate (%)"},
	}

	for rows.Next() {
		var taskType string
		var total, completed, failed, dlq int
		var avgRetries, avgDuration, successRate sql.NullFloat64
		var maxDuration, minDuration sql.NullInt64

		err := rows.Scan(&taskType, &total, &completed, &failed, &dlq, &avgRetries, &avgDuration, &maxDuration, &minDuration, &successRate)
		if err != nil {
			return nil, fmt.Errorf("scan failed: %w", err)
		}

		data = append(data, []string{
			taskType,
			fmt.Sprintf("%d", total),
			fmt.Sprintf("%d", completed),
			fmt.Sprintf("%d", failed),
			fmt.Sprintf("%d", dlq),
			formatFloat(avgRetries, 2),
			formatFloat(avgDuration, 0),
			formatInt64(maxDuration),
			formatInt64(minDuration),
			formatFloat(successRate, 2),
		})
	}

	return data, rows.Err()
}

func (rg *ReportGenerator) generateWorkerPerformance(ctx context.Context, startTime, endTime time.Time) ([][]string, error) {
	query := `
		SELECT 
			COALESCE(worker_id, 'unknown') as worker_id,
			COUNT(*) as tasks_processed,
			COUNT(*) FILTER (WHERE status = 'completed') as completed,
			COUNT(*) FILTER (WHERE status = 'failed') as failed,
			AVG(duration_ms) FILTER (WHERE duration_ms IS NOT NULL) as avg_duration_ms,
			MAX(duration_ms) as max_duration_ms,
			ROUND(100.0 * COUNT(*) FILTER (WHERE status = 'completed') / NULLIF(COUNT(*), 0), 2) as success_rate
		FROM task_history
		WHERE created_at BETWEEN $1 AND $2
			AND worker_id IS NOT NULL
		GROUP BY worker_id
		ORDER BY tasks_processed DESC
	`

	rows, err := rg.db.QueryContext(ctx, query, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			log.Printf("failed to close rows: %v", closeErr)
		}
	}()

	data := [][]string{
		{"Worker ID", "Tasks Processed", "Completed", "Failed", "Avg Duration (ms)", "Max Duration (ms)", "Success Rate (%)"},
	}

	for rows.Next() {
		var workerID string
		var tasksProcessed, completed, failed int
		var avgDuration, successRate sql.NullFloat64
		var maxDuration sql.NullInt64

		err := rows.Scan(&workerID, &tasksProcessed, &completed, &failed, &avgDuration, &maxDuration, &successRate)
		if err != nil {
			return nil, fmt.Errorf("scan failed: %w", err)
		}

		data = append(data, []string{
			workerID,
			fmt.Sprintf("%d", tasksProcessed),
			fmt.Sprintf("%d", completed),
			fmt.Sprintf("%d", failed),
			formatFloat(avgDuration, 0),
			formatInt64(maxDuration),
			formatFloat(successRate, 2),
		})
	}

	return data, rows.Err()
}

func (rg *ReportGenerator) generateFailureAnalysis(ctx context.Context, startTime, endTime time.Time) ([][]string, error) {
	query := `
		SELECT 
			type,
			LEFT(COALESCE(failure_reason, last_error, 'unknown'), 100) as error_type,
			COUNT(*) as occurrences,
			MAX(created_at) as last_occurrence,
			AVG(retry_count) as avg_retry_count
		FROM task_history
		WHERE created_at BETWEEN $1 AND $2
			AND status IN ('failed', 'moved_to_dlq')
		GROUP BY type, LEFT(COALESCE(failure_reason, last_error, 'unknown'), 100)
		ORDER BY occurrences DESC
		LIMIT 50
	`

	rows, err := rg.db.QueryContext(ctx, query, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			log.Printf("failed to close rows: %v", closeErr)
		}
	}()

	data := [][]string{
		{"Task Type", "Error", "Occurrences", "Last Occurrence", "Avg Retry Count"},
	}

	for rows.Next() {
		var taskType, errorType string
		var occurrences int
		var lastOccurrence time.Time
		var avgRetryCount sql.NullFloat64

		err := rows.Scan(&taskType, &errorType, &occurrences, &lastOccurrence, &avgRetryCount)
		if err != nil {
			return nil, fmt.Errorf("scan failed: %w", err)
		}

		data = append(data, []string{
			taskType,
			errorType,
			fmt.Sprintf("%d", occurrences),
			lastOccurrence.Format("2006-01-02 15:04:05"),
			formatFloat(avgRetryCount, 2),
		})
	}

	return data, rows.Err()
}

func (rg *ReportGenerator) generateHourlyBreakdown(ctx context.Context, startTime, endTime time.Time) ([][]string, error) {
	query := `
		SELECT 
			DATE_TRUNC('hour', created_at) as hour,
			COUNT(*) as total_tasks,
			COUNT(*) FILTER (WHERE status = 'completed') as completed,
			COUNT(*) FILTER (WHERE status = 'failed') as failed,
			AVG(duration_ms) FILTER (WHERE duration_ms IS NOT NULL) as avg_duration_ms
		FROM task_history
		WHERE created_at BETWEEN $1 AND $2
		GROUP BY DATE_TRUNC('hour', created_at)
		ORDER BY hour DESC
	`

	rows, err := rg.db.QueryContext(ctx, query, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			log.Printf("failed to close rows: %v", closeErr)
		}
	}()

	data := [][]string{
		{"Hour", "Total Tasks", "Completed", "Failed", "Avg Duration (ms)"},
	}

	for rows.Next() {
		var hour time.Time
		var total, completed, failed int
		var avgDuration sql.NullFloat64

		err := rows.Scan(&hour, &total, &completed, &failed, &avgDuration)
		if err != nil {
			return nil, fmt.Errorf("scan failed: %w", err)
		}

		data = append(data, []string{
			hour.Format("2006-01-02 15:00"),
			fmt.Sprintf("%d", total),
			fmt.Sprintf("%d", completed),
			fmt.Sprintf("%d", failed),
			formatFloat(avgDuration, 0),
		})
	}

	return data, rows.Err()
}

func (rg *ReportGenerator) generateRetryAnalysis(ctx context.Context, startTime, endTime time.Time) ([][]string, error) {
	query := `
		SELECT 
			type,
			retry_count,
			COUNT(*) as task_count,
			COUNT(*) FILTER (WHERE status = 'completed') as eventually_succeeded,
			COUNT(*) FILTER (WHERE status = 'failed') as failed,
			COUNT(*) FILTER (WHERE status = 'moved_to_dlq') as moved_to_dlq
		FROM task_history
		WHERE created_at BETWEEN $1 AND $2
			AND retry_count > 0
		GROUP BY type, retry_count
		ORDER BY type, retry_count
	`

	rows, err := rg.db.QueryContext(ctx, query, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			log.Printf("failed to close rows: %v", closeErr)
		}
	}()

	data := [][]string{
		{"Task Type", "Retry Count", "Total", "Eventually Succeeded", "Failed", "Moved to DLQ"},
	}

	for rows.Next() {
		var taskType string
		var retryCount, taskCount, succeeded, failed, dlq int

		err := rows.Scan(&taskType, &retryCount, &taskCount, &succeeded, &failed, &dlq)
		if err != nil {
			return nil, fmt.Errorf("scan failed: %w", err)
		}

		data = append(data, []string{
			taskType,
			fmt.Sprintf("%d", retryCount),
			fmt.Sprintf("%d", taskCount),
			fmt.Sprintf("%d", succeeded),
			fmt.Sprintf("%d", failed),
			fmt.Sprintf("%d", dlq),
		})
	}

	return data, rows.Err()
}

func formatFloat(val sql.NullFloat64, precision int) string {
	if !val.Valid {
		return "0"
	}
	return fmt.Sprintf("%.*f", precision, val.Float64)
}

func formatInt64(val sql.NullInt64) string {
	if !val.Valid {
		return "0"
	}
	return fmt.Sprintf("%d", val.Int64)
}

func saveReport(payload *ReportPayload, data [][]string) (string, error) {
	if err := os.MkdirAll(payload.OutputPath, 0755); err != nil {
		return "", err
	}

	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("nexq_%s_%s.%s", payload.ReportType, timestamp, payload.Format)
	fullPath := filepath.Join(payload.OutputPath, filename)

	switch payload.Format {
	case "csv":
		return fullPath, saveAsCSV(fullPath, data)
	case "json":
		return fullPath, saveAsJSON(fullPath, data)
	default:
		return "", fmt.Errorf("unsupported format: %s", payload.Format)
	}
}

func saveAsCSV(path string, data [][]string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() {
		if fileErr := file.Close(); err != nil {
			log.Printf("failed to close file: %v", fileErr)
		}
	}()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	return writer.WriteAll(data)
}

func saveAsJSON(path string, data [][]string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() {
		if fileErr := file.Close(); err != nil {
			log.Printf("failed to close file: %v", fileErr)
		}
	}()

	if len(data) < 2 {
		return errors.New("insufficient data for JSON export")
	}

	headers := data[0]
	rows := data[1:]

	var records []map[string]string
	for _, row := range rows {
		record := make(map[string]string)
		for i, header := range headers {
			if i < len(row) {
				record[header] = row[i]
			}
		}

		records = append(records, record)
	}

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(map[string]any{
		"generated_at": time.Now().Format(time.RFC3339),
		"data":         records,
		"total_rows":   len(records),
	})
}
