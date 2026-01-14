package handlers

import (
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/nadmax/nexq/internal/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePayload(t *testing.T) {
	tests := []struct {
		name        string
		payload     map[string]any
		expected    *ReportPayload
		expectError bool
	}{
		{
			name: "valid payload with all fields",
			payload: map[string]any{
				"report_type": "task_summary",
				"start_time":  "2024-01-01T00:00:00Z",
				"end_time":    "2024-01-02T00:00:00Z",
				"format":      "csv",
				"output_path": "/tmp/reports",
				"schedule_in": 5,
			},
			expected: &ReportPayload{
				ReportType: "task_summary",
				StartTime:  "2024-01-01T00:00:00Z",
				EndTime:    "2024-01-02T00:00:00Z",
				Format:     "csv",
				OutputPath: "/tmp/reports",
				ScheduleIn: 5,
			},
			expectError: false,
		},
		{
			name: "minimal valid payload with defaults",
			payload: map[string]any{
				"report_type": "worker_performance",
			},
			expected: &ReportPayload{
				ReportType: "worker_performance",
				Format:     "csv",
				OutputPath: "./reports",
			},
			expectError: false,
		},
		{
			name:        "missing report_type",
			payload:     map[string]any{},
			expectError: true,
		},
		{
			name: "json format",
			payload: map[string]any{
				"report_type": "failure_analysis",
				"format":      "json",
			},
			expected: &ReportPayload{
				ReportType: "failure_analysis",
				Format:     "json",
				OutputPath: "./reports",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parsePayload(tt.payload)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected.ReportType, result.ReportType)
			assert.Equal(t, tt.expected.Format, result.Format)
			assert.Equal(t, tt.expected.OutputPath, result.OutputPath)
		})
	}
}

func TestParseTimeRange(t *testing.T) {
	tests := []struct {
		name        string
		payload     *ReportPayload
		expectError bool
	}{
		{
			name: "valid time range",
			payload: &ReportPayload{
				StartTime: "2024-01-01T00:00:00Z",
				EndTime:   "2024-01-02T00:00:00Z",
			},
			expectError: false,
		},
		{
			name:        "empty times use defaults",
			payload:     &ReportPayload{},
			expectError: false,
		},
		{
			name: "invalid start time format",
			payload: &ReportPayload{
				StartTime: "invalid-date",
			},
			expectError: true,
		},
		{
			name: "invalid end time format",
			payload: &ReportPayload{
				StartTime: "2024-01-01T00:00:00Z",
				EndTime:   "not-a-date",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end, err := parseTimeRange(tt.payload)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.False(t, start.IsZero())
			assert.False(t, end.IsZero())
			assert.True(t, start.Before(end) || start.Equal(end))
		})
	}
}

func TestGenerateTaskSummary(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	rg := NewReportGenerator(db)

	startTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	endTime := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)

	rows := sqlmock.NewRows([]string{
		"type", "total_tasks", "completed", "failed", "moved_to_dlq",
		"avg_retries", "avg_duration_ms", "max_duration_ms", "min_duration_ms", "success_rate",
	}).
		AddRow("email", 100, 95, 3, 2, 1.2, 150.5, 500, 50, 95.0).
		AddRow("report", 50, 48, 2, 0, 0.5, 2000.0, 5000, 1000, 96.0)

	mock.ExpectQuery(`SELECT\s+type,.*FROM task_history.*WHERE created_at BETWEEN.*GROUP BY type`).
		WithArgs(startTime, endTime).
		WillReturnRows(rows)

	data, err := rg.generateTaskSummary(context.Background(), startTime, endTime)

	require.NoError(t, err)
	assert.Len(t, data, 3) // header + 2 rows
	assert.Equal(t, "Task Type", data[0][0])
	assert.Equal(t, "email", data[1][0])
	assert.Equal(t, "100", data[1][1])
	assert.Equal(t, "95", data[1][2])
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGenerateWorkerPerformance(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	rg := NewReportGenerator(db)

	startTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	endTime := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)

	rows := sqlmock.NewRows([]string{
		"worker_id", "tasks_processed", "completed", "failed",
		"avg_duration_ms", "max_duration_ms", "success_rate",
	}).
		AddRow("worker-1", 150, 145, 5, 200.0, 1000, 96.67).
		AddRow("worker-2", 120, 118, 2, 180.0, 800, 98.33)

	mock.ExpectQuery(`SELECT\s+COALESCE.*FROM task_history.*WHERE created_at BETWEEN.*AND worker_id IS NOT NULL`).
		WithArgs(startTime, endTime).
		WillReturnRows(rows)

	data, err := rg.generateWorkerPerformance(context.Background(), startTime, endTime)

	require.NoError(t, err)
	assert.Len(t, data, 3)
	assert.Equal(t, "Worker ID", data[0][0])
	assert.Equal(t, "worker-1", data[1][0])
	assert.Equal(t, "150", data[1][1])
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGenerateFailureAnalysis(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	rg := NewReportGenerator(db)

	startTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	endTime := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
	lastOccurrence := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	rows := sqlmock.NewRows([]string{
		"type", "error_type", "occurrences", "last_occurrence", "avg_retry_count",
	}).
		AddRow("email", "connection timeout", 10, lastOccurrence, 2.5).
		AddRow("report", "invalid data format", 5, lastOccurrence, 1.0)

	mock.ExpectQuery(`SELECT\s+type,\s+LEFT\(COALESCE.*FROM task_history.*WHERE.*status IN`).
		WithArgs(startTime, endTime).
		WillReturnRows(rows)

	data, err := rg.generateFailureAnalysis(context.Background(), startTime, endTime)

	require.NoError(t, err)
	assert.Len(t, data, 3)
	assert.Equal(t, "Task Type", data[0][0])
	assert.Equal(t, "email", data[1][0])
	assert.Equal(t, "connection timeout", data[1][1])
	assert.Equal(t, "10", data[1][2])
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGenerateHourlyBreakdown(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	rg := NewReportGenerator(db)

	startTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	endTime := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
	hour := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	rows := sqlmock.NewRows([]string{
		"hour", "total_tasks", "completed", "failed", "avg_duration_ms",
	}).
		AddRow(hour, 50, 48, 2, 150.0).
		AddRow(hour.Add(-time.Hour), 45, 44, 1, 140.0)

	mock.ExpectQuery(`SELECT\s+DATE_TRUNC\('hour', created_at\).*FROM task_history`).
		WithArgs(startTime, endTime).
		WillReturnRows(rows)

	data, err := rg.generateHourlyBreakdown(context.Background(), startTime, endTime)

	require.NoError(t, err)
	assert.Len(t, data, 3)
	assert.Equal(t, "Hour", data[0][0])
	assert.Equal(t, "2024-01-01 12:00", data[1][0])
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGenerateRetryAnalysis(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	rg := NewReportGenerator(db)

	startTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	endTime := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)

	rows := sqlmock.NewRows([]string{
		"type", "retry_count", "task_count", "eventually_succeeded", "failed", "moved_to_dlq",
	}).
		AddRow("email", 1, 20, 18, 2, 0).
		AddRow("email", 2, 5, 4, 1, 0).
		AddRow("report", 1, 10, 8, 1, 1)

	mock.ExpectQuery(`SELECT\s+type,\s+retry_count.*FROM task_history.*WHERE.*retry_count > 0`).
		WithArgs(startTime, endTime).
		WillReturnRows(rows)

	data, err := rg.generateRetryAnalysis(context.Background(), startTime, endTime)

	require.NoError(t, err)
	assert.Len(t, data, 4)
	assert.Equal(t, "Task Type", data[0][0])
	assert.Equal(t, "email", data[1][0])
	assert.Equal(t, "1", data[1][1])
	assert.Equal(t, "20", data[1][2])
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestFormatFloat(t *testing.T) {
	tests := []struct {
		name      string
		val       sql.NullFloat64
		precision int
		expected  string
	}{
		{
			name:      "valid float with 2 precision",
			val:       sql.NullFloat64{Float64: 123.456, Valid: true},
			precision: 2,
			expected:  "123.46",
		},
		{
			name:      "valid float with 0 precision",
			val:       sql.NullFloat64{Float64: 123.456, Valid: true},
			precision: 0,
			expected:  "123",
		},
		{
			name:      "null float",
			val:       sql.NullFloat64{Valid: false},
			precision: 2,
			expected:  "0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatFloat(tt.val, tt.precision)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatInt64(t *testing.T) {
	tests := []struct {
		name     string
		val      sql.NullInt64
		expected string
	}{
		{
			name:     "valid int64",
			val:      sql.NullInt64{Int64: 12345, Valid: true},
			expected: "12345",
		},
		{
			name:     "null int64",
			val:      sql.NullInt64{Valid: false},
			expected: "0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatInt64(tt.val)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSaveAsCSV(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.csv")

	data := [][]string{
		{"Header1", "Header2", "Header3"},
		{"Value1", "Value2", "Value3"},
		{"Value4", "Value5", "Value6"},
	}

	err := saveAsCSV(path, data)
	require.NoError(t, err)

	// Verify file exists and can be read
	file, err := os.Open(path)
	require.NoError(t, err)
	defer func() { _ = file.Close() }()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	require.NoError(t, err)

	assert.Equal(t, data, records)
}

func TestSaveAsJSON(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.json")

	data := [][]string{
		{"Name", "Age", "City"},
		{"Alice", "30", "NYC"},
		{"Bob", "25", "LA"},
	}

	err := saveAsJSON(path, data)
	require.NoError(t, err)

	content, err := os.ReadFile(path)
	require.NoError(t, err)

	var result map[string]any
	err = json.Unmarshal(content, &result)
	require.NoError(t, err)

	assert.Contains(t, result, "generated_at")
	assert.Contains(t, result, "data")
	assert.Contains(t, result, "total_rows")
	assert.Equal(t, float64(2), result["total_rows"])

	records := result["data"].([]any)
	assert.Len(t, records, 2)
}

func TestSaveAsJSON_InsufficientData(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.json")

	data := [][]string{
		{"Header"},
	}

	err := saveAsJSON(path, data)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient data")
}

func TestSaveReport(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name        string
		payload     *ReportPayload
		data        [][]string
		expectError bool
	}{
		{
			name: "save as CSV",
			payload: &ReportPayload{
				ReportType: "test_report",
				Format:     "csv",
				OutputPath: tmpDir,
			},
			data: [][]string{
				{"Col1", "Col2"},
				{"Val1", "Val2"},
			},
			expectError: false,
		},
		{
			name: "save as JSON",
			payload: &ReportPayload{
				ReportType: "test_report",
				Format:     "json",
				OutputPath: tmpDir,
			},
			data: [][]string{
				{"Col1", "Col2"},
				{"Val1", "Val2"},
			},
			expectError: false,
		},
		{
			name: "unsupported format",
			payload: &ReportPayload{
				ReportType: "test_report",
				Format:     "xml",
				OutputPath: tmpDir,
			},
			data: [][]string{
				{"Col1", "Col2"},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, err := saveReport(tt.payload, tt.data)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Contains(t, path, "nexq_test_report")
			assert.Contains(t, path, tt.payload.Format)

			// Verify file exists
			_, err = os.Stat(path)
			assert.NoError(t, err)
		})
	}
}

func TestGenerateReportHandler(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	rg := NewReportGenerator(db)
	tmpDir := t.TempDir()

	t.Run("successful task_summary report", func(t *testing.T) {
		tsk := &task.Task{
			ID:   "test-task-1",
			Type: "generate_report",
			Payload: map[string]any{
				"report_type": "task_summary",
				"start_time":  "2024-01-01T00:00:00Z",
				"end_time":    "2024-01-02T00:00:00Z",
				"format":      "csv",
				"output_path": tmpDir,
			},
		}

		rows := sqlmock.NewRows([]string{
			"type", "total_tasks", "completed", "failed", "moved_to_dlq",
			"avg_retries", "avg_duration_ms", "max_duration_ms", "min_duration_ms", "success_rate",
		}).AddRow("email", 10, 9, 1, 0, 0.5, 100.0, 200, 50, 90.0)

		mock.ExpectQuery(`SELECT\s+type,.*FROM task_history`).WillReturnRows(rows)

		err := rg.GenerateReportHandler(context.Background(), tsk)
		assert.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("invalid payload", func(t *testing.T) {
		tsk := &task.Task{
			ID:      "test-task-2",
			Type:    "generate_report",
			Payload: map[string]any{},
		}

		err := rg.GenerateReportHandler(context.Background(), tsk)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid payload")
	})

	t.Run("unsupported report type", func(t *testing.T) {
		tsk := &task.Task{
			ID:   "test-task-3",
			Type: "generate_report",
			Payload: map[string]any{
				"report_type": "unsupported_type",
				"output_path": tmpDir,
			},
		}

		err := rg.GenerateReportHandler(context.Background(), tsk)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported report type")
	})
}

func TestNewReportGenerator(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	rg := NewReportGenerator(db)
	assert.NotNil(t, rg)
	assert.Equal(t, db, rg.db)
}
