package convergence

import (
	"net/http"
	"os"
	"testing"
	"time"
)

const (
	registry1URL = "http://localhost:8080"
	registry2URL = "http://localhost:8081"
	registry3URL = "http://localhost:8082"

	sampleCount        = 20
	pollingInterval    = 10 * time.Millisecond
	healthTimeout      = 5 * time.Second
	convergenceTimeout = 5 * time.Second
	recoveryTimeout    = 10 * time.Second

	// Must match PEER_TIMEOUT in docker-compose.yml.
	composePeerTimeout = 2 * time.Second
	gossipDrainMargin  = 500 * time.Millisecond
	gossipDrainDelay   = composePeerTimeout + gossipDrainMargin
)

func requireConvergenceTests(t *testing.T) {
	t.Helper()

	if os.Getenv("RUN_CONVERGENCE_TESTS") != "1" {
		t.Skip(
			"set RUN_CONVERGENCE_TESTS=1 to run convergence tests",
		)
	}
}

func newTestClient() *http.Client {
	return &http.Client{
		Timeout: time.Second,
	}
}

func waitForClusterHealth(
	t *testing.T,
	client *http.Client,
) {
	t.Helper()

	for _, baseURL := range []string{
		registry1URL,
		registry2URL,
		registry3URL,
	} {
		waitForHealth(
			t,
			client,
			baseURL,
			healthTimeout,
		)
	}
}
