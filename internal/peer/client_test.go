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

func TestClientSynchronizesFromPeer(t *testing.T) {
	source := registry.New("registry-1")

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

	server := httptest.NewServer(api.NewHandler(source))
	defer server.Close()

	target := registry.New("registry-2")
	client := NewClient(time.Second)

	applied, err := client.Sync(
		context.Background(),
		server.URL,
		target,
	)
	if err != nil {
		t.Fatalf("synchronize from peer: %v", err)
	}

	if applied != 2 {
		t.Fatalf(
			"applied records = %d, want 2",
			applied,
		)
	}

	instances := target.Discover("payments")
	if len(instances) != 1 {
		t.Fatalf(
			"target discovered %d instances, want 1",
			len(instances),
		)
	}

	got := instances[0]

	if got.ID != "payment-1" {
		t.Errorf(
			"instance ID = %q, want %q",
			got.ID,
			"payment-1",
		)
	}

	if got.Address != "10.0.0.1" {
		t.Errorf(
			"address = %q, want %q",
			got.Address,
			"10.0.0.1",
		)
	}

	if got.Port != 8080 {
		t.Errorf("port = %d, want %d", got.Port, 8080)
	}
}
func TestClientRejectsInvalidPeerResponse(t *testing.T) {
	tests := []struct {
		name    string
		handler http.Handler
	}{
		{
			name: "unexpected status",
			handler: http.HandlerFunc(
				func(w http.ResponseWriter, _ *http.Request) {
					http.Error(
						w,
						"peer unavailable",
						http.StatusServiceUnavailable,
					)
				},
			),
		},
		{
			name: "invalid JSON",
			handler: http.HandlerFunc(
				func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Set(
						"Content-Type",
						"application/json",
					)
					w.WriteHeader(http.StatusOK)

					if _, err := w.Write([]byte(`{`)); err != nil {
						t.Errorf("write response: %v", err)
					}
				},
			),
		},
		{
			name: "unknown JSON field",
			handler: http.HandlerFunc(
				func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Set(
						"Content-Type",
						"application/json",
					)
					w.WriteHeader(http.StatusOK)

					if _, err := w.Write(
						[]byte(`{"unknown":[]}`),
					); err != nil {
						t.Errorf("write response: %v", err)
					}
				},
			),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(test.handler)
			defer server.Close()

			target := registry.New("registry-1")
			client := NewClient(time.Second)

			applied, err := client.Sync(
				context.Background(),
				server.URL,
				target,
			)

			if err == nil {
				t.Fatal("expected synchronization error")
			}

			if applied != 0 {
				t.Errorf(
					"applied records = %d, want 0",
					applied,
				)
			}

			state := target.Snapshot()

			if len(state.Services) != 0 {
				t.Errorf(
					"registry contains %d services after error",
					len(state.Services),
				)
			}

			if len(state.Instances) != 0 {
				t.Errorf(
					"registry contains %d instances after error",
					len(state.Instances),
				)
			}
		})
	}
}
func TestClientHandlesUnreachablePeer(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
		),
	)

	peerURL := server.URL
	server.Close()

	target := registry.New("registry-1")
	client := NewClient(100 * time.Millisecond)

	applied, err := client.Sync(
		context.Background(),
		peerURL,
		target,
	)

	if err == nil {
		t.Fatal("expected error for unreachable peer")
	}

	if applied != 0 {
		t.Errorf(
			"applied records = %d, want 0",
			applied,
		)
	}

	state := target.Snapshot()

	if len(state.Services) != 0 ||
		len(state.Instances) != 0 {
		t.Fatalf(
			"registry was modified after connection error: %#v",
			state,
		)
	}
}
func TestClientPushesStateToPeer(t *testing.T) {
	source := registry.New("registry-1")
	target := registry.New("registry-2")

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

	server := httptest.NewServer(api.NewHandler(target))
	defer server.Close()

	client := NewClient(time.Second)

	applied, err := client.Push(
		context.Background(),
		server.URL,
		source.Snapshot(),
	)
	if err != nil {
		t.Fatalf("push state to peer: %v", err)
	}

	if applied != 2 {
		t.Fatalf(
			"applied records = %d, want 2",
			applied,
		)
	}

	instances := target.Discover("payments")
	if len(instances) != 1 {
		t.Fatalf(
			"target discovered %d instances, want 1",
			len(instances),
		)
	}

	got := instances[0]

	if got.ID != "payment-1" {
		t.Errorf(
			"instance ID = %q, want %q",
			got.ID,
			"payment-1",
		)
	}

	if got.Address != "10.0.0.1" {
		t.Errorf(
			"address = %q, want %q",
			got.Address,
			"10.0.0.1",
		)
	}

	if got.Port != 8080 {
		t.Errorf("port = %d, want %d", got.Port, 8080)
	}
}
