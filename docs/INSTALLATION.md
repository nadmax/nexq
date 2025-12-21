# Installation

## Getting Started

```sh
# Clone the repository
git clone https://github.com/yourusername/nexq.git
cd nexq
```

After cloning the repository, two options are possible.

### 1. Locally

```sh
# Install dependencies
go mod download

# Build Pogocache from source
git clone https://github.com/pogocache/pogocache.git
cd pogocache
make
./pogocache

# Run the API server
go run cmd/server/main.go

# In another terminal, run a worker
go run cmd/worker/main.go
```

### 2. With Docker

```sh
# Start all services
docker compose up -d
```

Open your browser and navigate to `http://localhost:8080` to access the dashboard.  

### Basic examples

**Create a task via API:**

```sh
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

```sh
curl -X POST http://localhost:8080/api/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "type": "generate_report",
    "payload": {"report_type": "monthly"},
    "schedule_in": 3600
  }'
```
