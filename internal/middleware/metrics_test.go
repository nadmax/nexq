package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type mockMetricsRecorder struct {
	records []metricRecord
}

type metricRecord struct {
	method   string
	endpoint string
	status   string
	duration time.Duration
}

func (m *mockMetricsRecorder) record(method, endpoint, status string, duration time.Duration) {
	m.records = append(m.records, metricRecord{
		method:   method,
		endpoint: endpoint,
		status:   status,
		duration: duration,
	})
}

func (m *mockMetricsRecorder) reset() {
	m.records = []metricRecord{}
}

var mockRecorder = &mockMetricsRecorder{}

func setupMock() func() {
	original := recordHTTPRequest
	recordHTTPRequest = func(method, endpoint, status string, duration time.Duration) {
		mockRecorder.record(method, endpoint, status, duration)
	}
	return func() { recordHTTPRequest = original }
}

func TestResponseWriter_WriteHeader(t *testing.T) {
	tests := []struct {
		name           string
		statusCode     int
		expectedStatus int
	}{
		{
			name:           "sets status code 200",
			statusCode:     http.StatusOK,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "sets status code 404",
			statusCode:     http.StatusNotFound,
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "sets status code 500",
			statusCode:     http.StatusInternalServerError,
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			rw := &responseWriter{
				ResponseWriter: rec,
				statusCode:     http.StatusOK,
			}

			rw.WriteHeader(tt.statusCode)

			if rw.statusCode != tt.expectedStatus {
				t.Errorf("expected status code %d, got %d", tt.expectedStatus, rw.statusCode)
			}

			if rec.Code != tt.expectedStatus {
				t.Errorf("expected underlying response writer status %d, got %d", tt.expectedStatus, rec.Code)
			}
		})
	}
}

func TestResponseWriter_DefaultStatusCode(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := &responseWriter{
		ResponseWriter: rec,
		statusCode:     http.StatusOK,
	}

	if rw.statusCode != http.StatusOK {
		t.Errorf("expected default status code %d, got %d", http.StatusOK, rw.statusCode)
	}
}

func TestNormalizeEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "task by id",
			path:     "/api/tasks/123",
			expected: "/api/tasks/:id",
		},
		{
			name:     "task with uuid",
			path:     "/api/tasks/abc-def-456",
			expected: "/api/tasks/:id",
		},
		{
			name:     "task with nested path (should not normalize)",
			path:     "/api/tasks/123/subtask",
			expected: "/api/tasks/123/subtask",
		},
		{
			name:     "dlq task by id",
			path:     "/api/dlq/tasks/456",
			expected: "/api/dlq/tasks/:id",
		},
		{
			name:     "dlq task retry",
			path:     "/api/dlq/tasks/789/retry",
			expected: "/api/dlq/tasks/:id/retry",
		},
		{
			name:     "history task by id",
			path:     "/api/history/task/101",
			expected: "/api/history/task/:id",
		},
		{
			name:     "history by type",
			path:     "/api/history/type/email",
			expected: "/api/history/type/:type",
		},
		{
			name:     "history by type with long name",
			path:     "/api/history/type/notification-email",
			expected: "/api/history/type/:type",
		},
		{
			name:     "root path",
			path:     "/",
			expected: "/",
		},
		{
			name:     "health endpoint",
			path:     "/health",
			expected: "/health",
		},
		{
			name:     "metrics endpoint",
			path:     "/metrics",
			expected: "/metrics",
		},
		{
			name:     "api tasks list",
			path:     "/api/tasks",
			expected: "/api/tasks",
		},
		{
			name:     "unknown endpoint",
			path:     "/api/unknown/path",
			expected: "/api/unknown/path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeEndpoint(tt.path)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestMetricsMiddleware(t *testing.T) {
	cleanup := setupMock()
	defer cleanup()

	tests := []struct {
		name               string
		method             string
		path               string
		handlerStatusCode  int
		expectedMethod     string
		expectedEndpoint   string
		expectedStatusCode string
	}{
		{
			name:               "GET task by id with 200",
			method:             http.MethodGet,
			path:               "/api/tasks/123",
			handlerStatusCode:  http.StatusOK,
			expectedMethod:     http.MethodGet,
			expectedEndpoint:   "/api/tasks/:id",
			expectedStatusCode: "200",
		},
		{
			name:               "POST task with 201",
			method:             http.MethodPost,
			path:               "/api/tasks",
			handlerStatusCode:  http.StatusCreated,
			expectedMethod:     http.MethodPost,
			expectedEndpoint:   "/api/tasks",
			expectedStatusCode: "201",
		},
		{
			name:               "DELETE task with 404",
			method:             http.MethodDelete,
			path:               "/api/tasks/999",
			handlerStatusCode:  http.StatusNotFound,
			expectedMethod:     http.MethodDelete,
			expectedEndpoint:   "/api/tasks/:id",
			expectedStatusCode: "404",
		},
		{
			name:               "GET dlq task retry with 200",
			method:             http.MethodGet,
			path:               "/api/dlq/tasks/456/retry",
			handlerStatusCode:  http.StatusOK,
			expectedMethod:     http.MethodGet,
			expectedEndpoint:   "/api/dlq/tasks/:id/retry",
			expectedStatusCode: "200",
		},
		{
			name:               "internal server error",
			method:             http.MethodGet,
			path:               "/api/tasks/123",
			handlerStatusCode:  http.StatusInternalServerError,
			expectedMethod:     http.MethodGet,
			expectedEndpoint:   "/api/tasks/:id",
			expectedStatusCode: "500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRecorder.reset()

			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.handlerStatusCode)
				_, _ = w.Write([]byte("test response"))
			})

			handler := MetricsMiddleware(testHandler)
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.handlerStatusCode {
				t.Errorf("expected status code %d, got %d", tt.handlerStatusCode, rec.Code)
			}

			if len(mockRecorder.records) != 1 {
				t.Fatalf("expected 1 metric recorded, got %d", len(mockRecorder.records))
			}

			m := mockRecorder.records[0]
			if m.method != tt.expectedMethod {
				t.Errorf("expected method %q, got %q", tt.expectedMethod, m.method)
			}
			if m.endpoint != tt.expectedEndpoint {
				t.Errorf("expected endpoint %q, got %q", tt.expectedEndpoint, m.endpoint)
			}
			if m.status != tt.expectedStatusCode {
				t.Errorf("expected status %q, got %q", tt.expectedStatusCode, m.status)
			}
			if m.duration <= 0 {
				t.Error("expected duration > 0")
			}
		})
	}
}

func TestMetricsMiddleware_CallsNextHandler(t *testing.T) {
	cleanup := setupMock()
	defer cleanup()

	mockRecorder.reset()
	handlerCalled := false

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	handler := MetricsMiddleware(testHandler)
	req := httptest.NewRequest(http.MethodGet, "/api/tasks", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !handlerCalled {
		t.Error("expected next handler to be called")
	}
}

func TestMetricsMiddleware_RecordsDuration(t *testing.T) {
	cleanup := setupMock()
	defer cleanup()

	mockRecorder.reset()
	delay := 50 * time.Millisecond

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(delay)
		w.WriteHeader(http.StatusOK)
	})

	handler := MetricsMiddleware(testHandler)
	req := httptest.NewRequest(http.MethodGet, "/api/tasks", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if len(mockRecorder.records) != 1 {
		t.Fatalf("expected 1 metric recorded, got %d", len(mockRecorder.records))
	}

	recorded := mockRecorder.records[0]
	if recorded.duration < delay {
		t.Errorf("expected duration >= %v, got %v", delay, recorded.duration)
	}
}
