package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nadmax/nexq/internal/api"
	"github.com/nadmax/nexq/internal/middleware"
	"github.com/nadmax/nexq/internal/queue"
	"github.com/nadmax/nexq/internal/repository"
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
			log.Printf("failed to close server queue: %v", qErr)
		}
	}()

	go startMetricsCollector(q)

	apiHandler := api.NewAPI(q)
	handler := middleware.MetricsMiddleware(apiHandler)
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	server := &http.Server{
		Addr:    ":" + port,
		Handler: handler,
	}

	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("Server starting on :%s", port)
		log.Printf("Connected to Pogocache at %s", pogocacheAddr)
		log.Printf("Metrics available at http://localhost:%s/metrics", port)

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	<-shutdownChan
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	log.Println("Server stopped")
}
