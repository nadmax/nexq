![CI](https://github.com/nadmax/nexq/actions/workflows/ci.yml/badge.svg) [![codecov](https://codecov.io/gh/nadmax/nexq/graph/badge.svg)](https://codecov.io/gh/nadmax/nexq)

# Nexq

A lightweight, high-performance distributed task queue system built in Go with Pogocache.  
Schedule, execute, and monitor background jobs across multiple worker nodes with blazing-fast speed and minimal resource usage.

## Why Pogocache?

Nexq uses Pogocache instead of traditional caching solutions because:

- **Faster**: Lower latency per request than Redis, Memcache, Valkey, and others
- **More Efficient**: Uses fewer CPU cycles, reducing server costs and energy usage
- **Better Scaling**: Optimized for both single-threaded and multi-threaded performance
- **Protocol Compatible**: Supports Redis wire protocol, making migration seamless

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

## Quick Start

### Prerequisites

- Go 1.25.3 or higher
- Docker (recommended)
- Pogocache server running locally or remotely

### Installation

See [INSTALLATION.md](https://github.com/nadmax/nexq/blob/master/docs/INSTALLATION.md) to know how to use Nexq.

## Architecture

See [ARCHITECTURE.md](https://github.com/nadmax/nexq/blob/master/docs/ARCHITECTURE.md) to learn about Nexq's architecture.

## Use Cases

- Background email sending
- Image/video processing pipelines
- Report task performance generator
- Data synchronization tasks
- Scheduled maintenance jobs
- Webhook delivery with retries
- Batch processing operations

## Configuration

See [CONFIGURATION.md](https://github.com/nadmax/nexq/blob/master/docs/CONFIGURATION.md) to know how to configure your own tasks and Pogocache.

## API Endpoints

See [ENDPOINTS.md](https://github.com/nadmax/nexq/blob/master/docs/ENDPOINTS.md) to know API endpoints.

## Future Enhancements

- [x] Dead letter queue for permanently failed tasks
- [x] Persistent task history
- [ ] Task dependencies and workflows
- [ ] Cron-like recurring tasks
- [ ] Worker health monitoring and metrics
- [ ] Authentication and authorization
- [ ] Webhook notifications
- [ ] Task cancellation support

## Contributing

Contributions are welcome!  
Please refer to [CONTRIBUTING.md](https://github.com/nadmax/nexq/blob/master/CONTRIBUTING.md).

## License

This project is under [MIT License](https://github.com/nadmax/nexq/blob/master/LICENSE).  
Feel free to use this project however you'd like!

## Resources

- [Go 1.25.3](https://golang.org/)
- [Pogocache](https://github.com/pogocache/pogocache) - Fast caching with focus on low latency and CPU efficiency
- [go-redis](https://github.com/redis/go-redis) - Redis/Pogocache client for Go
- [sendgrid-go](https://github.com/sendgrid/sendgrid-go) - SendGrid Golang API Library
- [pq](https://github.com/lib/pq) - Go PostgreSQL driver for `database/sql`
- [Prometheus](https://github.com/prometheus/client_golang) - Prometheus instrumentation library for Go
- Go standard library (`net/http`, `encoding/json`)
