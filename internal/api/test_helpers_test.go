package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/GBoc09/SDCC_project/internal/registry"
)

func request(
	t *testing.T,
	handler http.Handler,
	method string,
	path string,
	body string,
) *httptest.ResponseRecorder {
	t.Helper()

	httpRequest := httptest.NewRequest(
		method,
		path,
		strings.NewReader(body),
	)
	if body != "" {
		httpRequest.Header.Set("Content-Type", "application/json")
	}

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httpRequest)
	return response
}

type recordingNotifier struct {
	states []registry.RegistryState
}

func (n *recordingNotifier) Notify(state registry.RegistryState) {
	n.states = append(n.states, state)
}

func createAPIInstance(t *testing.T, store *registry.Registry, id string) {
	t.Helper()
	if _, _, err := store.InsertUpdateInstance("payments", id,
		registry.InstanceInput{Address: "10.0.0.1", Port: 8080}); err != nil {
		t.Fatalf("create instance %q: %v", id, err)
	}
}
