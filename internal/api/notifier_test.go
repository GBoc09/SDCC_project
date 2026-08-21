package api

import (
	"net/http"
	"testing"

	"github.com/GBoc09/SDCC_project/internal/registry"
)

func TestHTTPInsertUpdateNotifiesStateChange(t *testing.T) {
	store := registry.New("registry-1")
	notifier := &recordingNotifier{}
	handler := NewHandlerWithNotifier(store, notifier)

	response := request(
		t,
		handler,
		http.MethodPut,
		"/services/payments/instances/payment-1",
		`{"address":"10.0.0.1","port":8080}`,
	)
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d, body = %s", response.Code, http.StatusCreated, response.Body.String())
	}
	if len(notifier.states) != 1 {
		t.Fatalf("notifications = %d, want 1", len(notifier.states))
	}

	state := notifier.states[0]
	if len(state.Services) != 1 {
		t.Fatalf("notified services = %d, want 1", len(state.Services))
	}
	if len(state.Instances) != 1 {
		t.Fatalf("notified instances = %d, want 1", len(state.Instances))
	}

	instance := state.Instances[0]
	if instance.ID != "payment-1" {
		t.Errorf("instance ID = %q, want %q", instance.ID, "payment-1")
	}
	if instance.Status != registry.StatusActive {
		t.Errorf("instance status = %q, want %q", instance.Status, registry.StatusActive)
	}
}

func TestHTTPDeleteInstanceNotifiesStateChange(t *testing.T) {
	store := registry.New("registry-1")
	_, _, err := store.InsertUpdateInstance(
		"payments",
		"payment-1",
		registry.InstanceInput{Address: "10.0.0.1", Port: 8080},
	)
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}

	notifier := &recordingNotifier{}
	response := request(
		t,
		NewHandlerWithNotifier(store, notifier),
		http.MethodDelete,
		"/services/payments/instances/payment-1",
		"",
	)
	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d, body = %s", response.Code, http.StatusNoContent, response.Body.String())
	}
	if len(notifier.states) != 1 {
		t.Fatalf("notifications = %d, want 1", len(notifier.states))
	}

	state := notifier.states[0]
	if len(state.Instances) != 1 {
		t.Fatalf("notified instances = %d, want 1", len(state.Instances))
	}

	instance := state.Instances[0]
	if instance.ID != "payment-1" {
		t.Errorf("instance ID = %q, want %q", instance.ID, "payment-1")
	}
	if instance.Status != registry.StatusDeleted {
		t.Errorf("instance status = %q, want %q", instance.Status, registry.StatusDeleted)
	}
}

func TestHTTPDeleteServiceNotifiesStateChange(t *testing.T) {
	store := registry.New("registry-1")

	for _, instanceID := range []string{"payment-1", "payment-2"} {
		_, _, err := store.InsertUpdateInstance(
			"payments",
			instanceID,
			registry.InstanceInput{Address: "10.0.0.1", Port: 8080},
		)
		if err != nil {
			t.Fatalf("create instance %q: %v", instanceID, err)
		}
	}

	notifier := &recordingNotifier{}
	response := request(
		t,
		NewHandlerWithNotifier(store, notifier),
		http.MethodDelete,
		"/services/payments",
		"",
	)
	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d, body = %s", response.Code, http.StatusNoContent, response.Body.String())
	}
	if len(notifier.states) != 1 {
		t.Fatalf("notifications = %d, want 1", len(notifier.states))
	}

	state := notifier.states[0]
	if len(state.Services) != 1 {
		t.Fatalf("notified services = %d, want 1", len(state.Services))
	}
	if state.Services[0].Status != registry.StatusDeleted {
		t.Errorf("service status = %q, want %q", state.Services[0].Status, registry.StatusDeleted)
	}
	if len(state.Instances) != 2 {
		t.Fatalf("notified instances = %d, want 2", len(state.Instances))
	}

	for _, instance := range state.Instances {
		if instance.Status != registry.StatusDeleted {
			t.Errorf("instance %q status = %q, want %q", instance.ID, instance.Status, registry.StatusDeleted)
		}
	}
}
