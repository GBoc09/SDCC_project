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
