package peer

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/GBoc09/SDCC_project/internal/api"
	"github.com/GBoc09/SDCC_project/internal/registry"
)

func TestGossiperContinuesAfterPeerFailure(t *testing.T) {
	source := registry.New("registry-1")
	target := registry.New("registry-2")

	_, _, err := source.UpsertInstance(
		"payments",
		"payment-1",
		registry.InstanceInput{
			Address: "10.0.0.1",
			Port:    8080,
		},
	)
	if err != nil {
		t.Fatalf("create source instance: %v", err)
	}

	availablePeer := httptest.NewServer(api.NewHandler(target))
	defer availablePeer.Close()

	unavailablePeer := httptest.NewServer(
		api.NewHandler(registry.New("registry-3")),
	)
	unavailablePeerURL := unavailablePeer.URL
	unavailablePeer.Close()

	gossiper := NewGossiper(
		NewClient(100*time.Millisecond),
		[]string{
			unavailablePeerURL,
			availablePeer.URL,
		},
	)

	errors := gossiper.Publish(
		context.Background(),
		source.Snapshot(),
	)

	if len(errors) != 1 {
		t.Fatalf(
			"gossip errors = %d, want 1",
			len(errors),
		)
	}

	instances := target.Discover("payments")
	if len(instances) != 1 {
		t.Fatalf(
			"target discovered %d instances, want 1",
			len(instances),
		)
	}

	if instances[0].ID != "payment-1" {
		t.Errorf(
			"instance ID = %q, want %q",
			instances[0].ID,
			"payment-1",
		)
	}
}
func TestGossiperProcessesNotifications(t *testing.T) {
	source := registry.New("registry-1")
	target := registry.New("registry-2")

	_, _, err := source.UpsertInstance(
		"payments",
		"payment-1",
		registry.InstanceInput{
			Address: "10.0.0.1",
			Port:    8080,
		},
	)
	if err != nil {
		t.Fatalf("create source instance: %v", err)
	}

	server := httptest.NewServer(api.NewHandler(target))
	defer server.Close()

	gossiper := NewGossiper(
		NewClient(time.Second),
		[]string{server.URL},
	)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		defer close(done)
		gossiper.Run(ctx)
	}()

	gossiper.Notify(source.Snapshot())

	deadline := time.After(time.Second)
	check := time.NewTicker(5 * time.Millisecond)
	defer check.Stop()

	synchronized := false

	for !synchronized {
		select {
		case <-deadline:
			t.Fatal("gossip notification was not delivered")

		case <-check.C:
			instances := target.Discover("payments")
			synchronized = len(instances) == 1
		}
	}

	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("gossiper did not stop after cancellation")
	}
}
func TestGossiperReportsPublishErrors(t *testing.T) {
	server := httptest.NewServer(
		api.NewHandler(registry.New("registry-2")),
	)
	peerURL := server.URL
	server.Close()

	gossiper := NewGossiper(
		NewClient(100*time.Millisecond),
		[]string{peerURL},
	)

	reportedErrors := make(chan error, 1)

	gossiper.SetErrorHandler(
		func(err error) {
			reportedErrors <- err
		},
	)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		defer close(done)
		gossiper.Run(ctx)
	}()

	gossiper.Notify(registry.RegistryState{})

	select {
	case err := <-reportedErrors:
		if err == nil {
			t.Fatal("reported error is nil")
		}

	case <-time.After(time.Second):
		t.Fatal("gossip error was not reported")
	}

	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("gossiper did not stop after cancellation")
	}
}
