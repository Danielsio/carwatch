package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// Metric holds a single benchmark measurement with optional percentile breakdown.
type Metric struct {
	Value  float64 `json:"value"`
	Unit   string  `json:"unit"`
	P50    float64 `json:"p50,omitempty"`
	P95    float64 `json:"p95,omitempty"`
	P99    float64 `json:"p99,omitempty"`
	Min    float64 `json:"min,omitempty"`
	Max    float64 `json:"max,omitempty"`
	Count  int     `json:"count,omitempty"`
	Errors int     `json:"errors,omitempty"`
}

// PhaseResult captures one benchmark phase's output.
type PhaseResult struct {
	Phase       string            `json:"phase"`
	DurationMs  int64             `json:"duration_ms"`
	Pass        bool              `json:"pass"`
	FailReason  string            `json:"fail_reason,omitempty"`
	Metrics     map[string]Metric `json:"metrics"`
	ProfilePath string            `json:"profile_path,omitempty"`
}

// BenchReport is the top-level JSON output.
type BenchReport struct {
	Timestamp string                 `json:"timestamp"`
	GitCommit string                 `json:"git_commit"`
	Scale     map[string]int         `json:"scale"`
	Phases    []PhaseResult          `json:"phases"`
	Summary   map[string]interface{} `json:"summary"`
}

func writeJSON(path string, report *BenchReport) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func printTable(report *BenchReport) {
	line := strings.Repeat("─", 68)

	fmt.Println("════════════════════════════════════════════════════════════════════")
	fmt.Printf("  CARWATCH PERFORMANCE BENCHMARK\n")
	fmt.Printf("  Commit: %s  Scale: %du/%ds/%dl\n",
		report.GitCommit,
		report.Scale["users"],
		report.Scale["searches"],
		report.Scale["listings"])
	fmt.Println("════════════════════════════════════════════════════════════════════")
	fmt.Println()
	fmt.Printf("  %-4s %-22s %10s  %-4s  %s\n", "#", "Phase", "Duration", "Pass", "Key Metric")
	fmt.Println("  " + line)

	passed, total := 0, 0
	for i, p := range report.Phases {
		total++
		status := "FAIL"
		if p.Pass {
			status = "PASS"
			passed++
		}
		dur := formatDuration(time.Duration(p.DurationMs) * time.Millisecond)
		key := keyMetric(p)
		fmt.Printf("  %-4d %-22s %10s  %-4s  %s\n", i+1, p.Phase, dur, status, key)
	}

	fmt.Println("  " + line)
	totalDur := time.Duration(0)
	for _, p := range report.Phases {
		totalDur += time.Duration(p.DurationMs) * time.Millisecond
	}
	fmt.Printf("  TOTAL %35s  %d/%d PASS\n", formatDuration(totalDur), passed, total)
	fmt.Println()
}

func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%.1fms", float64(d.Microseconds())/1000)
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
}

func keyMetric(p PhaseResult) string {
	if m, ok := p.Metrics["per_op_us"]; ok {
		return fmt.Sprintf("p99=%.0fus", m.P99)
	}
	if m, ok := p.Metrics["per_listing_us"]; ok {
		return fmt.Sprintf("p99=%.0fus", m.P99)
	}
	if m, ok := p.Metrics["per_claim_us"]; ok {
		return fmt.Sprintf("p99=%.0fus", m.P99)
	}
	if m, ok := p.Metrics["per_query_us"]; ok {
		return fmt.Sprintf("p99=%.0fus", m.P99)
	}
	if m, ok := p.Metrics["round_trip_us"]; ok {
		return fmt.Sprintf("p99=%.0fus", m.P99)
	}
	if m, ok := p.Metrics["challenges"]; ok {
		return fmt.Sprintf("challenges=%.0f", m.Value)
	}
	if m, ok := p.Metrics["cycle_ms"]; ok {
		return fmt.Sprintf("cycle=%.0fms", m.Value)
	}
	if m, ok := p.Metrics["throughput"]; ok {
		return fmt.Sprintf("%.0f %s", m.Value, m.Unit)
	}
	return ""
}
