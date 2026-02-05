// Package api provides HTTP handlers for the job queue REST API endpoints.
package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/nadmax/nexq/internal/dashboard"
	"github.com/nadmax/nexq/internal/httputil"
	"github.com/nadmax/nexq/internal/metrics"
	"github.com/nadmax/nexq/internal/queue"
	"github.com/nadmax/nexq/internal/task"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type API struct {
	queue *queue.Queue
	mux   *http.ServeMux
}

type TaskRequest struct {
	Type       string             `json:"type"`
	Payload    map[string]any     `json:"payload"`
	Priority   *task.TaskPriority `json:"priority"`
	ScheduleIn *int               `json:"schedule_in"`
}

func NewAPI(q *queue.Queue) *API {
	api := &API{
		queue: q,
		mux:   http.NewServeMux(),
	}

	api.setupRoutes()
	return api
}

func (a *API) setupRoutes() {
	a.mux.HandleFunc("/api/tasks", a.handleTasks)
	a.mux.HandleFunc("/api/tasks/", a.handleTaskByID)
	a.mux.HandleFunc("/api/tasks/cancel/", a.handleCancelTask)

	dash := dashboard.NewDashboard(a.queue)
	a.mux.HandleFunc("/api/dashboard/stats", dash.GetStats)
	a.mux.HandleFunc("/api/dashboard/history", dash.GetRecentTasks)

	a.mux.HandleFunc("/api/dlq/tasks", a.handleDLQTasks)
	a.mux.HandleFunc("/api/dlq/tasks/", a.handleDLQTaskByID)
	a.mux.HandleFunc("/api/dlq/stats", a.handleDLQStats)

	a.mux.HandleFunc("/api/history/stats", a.handleHistoryStats)
	a.mux.HandleFunc("/api/history/recent", a.handleRecentHistory)
	a.mux.HandleFunc("/api/history/task/", a.handleTaskHistory)
	a.mux.HandleFunc("/api/history/type/", a.handleTasksByType)

	a.mux.HandleFunc("/api/reports", a.listReportsHandler)
	a.mux.HandleFunc("/api/reports/download/", a.downloadReportHandler)

	a.mux.Handle("/metrics", promhttp.Handler())

	fs := http.FileServer(http.Dir("./web"))
	a.mux.Handle("/", fs)
}

func (a *API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.mux.ServeHTTP(w, r)
}

func (a *API) handleTasks(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		a.createTask(w, r)
	case http.MethodGet:
		a.listTasks(w, r)
	default:
		httputil.WriteJSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a *API) createTask(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		httputil.WriteJSONError(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	defer func() {
		if err := r.Body.Close(); err != nil {
			log.Printf("failed to close request body: %v", err)
		}
	}()

	var req TaskRequest
	if err := json.Unmarshal(body, &req); err != nil {
		httputil.WriteJSONError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Type == "" {
		httputil.WriteJSONError(w, "Task type is required", http.StatusBadRequest)
		return
	}

	priority := task.MediumPriority
	if req.Priority != nil {
		priority = *req.Priority
	}

	t := task.NewTask(req.Type, req.Payload, priority)
	if req.ScheduleIn != nil {
		t.ScheduledAt = time.Now().Add(time.Duration(*req.ScheduleIn) * time.Second)
	}

	if err := a.queue.Enqueue(t); err != nil {
		httputil.WriteJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	metrics.RecordTaskEnqueued(t.Type, t.Priority)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(t); err != nil {
		httputil.WriteJSONError(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

func (a *API) listTasks(w http.ResponseWriter, _ *http.Request) {
	tasks, err := a.queue.GetAllTasks()
	if err != nil {
		httputil.WriteJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(tasks); err != nil {
		httputil.WriteJSONError(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

func (a *API) handleTaskByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httputil.WriteJSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	taskID := strings.TrimPrefix(r.URL.Path, "/api/tasks/")
	if taskID == "" {
		httputil.WriteJSONError(w, "Task ID is required", http.StatusBadRequest)
		return
	}

	task, err := a.queue.GetTask(taskID)
	if err != nil {
		httputil.WriteJSONError(w, "Task not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(task); err != nil {
		httputil.WriteJSONError(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

func (a *API) handleCancelTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httputil.WriteJSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	taskID := strings.TrimPrefix(r.URL.Path, "/api/tasks/cancel/")
	if taskID == "" {
		httputil.WriteJSONError(w, "Task ID required", http.StatusBadRequest)
		return
	}
	if err := a.queue.CancelTask(taskID); err != nil {
		if strings.Contains(err.Error(), "not found") {
			httputil.WriteJSONError(w, err.Error(), http.StatusNotFound)
			return
		}
		if strings.Contains(err.Error(), "cannot cancel") {
			httputil.WriteJSONError(w, err.Error(), http.StatusBadRequest)
			return
		}

		httputil.WriteJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{
		"message": "Task cancelled successfully",
		"task_id": taskID,
	}); err != nil {
		httputil.WriteJSONError(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

func (a *API) handleDLQTasks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httputil.WriteJSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tasks, err := a.queue.GetDeadLetterTasks()
	if err != nil {
		httputil.WriteJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(tasks); err != nil {
		httputil.WriteJSONError(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

func (a *API) handleDLQTaskByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/dlq/tasks/")
	parts := strings.Split(path, "/")

	if len(parts) == 0 || parts[0] == "" {
		httputil.WriteJSONError(w, "Task ID is required", http.StatusBadRequest)
		return
	}

	taskID := parts[0]

	switch r.Method {
	case http.MethodGet:
		a.getDLQTask(w, taskID)
	case http.MethodDelete:
		a.purgeDLQTask(w, taskID)
	case http.MethodPost:
		if len(parts) == 2 && parts[1] == "retry" {
			a.retryDLQTask(w, taskID)
		} else {
			httputil.WriteJSONError(w, "Invalid endpoint", http.StatusNotFound)
		}
	default:
		httputil.WriteJSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a *API) getDLQTask(w http.ResponseWriter, taskID string) {
	task, err := a.queue.GetDeadLetterTask(taskID)
	if err != nil {
		httputil.WriteJSONError(w, "Task not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(task); err != nil {
		httputil.WriteJSONError(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

func (a *API) retryDLQTask(w http.ResponseWriter, taskID string) {
	t, err := a.queue.GetDeadLetterTask(taskID)
	if err != nil {
		httputil.WriteJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := a.queue.RetryDeadLetterTask(taskID); err != nil {
		httputil.WriteJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	metrics.RecordTaskRetried(t.Type)

	w.Header().Set("Content-Type", "application/json")
	response := map[string]string{
		"message": "Task moved back to queue for retry",
		"task_id": taskID,
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		httputil.WriteJSONError(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

func (a *API) purgeDLQTask(w http.ResponseWriter, taskID string) {
	if err := a.queue.PurgeDeadLetterTask(taskID); err != nil {
		httputil.WriteJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (a *API) handleDLQStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httputil.WriteJSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stats, err := a.queue.GetDeadLetterStats()
	if err != nil {
		httputil.WriteJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(stats); err != nil {
		httputil.WriteJSONError(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

func (a *API) handleHistoryStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httputil.WriteJSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	repo := a.queue.GetRepository()
	if repo == nil {
		httputil.WriteJSONError(w, "History not available (PostgreSQL not configured)", http.StatusServiceUnavailable)
		return
	}

	hours := 24
	if h := r.URL.Query().Get("hours"); h != "" {
		if parsed, err := strconv.Atoi(h); err == nil {
			hours = parsed
		}
	}

	stats, err := repo.GetTaskStats(r.Context(), hours)
	if err != nil {
		httputil.WriteJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(stats); err != nil {
		httputil.WriteJSONError(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

func (a *API) handleRecentHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httputil.WriteJSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	repo := a.queue.GetRepository()
	if repo == nil {
		httputil.WriteJSONError(w, "History not available (PostgreSQL not configured)", http.StatusServiceUnavailable)
		return
	}

	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil {
			limit = parsed
		}
	}

	tasks, err := repo.GetRecentTasks(r.Context(), limit)
	if err != nil {
		httputil.WriteJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(tasks); err != nil {
		httputil.WriteJSONError(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

func (a *API) handleTaskHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httputil.WriteJSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	repo := a.queue.GetRepository()
	if repo == nil {
		httputil.WriteJSONError(w, "History not available (PostgreSQL not configured)", http.StatusServiceUnavailable)
		return
	}

	taskID := strings.TrimPrefix(r.URL.Path, "/api/history/task/")
	if taskID == "" {
		httputil.WriteJSONError(w, "Task ID is required", http.StatusBadRequest)
		return
	}

	history, err := repo.GetTaskHistory(r.Context(), taskID)
	if err != nil {
		httputil.WriteJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(history); err != nil {
		httputil.WriteJSONError(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

func (a *API) handleTasksByType(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httputil.WriteJSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	repo := a.queue.GetRepository()
	if repo == nil {
		httputil.WriteJSONError(w, "History not available (PostgreSQL not configured)", http.StatusServiceUnavailable)
		return
	}

	taskType := strings.TrimPrefix(r.URL.Path, "/api/history/type/")
	if taskType == "" {
		httputil.WriteJSONError(w, "Task type is required", http.StatusBadRequest)
		return
	}

	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil {
			limit = parsed
		}
	}

	tasks, err := repo.GetTasksByType(r.Context(), taskType, limit)
	if err != nil {
		httputil.WriteJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(tasks); err != nil {
		httputil.WriteJSONError(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

func (a *API) listReportsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httputil.WriteJSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	reportsDir := "./reports"
	files, err := os.ReadDir(reportsDir)
	if err != nil {
		if jErr := json.NewEncoder(w).Encode([]map[string]any{}); jErr != nil {
			httputil.WriteJSONError(w, "Failed to encode response", http.StatusInternalServerError)
			return
		}

		return
	}

	var reports []map[string]any
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		info, err := file.Info()
		if err != nil {
			continue
		}

		reports = append(reports, map[string]any{
			"filename":   file.Name(),
			"size":       info.Size(),
			"created_at": info.ModTime(),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(reports); err != nil {
		httputil.WriteJSONError(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

func (a *API) downloadReportHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httputil.WriteJSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	filename := strings.TrimPrefix(r.URL.Path, "/api/reports/download/")
	if filename == "" {
		httputil.WriteJSONError(w, "Filename required", http.StatusBadRequest)
		return
	}

	if strings.Contains(filename, "..") || strings.Contains(filename, "/") {
		httputil.WriteJSONError(w, "Invalid filename", http.StatusBadRequest)
		return
	}

	reportsDir := "./reports"
	filePath := filepath.Join(reportsDir, filename)
	info, err := os.Stat(filePath)
	if err != nil {
		httputil.WriteJSONError(w, "File not found", http.StatusNotFound)
		return
	}

	if info.IsDir() {
		httputil.WriteJSONError(w, "Invalid file", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

	if strings.HasSuffix(filename, ".csv") {
		w.Header().Set("Content-Type", "text/csv")
	} else if strings.HasSuffix(filename, ".json") {
		w.Header().Set("Content-Type", "application/json")
	} else {
		w.Header().Set("Content-Type", "application/octet-stream")
	}

	http.ServeFile(w, r, filePath)
}
