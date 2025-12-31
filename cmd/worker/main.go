package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nadmax/nexq/internal/queue"
	"github.com/nadmax/nexq/internal/repository"
	"github.com/nadmax/nexq/internal/task"
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

	repo, err := repository.NewPostgresTaskRepository(postgresDSN)
	if err != nil {
		log.Fatal(err)
	}

	defer func() {
		if err := repo.Close(); err != nil {
			log.Printf("failed to close Postgres repository: %v", err)
		}
	}()

	q, err := queue.NewQueue(pogocacheAddr, repo)
	if err != nil {
		log.Fatal(err)
	}

	defer func() {
		if err := q.Close(); err != nil {
			log.Printf("failed to close worker queue: %v", err)
		}
	}()

	workerID := os.Getenv("WORKER_ID")
	if workerID == "" {
		workerID = fmt.Sprintf("worker-%d", time.Now().Unix())
	}

	w := worker.NewWorker(workerID, q)

	w.RegisterHandler("send_email", handlers.SendEmailHandler)
	w.RegisterHandler("process_image", processImageHandler)
	w.RegisterHandler("generate_report", generateReportHandler)

	go w.Start()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down worker...")
	w.Stop()
}

func processImageHandler(t *task.Task) error {
	imageURL, ok := t.Payload["image_url"].(string)
	if !ok {
		return errors.New("missing 'image_url' field")
	}

	log.Printf("Processing image: %s", imageURL)
	time.Sleep(5 * time.Second)
	log.Printf("Image processed: %s", imageURL)
	return nil
}

func generateReportHandler(t *task.Task) error {
	reportType, ok := t.Payload["report_type"].(string)
	if !ok {
		return errors.New("missing 'report_type' field")
	}

	log.Printf("Generating report: %s", reportType)
	time.Sleep(3 * time.Second)
	log.Printf("Report generated: %s", reportType)
	return nil
}
