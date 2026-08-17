package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/GBoc09/SDCC_project/internal/registry"
)

func TestHTTPInstanceLifecycle(t *testing.T) {
	handler := NewHandler(registry.New("registry-1"))

	response := request(t, handler, http.MethodPut, "/services/payments/instances/payment-1",
		`{"address":"10.0.0.10","port":8080}`)
	if response.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", response.Code, response.Body.String())
	}

	response = request(t, handler, http.MethodPut, "/services/payments/instances/payment-1",
		`{"address":"10.0.0.11","port":9090}`)
	if response.Code != http.StatusOK {
		t.Fatalf("update status=%d body=%s", response.Code, response.Body.String())
	}

	response = request(t, handler, http.MethodGet, "/services/payments", "")
	if response.Code != http.StatusOK {
		t.Fatalf("discover status=%d body=%s", response.Code, response.Body.String())
	}
	var instances []registry.InstanceRecord
	if err := json.Unmarshal(response.Body.Bytes(), &instances); err != nil {
		t.Fatalf("decode discovery response: %v", err)
	}
	if len(instances) != 1 || instances[0].Port != 9090 {
		t.Fatalf("unexpected instances: %#v", instances)
	}

	response = request(t, handler, http.MethodDelete, "/services/payments/instances/payment-1", "")
	if response.Code != http.StatusNoContent {
		t.Fatalf("delete status=%d body=%s", response.Code, response.Body.String())
	}

	response = request(t, handler, http.MethodGet, "/services/payments", "")
	if response.Body.String() != "[]\n" {
		t.Fatalf("deleted instance is visible: %s", response.Body.String())
	}
}

func TestHTTPDeleteServiceAndRecreate(t *testing.T) {
	handler := NewHandler(registry.New("registry-1"))
	for _, id := range []string{"one", "two"} {
		response := request(t, handler, http.MethodPut, "/services/catalog/instances/"+id,
			`{"address":"localhost","port":8080}`)
		if response.Code != http.StatusCreated {
			t.Fatalf("create %s: status=%d body=%s", id, response.Code, response.Body.String())
		}
	}

	response := request(t, handler, http.MethodDelete, "/services/catalog", "")
	if response.Code != http.StatusNoContent {
		t.Fatalf("delete service: status=%d body=%s", response.Code, response.Body.String())
	}

	response = request(t, handler, http.MethodGet, "/services/catalog", "")
	if response.Body.String() != "[]\n" {
		t.Fatalf("deleted service is visible: %s", response.Body.String())
	}

	response = request(t, handler, http.MethodPut, "/services/catalog/instances/three",
		`{"address":"localhost","port":8081}`)
	if response.Code != http.StatusCreated {
		t.Fatalf("recreate service: status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestHTTPValidationAndMissingRecords(t *testing.T) {
	handler := NewHandler(registry.New("registry-1"))

	tests := []struct {
		name   string
		method string
		path   string
		body   string
		status int
	}{
		{"invalid JSON", http.MethodPut, "/services/api/instances/one", `{`, http.StatusBadRequest},
		{"unknown field", http.MethodPut, "/services/api/instances/one", `{"address":"localhost","port":80,"extra":true}`, http.StatusBadRequest},
		{"invalid port", http.MethodPut, "/services/api/instances/one", `{"address":"localhost","port":0}`, http.StatusBadRequest},
		{"missing instance", http.MethodDelete, "/services/api/instances/one", "", http.StatusNotFound},
		{"missing service", http.MethodDelete, "/services/api", "", http.StatusNotFound},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := request(t, handler, test.method, test.path, test.body)
			if response.Code != test.status {
				t.Fatalf("status=%d want=%d body=%s", response.Code, test.status, response.Body.String())
			}
		})
	}
}

func TestHealth(t *testing.T) {
	response := request(t, NewHandler(registry.New("registry-1")), http.MethodGet, "/health", "")
	if response.Code != http.StatusOK || response.Body.String() != "{\"status\":\"ok\"}\n" {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func request(t *testing.T, handler http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	httpRequest := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		httpRequest.Header.Set("Content-Type", "application/json")
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httpRequest)
	return response
}
func TestHTTPGetInternalState(t *testing.T) {
	store := registry.New("registry-1")

	_, _, err := store.UpsertInstance(
		"payments",
		"payment-1",
		registry.InstanceInput{
			Address: "10.0.0.1",
			Port:    8080,
		},
	)
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}

	if err := store.DeleteInstance(
		"payments",
		"payment-1",
	); err != nil {
		t.Fatalf("delete instance: %v", err)
	}

	handler := NewHandler(store)

	response := request(
		t,
		handler,
		http.MethodGet,
		"/internal/state",
		"",
	)

	if response.Code != http.StatusOK {
		t.Fatalf(
			"status = %d, want %d, body = %s",
			response.Code,
			http.StatusOK,
			response.Body.String(),
		)
	}

	var state registry.RegistryState

	if err := json.Unmarshal(response.Body.Bytes(), &state); err != nil {
		t.Fatalf("decode state response: %v", err)
	}

	if len(state.Services) != 1 {
		t.Fatalf(
			"received %d services, want 1",
			len(state.Services),
		)
	}

	if len(state.Instances) != 1 {
		t.Fatalf(
			"received %d instances, want 1",
			len(state.Instances),
		)
	}

	instance := state.Instances[0]

	if instance.ID != "payment-1" {
		t.Errorf(
			"instance ID = %q, want %q",
			instance.ID,
			"payment-1",
		)
	}

	if instance.Status != registry.StatusDeleted {
		t.Errorf(
			"instance status = %q, want %q",
			instance.Status,
			registry.StatusDeleted,
		)
	}
}

func TestHTTPPutInternalState(t *testing.T) {
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

	body, err := json.Marshal(source.Snapshot())
	if err != nil {
		t.Fatalf("encode source state: %v", err)
	}

	handler := NewHandler(target)

	response := request(
		t,
		handler,
		http.MethodPut,
		"/internal/state",
		string(body),
	)

	if response.Code != http.StatusOK {
		t.Fatalf(
			"status = %d, want %d, body = %s",
			response.Code,
			http.StatusOK,
			response.Body.String(),
		)
	}

	var result struct {
		Applied int `json:"applied"`
	}

	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode merge response: %v", err)
	}

	if result.Applied != 2 {
		t.Fatalf(
			"applied records = %d, want 2",
			result.Applied,
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
func TestHTTPPutInternalStateRejectsInvalidBody(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "invalid JSON",
			body: `{`,
		},
		{
			name: "unknown field",
			body: `{"unknown":[]}`,
		},
		{
			name: "multiple JSON values",
			body: `{} {}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := registry.New("registry-1")
			handler := NewHandler(store)

			response := request(
				t,
				handler,
				http.MethodPut,
				"/internal/state",
				test.body,
			)

			if response.Code != http.StatusBadRequest {
				t.Fatalf(
					"status = %d, want %d, body = %s",
					response.Code,
					http.StatusBadRequest,
					response.Body.String(),
				)
			}

			state := store.Snapshot()

			if len(state.Services) != 0 {
				t.Errorf(
					"registry contains %d services after invalid request",
					len(state.Services),
				)
			}

			if len(state.Instances) != 0 {
				t.Errorf(
					"registry contains %d instances after invalid request",
					len(state.Instances),
				)
			}
		})
	}
}

type recordingNotifier struct {
	states []registry.RegistryState
}

func (n *recordingNotifier) Notify(
	state registry.RegistryState,
) {
	n.states = append(n.states, state)
}
func TestHTTPUpsertNotifiesStateChange(t *testing.T) {
	store := registry.New("registry-1")
	notifier := &recordingNotifier{}

	handler := NewHandlerWithNotifier(
		store,
		notifier,
	)

	response := request(
		t,
		handler,
		http.MethodPut,
		"/services/payments/instances/payment-1",
		`{"address":"10.0.0.1","port":8080}`,
	)

	if response.Code != http.StatusCreated {
		t.Fatalf(
			"status = %d, want %d, body = %s",
			response.Code,
			http.StatusCreated,
			response.Body.String(),
		)
	}

	if len(notifier.states) != 1 {
		t.Fatalf(
			"notifications = %d, want 1",
			len(notifier.states),
		)
	}

	state := notifier.states[0]

	if len(state.Services) != 1 {
		t.Fatalf(
			"notified services = %d, want 1",
			len(state.Services),
		)
	}

	if len(state.Instances) != 1 {
		t.Fatalf(
			"notified instances = %d, want 1",
			len(state.Instances),
		)
	}

	instance := state.Instances[0]

	if instance.ID != "payment-1" {
		t.Errorf(
			"instance ID = %q, want %q",
			instance.ID,
			"payment-1",
		)
	}

	if instance.Status != registry.StatusActive {
		t.Errorf(
			"instance status = %q, want %q",
			instance.Status,
			registry.StatusActive,
		)
	}
}
func TestHTTPDeleteInstanceNotifiesStateChange(t *testing.T) {
	store := registry.New("registry-1")

	_, _, err := store.UpsertInstance(
		"payments",
		"payment-1",
		registry.InstanceInput{
			Address: "10.0.0.1",
			Port:    8080,
		},
	)
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}

	notifier := &recordingNotifier{}

	handler := NewHandlerWithNotifier(
		store,
		notifier,
	)

	response := request(
		t,
		handler,
		http.MethodDelete,
		"/services/payments/instances/payment-1",
		"",
	)

	if response.Code != http.StatusNoContent {
		t.Fatalf(
			"status = %d, want %d, body = %s",
			response.Code,
			http.StatusNoContent,
			response.Body.String(),
		)
	}

	if len(notifier.states) != 1 {
		t.Fatalf(
			"notifications = %d, want 1",
			len(notifier.states),
		)
	}

	state := notifier.states[0]

	if len(state.Instances) != 1 {
		t.Fatalf(
			"notified instances = %d, want 1",
			len(state.Instances),
		)
	}

	instance := state.Instances[0]

	if instance.ID != "payment-1" {
		t.Errorf(
			"instance ID = %q, want %q",
			instance.ID,
			"payment-1",
		)
	}

	if instance.Status != registry.StatusDeleted {
		t.Errorf(
			"instance status = %q, want %q",
			instance.Status,
			registry.StatusDeleted,
		)
	}
}
func TestHTTPDeleteServiceNotifiesStateChange(t *testing.T) {
	store := registry.New("registry-1")

	for _, instanceID := range []string{
		"payment-1",
		"payment-2",
	} {
		_, _, err := store.UpsertInstance(
			"payments",
			instanceID,
			registry.InstanceInput{
				Address: "10.0.0.1",
				Port:    8080,
			},
		)
		if err != nil {
			t.Fatalf(
				"create instance %q: %v",
				instanceID,
				err,
			)
		}
	}

	notifier := &recordingNotifier{}

	handler := NewHandlerWithNotifier(
		store,
		notifier,
	)

	response := request(
		t,
		handler,
		http.MethodDelete,
		"/services/payments",
		"",
	)

	if response.Code != http.StatusNoContent {
		t.Fatalf(
			"status = %d, want %d, body = %s",
			response.Code,
			http.StatusNoContent,
			response.Body.String(),
		)
	}

	if len(notifier.states) != 1 {
		t.Fatalf(
			"notifications = %d, want 1",
			len(notifier.states),
		)
	}

	state := notifier.states[0]

	if len(state.Services) != 1 {
		t.Fatalf(
			"notified services = %d, want 1",
			len(state.Services),
		)
	}

	service := state.Services[0]

	if service.Status != registry.StatusDeleted {
		t.Errorf(
			"service status = %q, want %q",
			service.Status,
			registry.StatusDeleted,
		)
	}

	if len(state.Instances) != 2 {
		t.Fatalf(
			"notified instances = %d, want 2",
			len(state.Instances),
		)
	}

	for _, instance := range state.Instances {
		if instance.Status != registry.StatusDeleted {
			t.Errorf(
				"instance %q status = %q, want %q",
				instance.ID,
				instance.Status,
				registry.StatusDeleted,
			)
		}
	}
}
