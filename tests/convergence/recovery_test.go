package convergence

import (
	"fmt"
	"testing"
	"time"
)

func TestAntiEntropyRecoveryConvergence(t *testing.T) {
	requireConvergenceTests(t)

	if gossipDrainDelay <= composePeerTimeout {
		t.Fatalf(
			"gossip drain delay %s must exceed peer timeout %s",
			gossipDrainDelay,
			composePeerTimeout,
		)
	}

	client := newTestClient()
	waitForClusterHealth(t, client)

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
			healthTimeout,
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
			convergenceTimeout,
		)

		// Wait until the gossip attempt to the paused node times out.
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
			healthTimeout,
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

	logLatencyStatistics(
		t,
		"end-to-end anti-entropy recovery convergence",
		latencies,
	)
}
