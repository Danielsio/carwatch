package yad2

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dsionov/carwatch/internal/model"
)

func newTestEnricher(t *testing.T, handler http.Handler, cfg EnricherConfig) *Enricher {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	client, err := newPlainClient([]string{"test-ua"}, "")
	if err != nil {
		t.Fatalf("create plain client: %v", err)
	}

	ic, err := newPlainClient([]string{"test-ua"}, "")
	if err != nil {
		t.Fatalf("create plain client (itemClient): %v", err)
	}

	fetcher := &Yad2Fetcher{
		client:     client,
		itemClient: ic,
		baseURL:    srv.URL,
		logger:     slog.Default(),
		userAgents: []string{"test-ua"},
	}

	return NewEnricher(fetcher, slog.Default(), cfg)
}

func itemPageHandler(km int) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintf(w, `<html><script id="__NEXT_DATA__" type="application/json">
{"props":{"pageProps":{"itemData":{"km":%d,"address":{"city":{"text":"תל אביב","textEng":"tel_aviv"},"area":{"text":"מרכז","textEng":"center"}}}}}}
</script></html>`, km)
	}
}

func TestEnricher_HighLimitEnrichesAll(t *testing.T) {
	var requestCount atomic.Int32
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		_, _ = fmt.Fprintf(w, `<html><script id="__NEXT_DATA__" type="application/json">
{"props":{"pageProps":{"itemData":{"km":42000}}}}
</script></html>`)
	})
	enricher := newTestEnricher(t, handler, EnricherConfig{Delay: time.Millisecond, MaxPerCycle: 100})

	listings := []model.RawListing{
		{Token: "a", Km: 0}, {Token: "b", Km: 0}, {Token: "c", Km: 0},
		{Token: "d", Km: 0}, {Token: "e", Km: 0},
	}
	count := enricher.Enrich(context.Background(), listings)
	if count != 5 {
		t.Errorf("enriched = %d, want 5", count)
	}
	if got := requestCount.Load(); got != 5 {
		t.Errorf("requests = %d, want 5", got)
	}
	for i, l := range listings {
		if l.Km != 42000 {
			t.Errorf("listing[%d].Km = %d, want 42000", i, l.Km)
		}
	}
}

func TestEnricher_DefaultMaxPerCycle(t *testing.T) {
	var requestCount atomic.Int32
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		_, _ = fmt.Fprintf(w, `<html><script id="__NEXT_DATA__" type="application/json">
{"props":{"pageProps":{"itemData":{"km":42000}}}}
</script></html>`)
	})
	enricher := newTestEnricher(t, handler, EnricherConfig{Delay: time.Millisecond})

	listings := make([]model.RawListing, defaultMaxPerCycle+5)
	for i := range listings {
		listings[i] = model.RawListing{Token: fmt.Sprintf("tok-%d", i), Km: 0}
	}
	count := enricher.Enrich(context.Background(), listings)
	if count != defaultMaxPerCycle {
		t.Errorf("enriched = %d, want %d (defaultMaxPerCycle)", count, defaultMaxPerCycle)
	}
	if got := requestCount.Load(); got != int32(defaultMaxPerCycle) {
		t.Errorf("requests = %d, want %d", got, defaultMaxPerCycle)
	}
}

func TestEnricher_NegativeMaxPerCycleIsUnlimited(t *testing.T) {
	var requestCount atomic.Int32
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		_, _ = fmt.Fprintf(w, `<html><script id="__NEXT_DATA__" type="application/json">
{"props":{"pageProps":{"itemData":{"km":42000}}}}
</script></html>`)
	})
	enricher := newTestEnricher(t, handler, EnricherConfig{Delay: time.Millisecond, MaxPerCycle: -1})

	listings := make([]model.RawListing, 10)
	for i := range listings {
		listings[i] = model.RawListing{Token: fmt.Sprintf("tok-%d", i), Km: 0}
	}
	count := enricher.Enrich(context.Background(), listings)
	if count != 10 {
		t.Errorf("enriched = %d, want 10 (negative MaxPerCycle = unlimited)", count)
	}
	if got := requestCount.Load(); got != 10 {
		t.Errorf("requests = %d, want 10", got)
	}
}

func TestEnricher_FillsMissingKm(t *testing.T) {
	enricher := newTestEnricher(t, itemPageHandler(75000), EnricherConfig{
		Delay:       time.Millisecond,
		MaxPerCycle: 10,
	})

	listings := []model.RawListing{
		{Token: "a", Km: 50000, City: "Haifa", Area: "Center"},
		{Token: "b", Km: 0},
		{Token: "c", Km: 0},
	}

	count := enricher.Enrich(context.Background(), listings)
	if count != 2 {
		t.Errorf("enriched = %d, want 2", count)
	}
	if listings[0].Km != 50000 {
		t.Errorf("listing[0].Km = %d, want 50000 (unchanged)", listings[0].Km)
	}
	if listings[1].Km != 75000 {
		t.Errorf("listing[1].Km = %d, want 75000", listings[1].Km)
	}
	if listings[2].Km != 75000 {
		t.Errorf("listing[2].Km = %d, want 75000", listings[2].Km)
	}
}

func TestEnricher_RespectsMaxPerCycle(t *testing.T) {
	var requestCount atomic.Int32
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		_, _ = fmt.Fprintf(w, `<html><script id="__NEXT_DATA__" type="application/json">
{"props":{"pageProps":{"itemData":{"km":10000}}}}
</script></html>`)
	})

	enricher := newTestEnricher(t, handler, EnricherConfig{
		Delay:       time.Millisecond,
		MaxPerCycle: 1,
	})

	listings := []model.RawListing{
		{Token: "a", Km: 0},
		{Token: "b", Km: 0},
		{Token: "c", Km: 0},
	}

	count := enricher.Enrich(context.Background(), listings)
	if count != 1 {
		t.Errorf("enriched = %d, want 1 (max per cycle)", count)
	}
	// With shuffle, exactly 1 listing is enriched (any of the 3).
	enrichedCount := 0
	for _, l := range listings {
		if l.Km == 10000 {
			enrichedCount++
		}
	}
	if enrichedCount != 1 {
		t.Errorf("listings with km=10000: %d, want 1", enrichedCount)
	}
	if got := requestCount.Load(); got != 1 {
		t.Errorf("requests = %d, want 1 (budget limits successful enrichments)", got)
	}
}

func TestEnricher_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	enricher := newTestEnricher(t, itemPageHandler(10000), EnricherConfig{
		Delay:       time.Millisecond,
		MaxPerCycle: 10,
	})

	listings := []model.RawListing{
		{Token: "a", Km: 0},
		{Token: "b", Km: 0},
	}

	count := enricher.Enrich(ctx, listings)
	if count > 0 {
		t.Errorf("enriched = %d, want 0 (ctx canceled before first attempt)", count)
	}
}

func TestEnricher_SkipsOnError(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	enricher := newTestEnricher(t, handler, EnricherConfig{
		Delay:       time.Millisecond,
		MaxPerCycle: 10,
	})

	listings := []model.RawListing{
		{Token: "a", Km: 0},
	}

	count := enricher.Enrich(context.Background(), listings)
	if count != 0 {
		t.Errorf("enriched = %d, want 0 (error)", count)
	}
	if listings[0].Km != 0 {
		t.Errorf("listing[0].Km = %d, want 0 (unchanged on error)", listings[0].Km)
	}
}

func TestEnricher_AllHaveKm(t *testing.T) {
	var requestCount atomic.Int32
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		_, _ = fmt.Fprintf(w, `<html><script id="__NEXT_DATA__" type="application/json">
{"props":{"pageProps":{"itemData":{"km":50000}}}}
</script></html>`)
	})

	enricher := newTestEnricher(t, handler, EnricherConfig{
		Delay:       time.Millisecond,
		MaxPerCycle: 10,
	})

	listings := []model.RawListing{
		{Token: "a", Km: 10000, ImageURL: "https://img.yad2.co.il/a.jpg", City: "Tel Aviv"},
		{Token: "b", Km: 20000, ImageURL: "https://img.yad2.co.il/b.jpg", City: "Haifa"},
	}

	count := enricher.Enrich(context.Background(), listings)
	if count != 0 {
		t.Errorf("enriched = %d, want 0 (all have km, image, and city)", count)
	}
	if got := requestCount.Load(); got != 0 {
		t.Errorf("requests = %d, want 0 (no fetches needed)", got)
	}
}

func TestEnricher_FillsMissingImageOnly(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintf(w, `<html><script id="__NEXT_DATA__" type="application/json">
{"props":{"pageProps":{"itemData":{"km":50000,"coverImage":"https://img.yad2.co.il/test.jpg","address":{"city":{"text":"חיפה","textEng":"haifa"}}}}}}
</script></html>`)
	})

	enricher := newTestEnricher(t, handler, EnricherConfig{
		Delay:       time.Millisecond,
		MaxPerCycle: 10,
	})

	listings := []model.RawListing{
		{Token: "a", Km: 50000, ImageURL: "", City: "Haifa"},
	}

	count := enricher.Enrich(context.Background(), listings)
	if count != 1 {
		t.Errorf("enriched = %d, want 1 (image only)", count)
	}
	if listings[0].ImageURL != "https://img.yad2.co.il/test.jpg" {
		t.Errorf("listing[0].ImageURL = %q, want test.jpg URL", listings[0].ImageURL)
	}
	if listings[0].Km != 50000 {
		t.Errorf("listing[0].Km = %d, want 50000 (unchanged)", listings[0].Km)
	}
}

func TestEnricher_FillsMissingCity(t *testing.T) {
	enricher := newTestEnricher(t, itemPageHandler(75000), EnricherConfig{
		Delay:       time.Millisecond,
		MaxPerCycle: 10,
	})

	listings := []model.RawListing{
		{Token: "a", Km: 50000, ImageURL: "https://img.yad2.co.il/a.jpg", City: ""},
		{Token: "b", Km: 50000, ImageURL: "https://img.yad2.co.il/b.jpg", City: "Haifa"},
	}

	count := enricher.Enrich(context.Background(), listings)
	if count != 1 {
		t.Errorf("enriched = %d, want 1 (only first needs city)", count)
	}
	if listings[0].City != "tel_aviv" {
		t.Errorf("listing[0].City = %q, want tel_aviv", listings[0].City)
	}
	if listings[1].City != "Haifa" {
		t.Errorf("listing[1].City = %q, want Haifa (unchanged)", listings[1].City)
	}
}

func TestEnricher_FillsKmAndCity(t *testing.T) {
	enricher := newTestEnricher(t, itemPageHandler(90000), EnricherConfig{
		Delay:       time.Millisecond,
		MaxPerCycle: 10,
	})

	listings := []model.RawListing{
		{Token: "a", Km: 0, City: ""},
	}

	count := enricher.Enrich(context.Background(), listings)
	if count != 1 {
		t.Errorf("enriched = %d, want 1", count)
	}
	if listings[0].Km != 90000 {
		t.Errorf("listing[0].Km = %d, want 90000", listings[0].Km)
	}
	if listings[0].City != "tel_aviv" {
		t.Errorf("listing[0].City = %q, want tel_aviv", listings[0].City)
	}
}

func TestEnricher_AbortsOnBotChallenge(t *testing.T) {
	// With soft resume: one challenge backs off and continues; 2 consecutive
	// challenges abort. Here only tok-b triggers a challenge, so the enricher
	// backs off once and continues to the remaining items.
	var requestCount atomic.Int32
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		if strings.Contains(r.URL.Path, "tok-b") {
			w.WriteHeader(http.StatusForbidden)
			_, _ = fmt.Fprint(w, `<html>validate.perfdrive.com - Are you for real?</html>`)
			return
		}
		_, _ = fmt.Fprintf(w, `<html><script id="__NEXT_DATA__" type="application/json">
{"props":{"pageProps":{"itemData":{"km":10000}}}}
</script></html>`)
	})

	enricher := newTestEnricher(t, handler, EnricherConfig{
		Delay:            time.Millisecond,
		ChallengeBackoff: time.Millisecond,
	})

	listings := []model.RawListing{
		{Token: "tok-a", Km: 0},
		{Token: "tok-b", Km: 0},
		{Token: "tok-c", Km: 0},
		{Token: "tok-d", Km: 0},
	}

	count := enricher.Enrich(context.Background(), listings)
	// 3 out of 4 should be enriched (all except tok-b which triggers challenge).
	if count != 3 {
		t.Errorf("enriched = %d, want 3 (soft resume continues after one challenge)", count)
	}
	if got := requestCount.Load(); got != 4 {
		t.Errorf("requests = %d, want 4 (all tokens attempted)", got)
	}
	// tok-b should NOT be enriched.
	if listings[1].Km != 0 {
		t.Errorf("listing[1] (tok-b).Km = %d, want 0 (challenge token)", listings[1].Km)
	}
}

func TestEnricher_AbortsAfterRepeatedChallenges(t *testing.T) {
	// When ALL tokens trigger a challenge, the enricher should abort after
	// maxChallengeRetries consecutive challenges.
	var requestCount atomic.Int32
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusForbidden)
		_, _ = fmt.Fprint(w, `<html>validate.perfdrive.com - Are you for real?</html>`)
	})

	enricher := newTestEnricher(t, handler, EnricherConfig{
		Delay:            time.Millisecond,
		ChallengeBackoff: time.Millisecond,
	})

	listings := []model.RawListing{
		{Token: "a", Km: 0},
		{Token: "b", Km: 0},
		{Token: "c", Km: 0},
		{Token: "d", Km: 0},
	}

	count := enricher.Enrich(context.Background(), listings)
	if count != 0 {
		t.Errorf("enriched = %d, want 0 (all challenges)", count)
	}
	if got := requestCount.Load(); got != int32(maxChallengeRetries) {
		t.Errorf("requests = %d, want %d (abort after maxChallengeRetries)", got, maxChallengeRetries)
	}
}

func TestEnricher_AbortsAfterConsecutiveFailures(t *testing.T) {
	var requestCount atomic.Int32
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	})

	enricher := newTestEnricher(t, handler, EnricherConfig{Delay: time.Millisecond})

	listings := []model.RawListing{
		{Token: "a", Km: 0},
		{Token: "b", Km: 0},
		{Token: "c", Km: 0},
		{Token: "d", Km: 0},
		{Token: "e", Km: 0},
	}

	count := enricher.Enrich(context.Background(), listings)
	if count != 0 {
		t.Errorf("enriched = %d, want 0 (all failed)", count)
	}
	if got := requestCount.Load(); got != int32(maxConsecutiveFailures) {
		t.Errorf("requests = %d, want %d (abort after consecutive failures)", got, maxConsecutiveFailures)
	}
}

func TestEnricher_ConsecutiveFailureResets(t *testing.T) {
	var requestCount atomic.Int32
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := requestCount.Add(1)
		// Fail on requests 2 and 3, succeed on 1 and 4+
		if n == 2 || n == 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = fmt.Fprintf(w, `<html><script id="__NEXT_DATA__" type="application/json">
{"props":{"pageProps":{"itemData":{"km":50000}}}}
</script></html>`)
	})

	enricher := newTestEnricher(t, handler, EnricherConfig{Delay: time.Millisecond})

	listings := []model.RawListing{
		{Token: "a", Km: 0},
		{Token: "b", Km: 0},
		{Token: "c", Km: 0},
		{Token: "d", Km: 0},
		{Token: "e", Km: 0},
	}

	count := enricher.Enrich(context.Background(), listings)
	// Request 1 (tok-a): success, Request 2 (tok-b): fail, Request 3 (tok-c): fail,
	// Request 4 (tok-d): success (resets counter), Request 5 (tok-e): success
	if count != 3 {
		t.Errorf("enriched = %d, want 3 (success resets consecutive failure counter)", count)
	}
	if got := requestCount.Load(); got != 5 {
		t.Errorf("requests = %d, want 5 (all processed, counter reset by success)", got)
	}
}

func TestEnricher_FailedAttemptsDoNotConsumeSuccessBudget(t *testing.T) {
	var requestCount atomic.Int32
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	})

	enricher := newTestEnricher(t, handler, EnricherConfig{
		Delay:       time.Millisecond,
		MaxPerCycle: 2,
	})

	listings := []model.RawListing{
		{Token: "a", Km: 0},
		{Token: "b", Km: 0},
		{Token: "c", Km: 0},
	}

	count := enricher.Enrich(context.Background(), listings)
	if count != 0 {
		t.Errorf("enriched = %d, want 0 (all failed)", count)
	}
	if got := requestCount.Load(); got != int32(maxConsecutiveFailures) {
		t.Errorf("requests = %d, want %d (MaxPerCycle is successes only; abort on consecutive failures)", got, maxConsecutiveFailures)
	}
}
