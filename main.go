package main

import (
	"errors"
	"log"
	"net/http"
	"os"

	"github.com/GBoc09/SDCC_project/internal/api"
	"github.com/GBoc09/SDCC_project/internal/registry"
)

func main() {
	nodeID := valueOrDefault("NODE_ID", "registry-1")
	address := valueOrDefault("HTTP_ADDRESS", ":8080")

	store := registry.New(nodeID)
	server := &http.Server{
		Addr:    address,
		Handler: api.NewHandler(store),
	}

	log.Printf("registry node %q listening on %s", nodeID, address)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func valueOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
