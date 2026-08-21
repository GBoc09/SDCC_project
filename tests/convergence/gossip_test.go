package convergence

import (
	"fmt"
	"testing"
	"time"
)

func TestEndToEndGossipConvergence(t *testing.T) {
	requireConvergenceTests(t)

	client := newTestClient()
	waitForClusterHealth(t, client)

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

		for _, targetURL := range []string{
			registry2URL,
			registry3URL,
		} {
			waitForInstance(
				t,
				client,
				targetURL,
				serviceName,
				"instance-1",
				convergenceTimeout,
			)
		}

		latency := time.Since(startedAt)
		latencies = append(latencies, latency)

		t.Logf(
			"sample %02d end-to-end cluster gossip convergence: %s",
			sample+1,
			latency,
		)
	}

	logLatencyStatistics(
		t,
		"end-to-end cluster gossip convergence",
		latencies,
	)
}
