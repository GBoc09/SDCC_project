package convergence

import (
	"sort"
	"testing"
	"time"
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

func logLatencyStatistics(
	t *testing.T,
	label string,
	latencies []time.Duration,
) {
	t.Helper()

	statistics := calculateLatencyStatistics(latencies)

	t.Logf(
		"%s: samples=%d min=%s average=%s median=%s p95=%s max=%s",
		label,
		len(latencies),
		statistics.Minimum,
		statistics.Average,
		statistics.Median,
		statistics.P95,
		statistics.Maximum,
	)
}
