package main

import (
	"errors"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nadmax/nexq/internal/queue"
	"github.com/nadmax/nexq/internal/worker"
)

func main() {
	pogocacheAddr := os.Getenv("POGOCACHE_ADDR")
	if pogocacheAddr == "" {
		pogocacheAddr = "localhost:9401"
	}

	q, err := queue.NewQueue(pogocacheAddr)
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

	w.RegisterHandler("send_email", sendEmailHandler)
	w.RegisterHandler("process_image", processImageHandler)
	w.RegisterHandler("generate_report", generateReportHandler)

	go w.Start()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down worker...")
	w.Stop()
}

func sendEmailHandler(task *queue.Task) error {
	to, ok := task.Payload["to"].(string)
	if !ok {
		return errors.New("missing 'to' field")
	}

	log.Printf("Sending email to %s...", to)
	time.Sleep(2 * time.Second)

	if rand.Float32() < 0.2 {
		return errors.New("SMTP server connection failed")
	}

	log.Printf("Email sent to %s", to)
	return nil
}

func processImageHandler(task *queue.Task) error {
	imageURL, ok := task.Payload["image_url"].(string)
	if !ok {
		return errors.New("missing 'image_url' field")
	}

	log.Printf("Processing image: %s", imageURL)
	time.Sleep(5 * time.Second)
	log.Printf("Image processed: %s", imageURL)
	return nil
}

func generateReportHandler(task *queue.Task) error {
	reportType, ok := task.Payload["report_type"].(string)
	if !ok {
		return errors.New("missing 'report_type' field")
	}

	log.Printf("Generating report: %s", reportType)
	time.Sleep(3 * time.Second)
	log.Printf("Report generated: %s", reportType)
	return nil
}
