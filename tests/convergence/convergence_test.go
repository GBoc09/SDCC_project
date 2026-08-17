package convergence

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/GBoc09/SDCC_project/internal/registry"
)

const (
	registry1URL = "http://localhost:8080"
	registry2URL = "http://localhost:8081"
	registry3URL = "http://localhost:8082"

	pollingInterval = 10 * time.Millisecond

	// Must match PEER_TIMEOUT in docker-compose.yml.
	composePeerTimeout = 2 * time.Second
	gossipDrainMargin  = 500 * time.Millisecond
	gossipDrainDelay   = composePeerTimeout + gossipDrainMargin
)

type latencyStatistics struct {
	Minimum time.Duration
	Average time.Duration
	Median  time.Duration
	P95     time.Duration
	Maximum time.Duration
}

func calculateLatencyStatistics(
	latencies []time.Duration,
) latencyStatistics {
	if len(latencies) == 0 {
		return latencyStatistics{}
	}

	sorted := append(
		[]time.Duration(nil),
		latencies...,
	)

	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})

	var total time.Duration

	for _, latency := range sorted {
		total += latency
	}

	count := len(sorted)

	median := sorted[count/2]

	if count%2 == 0 {
		median = (sorted[count/2-1] +
			sorted[count/2]) / 2
	}

	p95Index := (95*count+99)/100 - 1

	return latencyStatistics{
		Minimum: sorted[0],
		Average: total / time.Duration(count),
		Median:  median,
		P95:     sorted[p95Index],
		Maximum: sorted[count-1],
	}
}
func TestEndToEndGossipConvergence(t *testing.T) {
	if os.Getenv("RUN_CONVERGENCE_TESTS") != "1" {
		t.Skip(
			"set RUN_CONVERGENCE_TESTS=1 to run convergence tests",
		)
	}

	client := &http.Client{
		Timeout: time.Second,
	}

	checkHealth(t, client, registry1URL)
	checkHealth(t, client, registry2URL)
	checkHealth(t, client, registry3URL)

	const sampleCount = 20

	latencies := make(
		[]time.Duration,
		0,
		sampleCount,
	)

	for sample := 0; sample < sampleCount; sample++ {
		serviceName := fmt.Sprintf(
			"convergence-%d-%d",
			time.Now().UnixNano(),
			sample,
		)

		startedAt := time.Now()

		registerInstance(
			t,
			client,
			registry1URL,
			serviceName,
			"instance-1",
		)

		waitForInstance(
			t,
			client,
			registry2URL,
			serviceName,
			"instance-1",
			5*time.Second,
		)
		waitForInstance(
			t,
			client,
			registry3URL,
			serviceName,
			"instance-1",
			5*time.Second,
		)

		latency := time.Since(startedAt)
		latencies = append(latencies, latency)

		t.Logf(
			"sample %02d cluster convergence: %s",
			sample+1,
			latency,
		)
	}

	if len(latencies) != sampleCount {
		t.Fatalf(
			"collected %d samples, want %d",
			len(latencies),
			sampleCount,
		)
	}
	statistics := calculateLatencyStatistics(latencies)

	t.Logf(
		"end-to-end cluster gossip convergence: samples=%d min=%s average=%s median=%s p95=%s max=%s",
		len(latencies),
		statistics.Minimum,
		statistics.Average,
		statistics.Median,
		statistics.P95,
		statistics.Maximum,
	)
}

func checkHealth(
	t *testing.T,
	client *http.Client,
	baseURL string,
) {
	t.Helper()

	response, err := client.Get(baseURL + "/health")
	if err != nil {
		t.Fatalf(
			"health check for %s: %v",
			baseURL,
			err,
		)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf(
			"health check for %s returned status %d",
			baseURL,
			response.StatusCode,
		)
	}
}
func registerInstance(
	t *testing.T,
	client *http.Client,
	baseURL string,
	serviceName string,
	instanceID string,
) {
	t.Helper()

	url := fmt.Sprintf(
		"%s/services/%s/instances/%s",
		baseURL,
		serviceName,
		instanceID,
	)

	body := strings.NewReader(
		`{"address":"convergence-service","port":9000}`,
	)

	request, err := http.NewRequest(
		http.MethodPut,
		url,
		body,
	)
	if err != nil {
		t.Fatalf("create registration request: %v", err)
	}

	request.Header.Set(
		"Content-Type",
		"application/json",
	)

	response, err := client.Do(request)
	if err != nil {
		t.Fatalf(
			"register instance on %s: %v",
			baseURL,
			err,
		)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusCreated {
		t.Fatalf(
			"registration returned status %d, want %d",
			response.StatusCode,
			http.StatusCreated,
		)
	}
}
func instanceExists(
	client *http.Client,
	baseURL string,
	serviceName string,
	instanceID string,
) (bool, error) {
	url := fmt.Sprintf(
		"%s/services/%s",
		baseURL,
		serviceName,
	)

	response, err := client.Get(url)
	if err != nil {
		return false, fmt.Errorf(
			"discovery request: %w",
			err,
		)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return false, fmt.Errorf(
			"discovery returned status %d",
			response.StatusCode,
		)
	}

	var instances []registry.InstanceRecord

	if err := json.NewDecoder(response.Body).Decode(
		&instances,
	); err != nil {
		return false, fmt.Errorf(
			"decode discovery response: %w",
			err,
		)
	}

	for _, instance := range instances {
		if instance.ID == instanceID {
			return true, nil
		}
	}

	return false, nil
}

func waitForInstance(
	t *testing.T,
	client *http.Client,
	baseURL string,
	serviceName string,
	instanceID string,
	timeout time.Duration,
) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	var lastError error

	for {
		found, err := instanceExists(
			client,
			baseURL,
			serviceName,
			instanceID,
		)

		if err != nil {
			lastError = err
		} else {
			lastError = nil

			if found {
				return
			}
		}

		if time.Now().After(deadline) {
			if lastError != nil {
				t.Fatalf(
					"instance %q did not converge to %s within %s; last error: %v",
					instanceID,
					baseURL,
					timeout,
					lastError,
				)
			}

			t.Fatalf(
				"instance %q did not converge to %s within %s",
				instanceID,
				baseURL,
				timeout,
			)
		}

		time.Sleep(pollingInterval)
	}
}
func composeFilePath(t *testing.T) string {
	t.Helper()

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine convergence test path")
	}

	return filepath.Clean(
		filepath.Join(
			filepath.Dir(currentFile),
			"..",
			"..",
			"docker-compose.yml",
		),
	)
}

func runDockerCompose(
	t *testing.T,
	arguments ...string,
) {
	t.Helper()

	commandArguments := []string{
		"compose",
		"-f",
		composeFilePath(t),
	}

	commandArguments = append(
		commandArguments,
		arguments...,
	)

	command := exec.Command(
		"docker",
		commandArguments...,
	)

	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf(
			"docker compose %v failed: %v\n%s",
			arguments,
			err,
			string(output),
		)
	}
}
func waitForHealth(
	t *testing.T,
	client *http.Client,
	baseURL string,
	timeout time.Duration,
) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	var lastError error

	for {
		available, err := healthAvailable(
			client,
			baseURL,
		)

		if err != nil {
			lastError = err
		} else {
			lastError = nil

			if available {
				return
			}
		}

		if time.Now().After(deadline) {
			t.Fatalf(
				"%s did not become healthy within %s; last error: %v",
				baseURL,
				timeout,
				lastError,
			)
		}

		time.Sleep(pollingInterval)
	}
}

func healthAvailable(
	client *http.Client,
	baseURL string,
) (bool, error) {
	response, err := client.Get(
		baseURL + "/health",
	)
	if err != nil {
		return false, fmt.Errorf(
			"health request: %w",
			err,
		)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return false, fmt.Errorf(
			"health returned status %d",
			response.StatusCode,
		)
	}

	return true, nil
}

func TestAntiEntropyRecoveryConvergence(t *testing.T) {
	if os.Getenv("RUN_CONVERGENCE_TESTS") != "1" {
		t.Skip(
			"set RUN_CONVERGENCE_TESTS=1 to run convergence tests",
		)
	}

	if gossipDrainDelay <= composePeerTimeout {
		t.Fatalf(
			"gossip drain delay %s must exceed peer timeout %s",
			gossipDrainDelay,
			composePeerTimeout,
		)
	}

	const (
		sampleCount     = 20
		recoveryTimeout = 10 * time.Second
	)

	client := &http.Client{
		Timeout: time.Second,
	}

	waitForHealth(
		t,
		client,
		registry1URL,
		5*time.Second,
	)
	waitForHealth(
		t,
		client,
		registry2URL,
		5*time.Second,
	)
	waitForHealth(
		t,
		client,
		registry3URL,
		5*time.Second,
	)

	latencies := make(
		[]time.Duration,
		0,
		sampleCount,
	)

	registry3Paused := false

	t.Cleanup(func() {
		if registry3Paused {
			runDockerCompose(
				t,
				"unpause",
				"registry-3",
			)
		}
	})

	for sample := 0; sample < sampleCount; sample++ {
		waitForHealth(
			t,
			client,
			registry3URL,
			5*time.Second,
		)

		runDockerCompose(
			t,
			"pause",
			"registry-3",
		)
		registry3Paused = true

		serviceName := fmt.Sprintf(
			"recovery-%d-%d",
			time.Now().UnixNano(),
			sample,
		)

		registerInstance(
			t,
			client,
			registry1URL,
			serviceName,
			"instance-1",
		)

		waitForInstance(
			t,
			client,
			registry2URL,
			serviceName,
			"instance-1",
			5*time.Second,
		)

		// Aspetta che il tentativo gossip verso il nodo
		// in pausa superi il timeout e venga abbandonato.
		time.Sleep(gossipDrainDelay)

		startedAt := time.Now()

		runDockerCompose(
			t,
			"unpause",
			"registry-3",
		)
		registry3Paused = false

		waitForHealth(
			t,
			client,
			registry3URL,
			5*time.Second,
		)

		waitForInstance(
			t,
			client,
			registry3URL,
			serviceName,
			"instance-1",
			recoveryTimeout,
		)

		latency := time.Since(startedAt)
		latencies = append(latencies, latency)

		t.Logf(
			"sample %02d end-to-end anti-entropy recovery convergence: %s",
			sample+1,
			latency,
		)
	}

	statistics := calculateLatencyStatistics(latencies)

	t.Logf(
		"end-to-end anti-entropy recovery convergence: samples=%d min=%s average=%s median=%s p95=%s max=%s",
		len(latencies),
		statistics.Minimum,
		statistics.Average,
		statistics.Median,
		statistics.P95,
		statistics.Maximum,
	)
}
