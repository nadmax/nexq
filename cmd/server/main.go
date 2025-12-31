package main

import (
	"log"
	"net/http"
	"os"

	"github.com/nadmax/nexq/internal/api"
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
			log.Printf("failed to close server queue: %v", err)
		}
	}()

	apiHandler := api.NewAPI(q)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on :%s", port)
	log.Printf("Connected to Pogocache at %s", pogocacheAddr)

	if err := http.ListenAndServe(":"+port, apiHandler); err != nil {
		log.Fatal(err)
	}
}
