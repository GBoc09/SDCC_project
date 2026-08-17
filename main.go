package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/GBoc09/SDCC_project/internal/api"
	"github.com/GBoc09/SDCC_project/internal/peer"
	"github.com/GBoc09/SDCC_project/internal/persistence"
	"github.com/GBoc09/SDCC_project/internal/registry"
)

func main() {
	nodeID := valueOrDefault("NODE_ID", "registry-1")
	address := valueOrDefault("HTTP_ADDRESS", ":8080")
	peers := parsePeers(os.Getenv("PEERS"))
	stateFile := strings.TrimSpace(os.Getenv("STATE_FILE"))

	peerTimeout, err := parseDuration(
		os.Getenv("PEER_TIMEOUT"),
		2*time.Second,
	)
	if err != nil {
		log.Fatalf("invalid PEER_TIMEOUT: %v", err)
	}

	syncInterval, err := parseDuration(
		os.Getenv("SYNC_INTERVAL"),
		5*time.Second,
	)
	if err != nil {
		log.Fatalf("invalid SYNC_INTERVAL: %v", err)
	}

	persistInterval := time.Second

	if stateFile != "" {
		persistInterval, err = parseDuration(
			os.Getenv("PERSIST_INTERVAL"),
			time.Second,
		)
		if err != nil {
			log.Fatalf("invalid PERSIST_INTERVAL: %v", err)
		}
	}

	store := registry.New(nodeID)

	var fileStore *persistence.FileStore

	if stateFile != "" {
		fileStore = persistence.NewFileStore(stateFile)

		state, err := fileStore.Load()
		if err != nil {
			log.Fatalf("load persisted state: %v", err)
		}

		applied := store.MergeState(state)

		log.Printf(
			"persistence enabled: file=%s interval=%s loaded=%d",
			stateFile,
			persistInterval,
			applied,
		)

		defer func() {
			if err := fileStore.Save(
				store.Snapshot(),
			); err != nil {
				log.Printf(
					"final persistence error: %v",
					err,
				)
			}
		}()
	}

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	handler := api.NewHandler(store)

	if fileStore != nil {
		persister := persistence.NewPersister(
			fileStore,
			store,
			persistInterval,
		)

		persister.SetErrorHandler(
			func(err error) {
				log.Printf(
					"persistence error: %v",
					err,
				)
			},
		)

		go persister.Run(ctx)
	}

	if len(peers) > 0 {
		client := peer.NewClient(peerTimeout)

		synchronizer := peer.NewSynchronizer(
			client,
			store,
			peers,
			syncInterval,
		)

		gossiper := peer.NewGossiper(
			client,
			peers,
		)

		synchronizer.SetErrorHandler(
			func(err error) {
				log.Printf(
					"anti-entropy error: %v",
					err,
				)
			},
		)

		gossiper.SetErrorHandler(
			func(err error) {
				log.Printf(
					"gossip error: %v",
					err,
				)
			},
		)

		handler = api.NewHandlerWithNotifier(
			store,
			gossiper,
		)

		go synchronizer.Run(ctx)
		go gossiper.Run(ctx)

		log.Printf(
			"replication enabled: peers=%v interval=%s timeout=%s",
			peers,
			syncInterval,
			peerTimeout,
		)
	}

	server := &http.Server{
		Addr:    address,
		Handler: handler,
	}

	go shutdownServer(ctx, server)

	log.Printf(
		"registry node %q listening on %s",
		nodeID,
		address,
	)

	if err := server.ListenAndServe(); err != nil &&
		!errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func shutdownServer(
	ctx context.Context,
	server *http.Server,
) {
	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server shutdown failed: %v", err)
	}
}

func valueOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}

	return fallback
}

func parsePeers(value string) []string {
	var peers []string

	for _, entry := range strings.Split(value, ",") {
		peerURL := strings.TrimSpace(entry)

		if peerURL == "" {
			continue
		}

		peers = append(peers, peerURL)
	}

	return peers
}

func parseDuration(
	value string,
	fallback time.Duration,
) (time.Duration, error) {
	value = strings.TrimSpace(value)

	if value == "" {
		return fallback, nil
	}

	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf(
			"invalid duration %q: %w",
			value,
			err,
		)
	}

	if duration <= 0 {
		return 0, errors.New("duration must be positive")
	}

	return duration, nil
}
