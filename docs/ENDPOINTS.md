# API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/tasks` | Create a new task |
| GET | `/api/tasks` | List all tasks |
| GET | `/api/tasks/:id` | Get task details |
| GET | `/api/dashboard/stats` | Get tasks statistics (total, pending, running, completed and failed)|
|GET | `/api/dashboard/history` | Get tasks history (from most recent to oldest) |
| GET | `/api/dlq/tasks` | List all dead letter tasks |
| GET | `/api/dlq/tasks/:id` | Get a dead letter task details |
| DELETE | `/api/dlq/tasks/:id` | Delete a dead letter task |
| POST | `/api/dlq/tasks/:id` | Retry a dead letter task |
| GET | `/api/dlq/stats` | Get dead letter queue statistics (total failed)|
| GET | `/api/history/stats` | Get stats for the last 24 hours |
| GET | `/api/history/recent` | Get the last 100 tasks |
| GET | `/api/history/task/:id` | Get execution history for a specific task |
| GET | `/api/history/type/:type`| Get tasks by type |
