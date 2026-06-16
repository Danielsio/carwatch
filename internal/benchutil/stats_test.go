package benchutil

import (
	"testing"
	"time"
)

func TestPercentiles(t *testing.T) {
	// 100 durations: 1ms, 2ms, ..., 100ms
	durations := make([]time.Duration, 100)
	for i := range 100 {
		durations[i] = time.Duration(i+1) * time.Millisecond
	}

	p50, p95, p99 := Percentiles(durations)
	if p50 != 50*time.Millisecond {
		t.Errorf("p50 = %v, want 50ms", p50)
	}
	if p95 != 95*time.Millisecond {
		t.Errorf("p95 = %v, want 95ms", p95)
	}
	if p99 != 99*time.Millisecond {
		t.Errorf("p99 = %v, want 99ms", p99)
	}
}

func TestPercentilesEmpty(t *testing.T) {
	p50, p95, p99 := Percentiles(nil)
	if p50 != 0 || p95 != 0 || p99 != 0 {
		t.Errorf("expected zeros for empty slice, got %v %v %v", p50, p95, p99)
	}
}

func TestMinMax(t *testing.T) {
	durations := []time.Duration{5 * time.Millisecond, 1 * time.Millisecond, 10 * time.Millisecond}
	min, max := MinMax(durations)
	if min != 1*time.Millisecond {
		t.Errorf("min = %v, want 1ms", min)
	}
	if max != 10*time.Millisecond {
		t.Errorf("max = %v, want 10ms", max)
	}
}

func TestThroughput(t *testing.T) {
	tp := Throughput(100, time.Second)
	if tp != 100.0 {
		t.Errorf("throughput = %v, want 100.0", tp)
	}
}
