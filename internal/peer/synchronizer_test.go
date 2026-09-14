package peer

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/GBoc09/SDCC_project/internal/api"
	"github.com/GBoc09/SDCC_project/internal/registry"
)

func TestSynchronizerContinuesAfterPeerFailure(t *testing.T) {
	source := registry.New("registry-2")

	_, _, err := source.InsertUpdateInstance(
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

	availablePeer := httptest.NewServer(api.NewHandler(source))
	defer availablePeer.Close()

	unavailablePeer := httptest.NewServer(api.NewHandler(
		registry.New("registry-3"),
	))
	unavailablePeerURL := unavailablePeer.URL
	unavailablePeer.Close()

	target := registry.New("registry-1")
	client := NewClient(100 * time.Millisecond)

	synchronizer := NewSynchronizer(
		client,
		target,
		[]string{
			unavailablePeerURL,
			availablePeer.URL,
		},
		time.Second,
	)

	errors := synchronizer.SyncOnce(context.Background())

	if len(errors) != 1 {
		t.Fatalf(
			"synchronization errors = %d, want 1",
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
func TestSynchronizerRunSynchronizesPeriodically(t *testing.T) {
	source := registry.New("registry-2")
	target := registry.New("registry-1")

	sourceHandler := api.NewHandler(source)
	requests := make(chan struct{}, 10)

	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				sourceHandler.ServeHTTP(w, r)

				if r.Method == http.MethodGet &&
					r.URL.Path == "/internal/state" {
					select {
					case requests <- struct{}{}:
					default:
					}
				}
			},
		),
	)
	defer server.Close()

	synchronizer := NewSynchronizer(
		NewClient(time.Second),
		target,
		[]string{server.URL},
		10*time.Millisecond,
	)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		defer close(done)
		synchronizer.Run(ctx)
	}()

	select {
	case <-requests:
	case <-time.After(time.Second):
		t.Fatal("initial synchronization did not run")
	}

	_, _, err := source.InsertUpdateInstance(
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

	deadline := time.After(time.Second)
	check := time.NewTicker(5 * time.Millisecond)
	defer check.Stop()

	synchronized := false

	for !synchronized {
		select {
		case <-deadline:
			t.Fatal("periodic synchronization did not update target")

		case <-check.C:
			instances := target.Discover("payments")
			synchronized = len(instances) == 1
		}
	}

	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("synchronizer did not stop after cancellation")
	}
}
