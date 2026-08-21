package convergence

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/GBoc09/SDCC_project/internal/registry"
)

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
