package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/nadmax/nexq/internal/queue"
	"github.com/nadmax/nexq/internal/repository/postgres"
	"github.com/nadmax/nexq/internal/worker"
	"github.com/nadmax/nexq/internal/worker/handlers"
)

func main() {
	pogocacheAddr := os.Getenv("POGOCACHE_ADDR")
	if pogocacheAddr == "" {
		pogocacheAddr = "localhost:9401"
	}

	postgresDSN := os.Getenv("POSTGRES_DSN")
	if postgresDSN == "" {
		log.Fatal("POSTGRES_DSN is required")
	}

	repo, err := postgres.NewPostgresTaskRepository(postgresDSN)
	if err != nil {
		log.Fatal(err)
	}

	defer func() {
		if repoErr := repo.Close(); repoErr != nil {
			log.Printf("failed to close Postgres repository: %v", repoErr)
		}
	}()

	q, err := queue.NewQueue(pogocacheAddr, repo)
	if err != nil {
		log.Fatal(err)
	}

	defer func() {
		if qErr := q.Close(); qErr != nil {
			log.Printf("failed to close worker queue: %v", qErr)
		}
	}()

	workerID := os.Getenv("WORKER_ID")
	if workerID == "" {
		workerID = fmt.Sprintf("worker-%d", time.Now().Unix())
	}

	w := worker.NewWorker(workerID, q)
	reportGen := handlers.NewReportGenerator(repo.DB())

	w.RegisterHandler("generate_report", reportGen.GenerateReportHandler)

	var wg sync.WaitGroup

	wg.Go(func() {
		w.Start()
	})

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down worker...")
	w.Stop()
	wg.Wait()

	log.Println("Worker stopped")
}
