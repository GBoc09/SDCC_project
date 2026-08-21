package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/GBoc09/SDCC_project/internal/registry"
)

func TestHTTPGetInternalState(t *testing.T) {
	store := registry.New("registry-1")

	_, _, err := store.InsertUpdateInstance(
		"payments",
		"payment-1",
		registry.InstanceInput{Address: "10.0.0.1", Port: 8080},
	)
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}

	if err := store.DeleteInstance("payments", "payment-1"); err != nil {
		t.Fatalf("delete instance: %v", err)
	}

	response := request(t, NewHandler(store), http.MethodGet, "/internal/state", "")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", response.Code, http.StatusOK, response.Body.String())
	}

	var state registry.RegistryState
	if err := json.Unmarshal(response.Body.Bytes(), &state); err != nil {
		t.Fatalf("decode state response: %v", err)
	}
	if len(state.Services) != 1 {
		t.Fatalf("received %d services, want 1", len(state.Services))
	}
	if len(state.Instances) != 1 {
		t.Fatalf("received %d instances, want 1", len(state.Instances))
	}

	instance := state.Instances[0]
	if instance.ID != "payment-1" {
		t.Errorf("instance ID = %q, want %q", instance.ID, "payment-1")
	}
	if instance.Status != registry.StatusDeleted {
		t.Errorf("instance status = %q, want %q", instance.Status, registry.StatusDeleted)
	}
}

func TestHTTPPutInternalState(t *testing.T) {
	source := registry.New("registry-1")
	target := registry.New("registry-2")

	_, _, err := source.InsertUpdateInstance(
		"payments",
		"payment-1",
		registry.InstanceInput{Address: "10.0.0.1", Port: 8080},
	)
	if err != nil {
		t.Fatalf("create source instance: %v", err)
	}

	body, err := json.Marshal(source.Snapshot())
	if err != nil {
		t.Fatalf("encode source state: %v", err)
	}

	response := request(t, NewHandler(target), http.MethodPut, "/internal/state", string(body))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", response.Code, http.StatusOK, response.Body.String())
	}

	var result struct {
		Applied int `json:"applied"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode merge response: %v", err)
	}
	if result.Applied != 2 {
		t.Fatalf("applied records = %d, want 2", result.Applied)
	}

	instances := target.Discover("payments")
	if len(instances) != 1 {
		t.Fatalf("target discovered %d instances, want 1", len(instances))
	}

	got := instances[0]
	if got.ID != "payment-1" {
		t.Errorf("instance ID = %q, want %q", got.ID, "payment-1")
	}
	if got.Address != "10.0.0.1" {
		t.Errorf("address = %q, want %q", got.Address, "10.0.0.1")
	}
	if got.Port != 8080 {
		t.Errorf("port = %d, want %d", got.Port, 8080)
	}
}

func TestHTTPPutInternalStateRejectsInvalidBody(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "invalid JSON", body: `{`},
		{name: "unknown field", body: `{"unknown":[]}`},
		{name: "multiple JSON values", body: `{} {}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := registry.New("registry-1")
			response := request(t, NewHandler(store), http.MethodPut, "/internal/state", test.body)

			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d, body = %s", response.Code, http.StatusBadRequest, response.Body.String())
			}

			state := store.Snapshot()
			if len(state.Services) != 0 {
				t.Errorf("registry contains %d services after invalid request", len(state.Services))
			}
			if len(state.Instances) != 0 {
				t.Errorf("registry contains %d instances after invalid request", len(state.Instances))
			}
		})
	}
}
