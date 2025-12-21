# Configuration

## Task Handlers

Add your own task handlers in `cmd/worker/main.go`:

```go
w.RegisterHandler("my_custom_task", func(task *queue.Task) error {
    // Your task logic here
    return nil
})
```

## Pogocache

Edit Pogocache connection in your code:

```go
q, err := queue.NewQueue("localhost:9401")
```

Adjust task retry settings in `internal/queue/task.go`:

```go
MaxRetries: 3  // Number of retry attempts
```

### Pogocache options

You can run Pogocache with custom options:

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
