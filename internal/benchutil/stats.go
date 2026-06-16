package benchutil

import (
	"math"
	"sort"
	"time"
)

// Percentiles computes p50, p95, p99 from a sorted copy of durations.
// Returns zero values if the slice is empty.
func Percentiles(durations []time.Duration) (p50, p95, p99 time.Duration) {
	if len(durations) == 0 {
		return 0, 0, 0
	}
	sorted := make([]time.Duration, len(durations))
	copy(sorted, durations)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	p50 = percentile(sorted, 0.50)
	p95 = percentile(sorted, 0.95)
	p99 = percentile(sorted, 0.99)
	return
}

func percentile(sorted []time.Duration, pct float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(math.Ceil(pct*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

// MinMax returns the minimum and maximum from a duration slice.
func MinMax(durations []time.Duration) (min, max time.Duration) {
	if len(durations) == 0 {
		return 0, 0
	}
	min, max = durations[0], durations[0]
	for _, d := range durations[1:] {
		if d < min {
			min = d
		}
		if d > max {
			max = d
		}
	}
	return
}

// Throughput returns operations per second given count and total duration.
func Throughput(count int, total time.Duration) float64 {
	if total <= 0 {
		return 0
	}
	return float64(count) / total.Seconds()
}
