package main

import (
	"log"
	"net/http"
	"os"

	"github.com/nadmax/nexq/internal/api"
	"github.com/nadmax/nexq/internal/queue"
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
