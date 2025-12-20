// Package api provides HTTP handlers for the job queue REST API endpoints.
package api

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/nadmax/nexq/internal/dashboard"
	"github.com/nadmax/nexq/internal/httputil"
	"github.com/nadmax/nexq/internal/queue"
)

type API struct {
	queue *queue.Queue
	mux   *http.ServeMux
}

type CreateTaskRequest struct {
	Type       string              `json:"type"`
	Payload    map[string]any      `json:"payload"`
	Priority   *queue.TaskPriority `json:"priority"`
	ScheduleIn *int                `json:"schedule_in"`
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

	dash := dashboard.NewDashboard(a.queue)
	a.mux.HandleFunc("/api/dashboard/stats", dash.GetStats)
	a.mux.HandleFunc("/api/dashboard/history", dash.GetRecentTasks)

	a.mux.HandleFunc("/api/dlq/tasks", a.handleDLQTasks)
	a.mux.HandleFunc("/api/dlq/tasks/", a.handleDLQTaskByID)
	a.mux.HandleFunc("/api/dlq/stats", a.handleDLQStats)

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

	var req CreateTaskRequest
	if err := json.Unmarshal(body, &req); err != nil {
		httputil.WriteJSONError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Type == "" {
		httputil.WriteJSONError(w, "Task type is required", http.StatusBadRequest)
		return
	}

	priority := queue.PriorityMedium
	if req.Priority != nil {
		priority = *req.Priority
	}

	task := queue.NewTask(req.Type, req.Payload, priority)
	if req.ScheduleIn != nil {
		task.ScheduledAt = time.Now().Add(time.Duration(*req.ScheduleIn) * time.Second)
	}

	if err := a.queue.Enqueue(task); err != nil {
		httputil.WriteJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(task); err != nil {
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
	if err := a.queue.RetryDeadLetterTask(taskID); err != nil {
		httputil.WriteJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}

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
