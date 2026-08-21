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
