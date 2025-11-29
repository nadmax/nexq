package api

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/nadmax/nexq/internal/dashboard"
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
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a *API) createTask(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	defer func() {
		if err := r.Body.Close(); err != nil {
			log.Printf("failed to close request body: %v", err)
		}
	}()

	var req CreateTaskRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Type == "" {
		http.Error(w, "Task type is required", http.StatusBadRequest)
		return
	}

	task := queue.NewTask(req.Type, req.Payload)
	if req.Priority != nil {
		task.Priority = *req.Priority
	}
	if req.ScheduleIn != nil {
		task.ScheduledAt = time.Now().Add(time.Duration(*req.ScheduleIn) * time.Second)
	}

	if err := a.queue.Enqueue(task); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(task); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

func (a *API) listTasks(w http.ResponseWriter, _ *http.Request) {
	tasks, err := a.queue.GetAllTasks()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(tasks); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

func (a *API) handleTaskByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	taskID := strings.TrimPrefix(r.URL.Path, "/api/tasks/")
	if taskID == "" {
		http.Error(w, "Task ID is required", http.StatusBadRequest)
		return
	}

	task, err := a.queue.GetTask(taskID)
	if err != nil {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(task); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}
