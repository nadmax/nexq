# Nexq

A lightweight, high-performance distributed task queue system built in Go with Pogocache. Schedule, execute, and monitor background jobs across multiple worker nodes with blazing-fast speed and minimal resource usage.

## Features

- **Distributed Processing**: Scale horizontally by adding more worker nodes
- **Priority Queues**: High, normal, and low priority task execution
- **Scheduled Tasks**: Delay task execution or schedule for future processing
- **Automatic Retries**: Configurable retry logic with exponential backoff
- **Real-time Dashboard**: Monitor task status and worker activity through a clean web interface
- **RESTful API**: Simple HTTP API using Go's standard `net/http` package
- **Multiple Task Types**: Register custom handlers for different job types
- **Pogocache-Backed**: Ultra-fast caching with lower latency and better CPU efficiency than Redis
- **Zero Dependencies**: Minimal external dependencies, uses Go standard library where possible

## 🚀 Quick Start

### Prerequisites

- Go 1.25.3 or higher
- Pogocache server running locally or remotely

### Installation

```bash
# Clone the repository
git clone https://github.com/yourusername/nexq.git
cd nexq

# Install dependencies
go mod download

# Start Pogocache (using Docker)
docker run -d -p 9401:9401 pogocache/pogocache

# Or build Pogocache from source
git clone https://github.com/pogocache/pogocache.git
cd pogocache
make
./pogocache

# Run the API server
go run cmd/server/main.go

# In another terminal, run a worker
go run cmd/worker/main.go
```

### Usage

Open your browser and navigate to `http://localhost:8080` to access the dashboard.

**Create a task via API:**

```bash
curl -X POST http://localhost:8080/api/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "type": "send_email",
    "payload": {
      "to": "user@example.com",
      "subject": "Hello from Nexq"
    },
    "priority": 5
  }'
```

**Schedule a task for later:**

```bash
curl -X POST http://localhost:8080/api/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "type": "generate_report",
    "payload": {"report_type": "monthly"},
    "schedule_in": 3600
  }'
```

Add your own task handlers in `cmd/worker/main.go`:

```go
w.RegisterHandler("my_custom_task", func(task *queue.Task) error {
    // Your task logic here
    return nil
})
```

## Architecture

```
┌─────────────┐      ┌─────────────┐      ┌─────────────┐
│   Client    │─────▶│  API Server │─────▶│  Pogocache  │
└─────────────┘      └─────────────┘      └──────┬──────┘
                                                  │
                     ┌────────────────────────────┘
                     │
         ┌───────────┼───────────┬───────────┐
         ▼           ▼           ▼           ▼
    ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐
    │Worker 1 │ │Worker 2 │ │Worker 3 │ │Worker N │
    └─────────┘ └─────────┘ └─────────┘ └─────────┘
```

## Use Cases

- Background email sending
- Image/video processing pipelines
- Report generation
- Data synchronization tasks
- Scheduled maintenance jobs
- Webhook delivery with retries
- Batch processing operations

## Configuration

Edit Pogocache connection in your code:

```go
q, err := queue.NewQueue("localhost:9401")
```

Adjust task retry settings in `internal/queue/task.go`:

```go
MaxRetries: 3  // Number of retry attempts
```

### Pogocache Configuration

Run Pogocache with custom options:

```bash
# Bind to specific address and port
./pogocache -h 0.0.0.0 -p 9401

# Set max memory usage
./pogocache --maxmemory 2GB

# Set number of threads
./pogocache --threads 16

# Enable authentication
./pogocache --auth mypassword

# Enable TLS
./pogocache --tlsport 9401 --tlscert cert.pem --tlskey key.pem
```

## API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/tasks` | Create a new task |
| GET | `/api/tasks` | List all tasks |
| GET | `/api/tasks/:id` | Get task details |

## Future Enhancements

- [ ] Dead letter queue for permanently failed tasks
- [ ] Task dependencies and workflows
- [ ] Cron-like recurring tasks
- [ ] Worker health monitoring and metrics
- [ ] Authentication and authorization
- [ ] Webhook notifications
- [ ] Task cancellation support
- [ ] Persistent task history

## 🤝 Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

MIT License - feel free to use this project however you'd like!

## Acknowledgments

Built with:

- [Go 1.25.3](https://golang.org/)
- [Pogocache](https://github.com/pogocache/pogocache) - Fast caching with focus on low latency and CPU efficiency
- [go-redis](https://github.com/redis/go-redis) - Redis/Pogocache client for Go
- Go standard library (`net/http`, `encoding/json`)

### Why Pogocache?

Nexq uses Pogocache instead of traditional caching solutions because:

- **Faster**: Lower latency per request than Redis, Memcache, Valkey, and others
- **More Efficient**: Uses fewer CPU cycles, reducing server costs and energy usage
- **Better Scaling**: Optimized for both single-threaded and multi-threaded performance
- **Protocol Compatible**: Supports Redis wire protocol, making migration seamless

---

