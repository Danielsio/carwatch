package api

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/dsionov/carwatch/internal/telemetry"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const (
	vitalsMaxBody  = 128 << 10 // 128 KB
	vitalsRingSize = 200
)

type vitalEntry struct {
	Name           string  `json:"name"`
	Value          float64 `json:"value"`
	Rating         string  `json:"rating"`
	Delta          float64 `json:"delta"`
	ID             string  `json:"id"`
	NavigationType string  `json:"navigation_type"`
	ReceivedAt     string  `json:"received_at"`
}

type vitalsSummary struct {
	Name  string  `json:"name"`
	P50   float64 `json:"p50"`
	P75   float64 `json:"p75"`
	P95   float64 `json:"p95"`
	Count int     `json:"count"`
	Good  int     `json:"good"`
	Fair  int     `json:"needs_improvement"`
	Poor  int     `json:"poor"`
}

type vitalsRing struct {
	mu      sync.Mutex
	entries []vitalEntry
	pos     int
	full    bool
}

func newVitalsRing() *vitalsRing {
	return &vitalsRing{entries: make([]vitalEntry, vitalsRingSize)}
}

func (r *vitalsRing) push(e vitalEntry) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries[r.pos] = e
	r.pos = (r.pos + 1) % vitalsRingSize
	if r.pos == 0 {
		r.full = true
	}
}

func (r *vitalsRing) snapshot() []vitalEntry {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := r.pos
	if r.full {
		n = vitalsRingSize
	}
	out := make([]vitalEntry, n)
	if r.full {
		copy(out, r.entries[r.pos:])
		copy(out[vitalsRingSize-r.pos:], r.entries[:r.pos])
	} else {
		copy(out, r.entries[:n])
	}
	return out
}

func (r *vitalsRing) summarize() []vitalsSummary {
	all := r.snapshot()
	byName := make(map[string][]vitalEntry)
	for _, e := range all {
		byName[e.Name] = append(byName[e.Name], e)
	}

	var out []vitalsSummary
	for name, entries := range byName {
		s := vitalsSummary{Name: name, Count: len(entries)}
		vals := make([]float64, len(entries))
		for i, e := range entries {
			vals[i] = e.Value
			switch e.Rating {
			case "good":
				s.Good++
			case "needs-improvement":
				s.Fair++
			case "poor":
				s.Poor++
			}
		}
		sortFloats(vals)
		s.P50 = percentile(vals, 0.50)
		s.P75 = percentile(vals, 0.75)
		s.P95 = percentile(vals, 0.95)
		out = append(out, s)
	}
	return out
}

func sortFloats(a []float64) {
	for i := 1; i < len(a); i++ {
		for j := i; j > 0 && a[j] < a[j-1]; j-- {
			a[j], a[j-1] = a[j-1], a[j]
		}
	}
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(float64(len(sorted)-1) * p)
	return sorted[idx]
}

func (s *Server) receiveVitals(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, vitalsMaxBody)

	var v struct {
		Name           string  `json:"name"`
		Value          float64 `json:"value"`
		Rating         string  `json:"rating"`
		Delta          float64 `json:"delta"`
		ID             string  `json:"id"`
		NavigationType string  `json:"navigationType"`
	}
	if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	switch v.Name {
	case "CLS", "FCP", "INP", "LCP", "TTFB":
	default:
		w.WriteHeader(http.StatusNoContent)
		return
	}

	s.vitals.push(vitalEntry{
		Name:           v.Name,
		Value:          v.Value,
		Rating:         v.Rating,
		Delta:          v.Delta,
		ID:             v.ID,
		NavigationType: v.NavigationType,
		ReceivedAt:     time.Now().UTC().Format(time.RFC3339),
	})

	recordVitalMetric(r.Context(), v.Name, v.Value, v.Rating)

	w.WriteHeader(http.StatusNoContent)
}

func recordVitalMetric(ctx context.Context, name string, value float64, rating string) {
	attrs := metric.WithAttributes(attribute.String("rating", rating))
	var h metric.Float64Histogram
	switch name {
	case "CLS":
		h = telemetry.VitalsCLS
	case "FCP":
		h = telemetry.VitalsFCP
	case "INP":
		h = telemetry.VitalsINP
	case "LCP":
		h = telemetry.VitalsLCP
	case "TTFB":
		h = telemetry.VitalsTTFB
	}
	if h != nil {
		h.Record(ctx, value, attrs)
	}
}
