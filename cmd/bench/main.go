// Command bench runs the CarWatch performance benchmark suite.
//
// Each phase runs serially with configurable cooldowns between Yad2-hitting
// phases to allow rate limits to reset.
//
// Usage:
//
//	bench --config config.yaml
//	bench --config config.yaml --phases percolator,scoring
//	bench --config config.yaml --phases yad2 --yad2-cooldown 60s
package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/dsionov/carwatch/internal/app"
	"github.com/dsionov/carwatch/internal/benchutil"
	"github.com/dsionov/carwatch/internal/broker"
	"github.com/dsionov/carwatch/internal/config"
	"github.com/dsionov/carwatch/internal/fetcher"
	"github.com/dsionov/carwatch/internal/model"
	"github.com/dsionov/carwatch/internal/percolator"
	"github.com/dsionov/carwatch/internal/scoring"
	"github.com/dsionov/carwatch/internal/storage"
	"github.com/dsionov/carwatch/internal/storage/postgres"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type benchConfig struct {
	configPath      string
	phases          string
	cooldown        time.Duration
	yad2Cooldown    time.Duration
	users           int
	searchesPerUser int
	listings        int
	output          string
	pprof           bool
}

func main() {
	var bc benchConfig
	flag.StringVar(&bc.configPath, "config", "config.yaml", "path to CarWatch config file")
	flag.StringVar(&bc.phases, "phases", "all", "comma-separated phases or 'all'")
	flag.DurationVar(&bc.cooldown, "cooldown", 5*time.Second, "cooldown between non-Yad2 phases")
	flag.DurationVar(&bc.yad2Cooldown, "yad2-cooldown", 60*time.Second, "cooldown after Yad2 phases")
	flag.IntVar(&bc.users, "users", 100, "number of synthetic users")
	flag.IntVar(&bc.searchesPerUser, "searches-per-user", 3, "searches per user")
	flag.IntVar(&bc.listings, "listings", 200, "listings per simulated cycle")
	flag.StringVar(&bc.output, "output", "bench-results.json", "JSON output path")
	flag.BoolVar(&bc.pprof, "pprof", false, "enable CPU/heap profiling per phase")
	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	// Register phases in execution order.
	registerAllPhases()

	selected := parsePhases(bc.phases)

	env := &BenchEnv{
		Config:         &bc,
		ProfileEnabled: bc.pprof,
		ProfileDir:     "bench-profiles",
	}

	fmt.Println("════════════════════════════════════════════════════════════════════")
	fmt.Printf("  CARWATCH PERFORMANCE BENCHMARK\n")
	fmt.Printf("  Scale: %d users × %d searches × %d listings\n",
		bc.users, bc.searchesPerUser, bc.listings)
	fmt.Println("════════════════════════════════════════════════════════════════════")

	exitCode := run(ctx, env, selected, bc)

	// Cleanup runs after all phases (including on error/interrupt).
	if env.DBCleanup != nil {
		env.DBCleanup()
	}
	os.Exit(exitCode)
}

func run(ctx context.Context, env *BenchEnv, selected map[string]bool, bc benchConfig) int {
	results, err := runPhases(ctx, env, selected)
	if err != nil {
		fmt.Fprintf(os.Stderr, "benchmark aborted: %v\n", err)
		return 1
	}

	commit := gitCommit()
	report := &BenchReport{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		GitCommit: commit,
		Scale: map[string]int{
			"users":    bc.users,
			"searches": bc.users * bc.searchesPerUser,
			"listings": bc.listings,
		},
		Phases:  results,
		Summary: buildSummary(results),
	}

	printTable(report)

	if err := writeJSON(bc.output, report); err != nil {
		fmt.Fprintf(os.Stderr, "write results: %v\n", err)
		return 1
	}
	fmt.Printf("Results written to %s\n", bc.output)
	return 0
}

func parsePhases(s string) map[string]bool {
	if s == "all" || s == "" {
		return nil // nil means run all
	}
	m := make(map[string]bool)
	for _, p := range strings.Split(s, ",") {
		m[strings.TrimSpace(p)] = true
	}
	return m
}

func gitCommit() string {
	out, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

func buildSummary(results []PhaseResult) map[string]interface{} {
	passed, total := 0, 0
	var totalMs int64
	for _, r := range results {
		total++
		totalMs += r.DurationMs
		if r.Pass {
			passed++
		}
	}
	return map[string]interface{}{
		"total_duration_ms": totalMs,
		"phases_passed":     passed,
		"phases_total":      total,
	}
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError + 1}))
}

// registerAllPhases sets up all benchmark phases in execution order.
func registerAllPhases() {
	registerPhase(Phase{
		Name:        "percolator",
		Description: "Match listings against searches (CPU only)",
		Run:         phasePercolator,
	})
	registerPhase(Phase{
		Name:        "scoring",
		Description: "Fitness + deal scoring throughput (CPU only)",
		Run:         phaseScoring,
	})
	registerPhase(Phase{
		Name:        "db-dedup",
		Description: "ClaimNew contention (PostgreSQL)",
		NeedsDB:     true,
		Run:         phaseDBDedup,
	})
	registerPhase(Phase{
		Name:        "db-queries",
		Description: "Listing queries at scale (PostgreSQL)",
		NeedsDB:     true,
		Run:         phaseDBQueries,
	})
	registerPhase(Phase{
		Name:        "market",
		Description: "Market cache rebuild + lookup (PostgreSQL)",
		NeedsDB:     true,
		Run:         phaseMarketCache,
	})
	registerPhase(Phase{
		Name:        "broker",
		Description: "Redis stream publish/consume round-trip",
		NeedsRedis:  true,
		Run:         phaseBroker,
	})
	registerPhase(Phase{
		Name:        "yad2",
		Description: "Real Yad2 rate limit probing",
		NeedsYad2:   true,
		Run:         phaseYad2,
	})
	registerPhase(Phase{
		Name:        "cycle",
		Description: "Full scheduler cycle simulation",
		NeedsDB:     true,
		Run:         phaseFullCycle,
	})
}

// ─── Phase 1: Percolator ─────────────────────────────────────────────────

func phasePercolator(ctx context.Context, env *BenchEnv) (*PhaseResult, error) {
	rng := rand.New(rand.NewPCG(42, 0))
	users := benchutil.GenerateUsers(rng, env.Config.users, env.Config.searchesPerUser)
	searches := benchutil.AllSearches(users)
	listings := benchutil.GenerateListings(rng, env.Config.listings)

	p := percolator.New()
	p.Load(searches)

	durations := make([]time.Duration, len(listings))
	totalMatches := 0
	for i, l := range listings {
		start := time.Now()
		matches := p.Match(l)
		durations[i] = time.Since(start)
		totalMatches += len(matches)
	}

	p50, p95, p99 := benchutil.Percentiles(durations)
	mn, mx := benchutil.MinMax(durations)
	total := time.Duration(0)
	for _, d := range durations {
		total += d
	}
	tp := benchutil.Throughput(len(listings), total)

	pass := true
	failReason := ""
	if p99 > 500*time.Microsecond {
		pass = false
		failReason = fmt.Sprintf("p99=%v exceeds 500us threshold", p99)
	}

	return &PhaseResult{
		Phase:      "percolator",
		Pass:       pass,
		FailReason: failReason,
		Metrics: map[string]Metric{
			"per_listing_us": {
				Value: float64(p99.Microseconds()),
				Unit:  "microseconds",
				P50:   float64(p50.Microseconds()),
				P95:   float64(p95.Microseconds()),
				P99:   float64(p99.Microseconds()),
				Min:   float64(mn.Microseconds()),
				Max:   float64(mx.Microseconds()),
				Count: len(listings),
			},
			"throughput": {
				Value: tp,
				Unit:  "listings/sec",
			},
			"match_rate": {
				Value: float64(totalMatches) / float64(len(listings)*len(searches)),
				Unit:  "fraction",
				Count: totalMatches,
			},
		},
	}, nil
}

// ─── Phase 2: Scoring ────────────────────────────────────────────────────

func phaseScoring(ctx context.Context, env *BenchEnv) (*PhaseResult, error) {
	rng := rand.New(rand.NewPCG(42, 0))
	listings := benchutil.GenerateListings(rng, env.Config.listings)
	medianEntries := benchutil.GenerateMedianEntries(rng, 500)

	scoringEntries := make([]scoring.MedianEntry, len(medianEntries))
	for i, e := range medianEntries {
		scoringEntries[i] = scoring.MedianEntry{
			Manufacturer: e.Manufacturer,
			Model:        e.Model,
			Year:         e.Year,
			MedianPrice:  e.MedianPrice,
			MedianKm:     e.MedianKm,
			CohortSize:   e.CohortSize,
		}
	}
	mc := scoring.NewMarketCacheFromMedians(scoringEntries)

	durations := make([]time.Duration, len(listings))
	for i, l := range listings {
		medianPrice, medianKm, _, _ := mc.Lookup(l.Manufacturer, l.Model, l.Year)

		start := time.Now()
		scoring.FitnessScoreDetailed(scoring.FitnessParams{
			Price:        l.Price,
			Km:           l.Km,
			Hand:         l.Hand,
			Year:         l.Year,
			EngineVolume: l.EngineVolume,
			PriceMax:     200000,
			MaxKm:        200000,
			MaxHand:      4,
			YearMin:      2016,
			YearMax:      2026,
			EngineMinCC:  1400,
			MedianPrice:  medianPrice,
			MedianKm:     medianKm,
		})
		scoring.ScoreWithKm(l.Price, l.Km, medianPrice, medianKm)
		durations[i] = time.Since(start)
	}

	p50, p95, p99 := benchutil.Percentiles(durations)
	mn, mx := benchutil.MinMax(durations)
	total := time.Duration(0)
	for _, d := range durations {
		total += d
	}
	tp := benchutil.Throughput(len(listings), total)

	pass := true
	failReason := ""
	if p99 > 50*time.Microsecond {
		pass = false
		failReason = fmt.Sprintf("p99=%v exceeds 50us threshold", p99)
	}

	return &PhaseResult{
		Phase:      "scoring",
		Pass:       pass,
		FailReason: failReason,
		Metrics: map[string]Metric{
			"per_listing_us": {
				Value: float64(p99.Microseconds()),
				Unit:  "microseconds",
				P50:   float64(p50.Microseconds()),
				P95:   float64(p95.Microseconds()),
				P99:   float64(p99.Microseconds()),
				Min:   float64(mn.Microseconds()),
				Max:   float64(mx.Microseconds()),
				Count: len(listings),
			},
			"throughput": {
				Value: tp,
				Unit:  "scores/sec",
			},
		},
	}, nil
}

// ─── Phase 3: DB Dedup Contention ────────────────────────────────────────

func phaseDBDedup(ctx context.Context, env *BenchEnv) (*PhaseResult, error) {
	store, err := openBenchDB(env)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	rng := rand.New(rand.NewPCG(42, 0))
	users := benchutil.GenerateUsers(rng, env.Config.users, env.Config.searchesPerUser)

	// Seed users + searches.
	for _, u := range users {
		if err := store.UpsertUser(ctx, u.ChatID, u.Username); err != nil {
			return nil, fmt.Errorf("upsert user: %w", err)
		}
		for j := range u.Searches {
			id, err := store.CreateSearch(ctx, u.Searches[j])
			if err != nil {
				return nil, fmt.Errorf("create search: %w", err)
			}
			u.Searches[j].ID = id
		}
	}

	tokens := make([]string, env.Config.listings)
	for i := range tokens {
		tokens[i] = fmt.Sprintf("dedup-tok-%06d", i)
	}

	// Sub-test 1: single goroutine baseline.
	singleDurations := make([]time.Duration, len(tokens))
	user0 := users[0]
	search0 := user0.Searches[0]
	for i, tok := range tokens {
		start := time.Now()
		_, err := store.ClaimNew(ctx, tok, user0.ChatID, search0.ID)
		singleDurations[i] = time.Since(start)
		if err != nil {
			return nil, fmt.Errorf("claim: %w", err)
		}
	}
	sP50, sP95, sP99 := benchutil.Percentiles(singleDurations)

	// Sub-test 2: 4 goroutines contending on same tokens.
	concurrency := 4
	concDurations := make([][]time.Duration, concurrency)
	var wg sync.WaitGroup
	for g := range concurrency {
		wg.Add(1)
		concDurations[g] = make([]time.Duration, len(tokens))
		go func(gIdx int) {
			defer wg.Done()
			u := users[gIdx%len(users)]
			s := u.Searches[gIdx%len(u.Searches)]
			for i, tok := range tokens {
				start := time.Now()
				_, _ = store.ClaimNew(ctx, tok, u.ChatID, s.ID)
				concDurations[gIdx][i] = time.Since(start)
			}
		}(g)
	}
	wg.Wait()

	var allConc []time.Duration
	for _, d := range concDurations {
		allConc = append(allConc, d...)
	}
	cP50, cP95, cP99 := benchutil.Percentiles(allConc)

	contentionRatio := float64(cP99) / float64(sP99)
	if sP99 == 0 {
		contentionRatio = 1.0
	}

	pass := true
	failReason := ""
	if contentionRatio > 3.0 {
		pass = false
		failReason = fmt.Sprintf("contention ratio %.1fx exceeds 3x threshold", contentionRatio)
	}

	return &PhaseResult{
		Phase:      "db-dedup",
		Pass:       pass,
		FailReason: failReason,
		Metrics: map[string]Metric{
			"single_goroutine_us": {
				Value: float64(sP99.Microseconds()),
				Unit:  "microseconds",
				P50:   float64(sP50.Microseconds()),
				P95:   float64(sP95.Microseconds()),
				P99:   float64(sP99.Microseconds()),
				Count: len(tokens),
			},
			"concurrent_4g_us": {
				Value: float64(cP99.Microseconds()),
				Unit:  "microseconds",
				P50:   float64(cP50.Microseconds()),
				P95:   float64(cP95.Microseconds()),
				P99:   float64(cP99.Microseconds()),
				Count: len(allConc),
			},
			"contention_ratio": {
				Value: contentionRatio,
				Unit:  "x",
			},
		},
	}, nil
}

// ─── Phase 4: DB Listing Queries ─────────────────────────────────────────

func phaseDBQueries(ctx context.Context, env *BenchEnv) (*PhaseResult, error) {
	store, err := openBenchDB(env)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	rng := rand.New(rand.NewPCG(42, 0))
	users := benchutil.GenerateUsers(rng, env.Config.users, env.Config.searchesPerUser)
	listings := benchutil.GenerateListings(rng, env.Config.listings)

	// Seed users + searches.
	for i := range users {
		if err := store.UpsertUser(ctx, users[i].ChatID, users[i].Username); err != nil {
			return nil, fmt.Errorf("upsert user: %w", err)
		}
		for j := range users[i].Searches {
			id, err := store.CreateSearch(ctx, users[i].Searches[j])
			if err != nil {
				return nil, fmt.Errorf("create search: %w", err)
			}
			users[i].Searches[j].ID = id
		}
	}

	// Build records: distribute distinct listings across users/searches.
	var allRecords []storage.ListingRecord
	listingIdx := 0
	for _, u := range users {
		for _, s := range u.Searches {
			count := min(20, len(listings))
			subset := make([]model.RawListing, count)
			for j := range count {
				l := listings[listingIdx%len(listings)]
				l.Token = fmt.Sprintf("%s-%d-%d", l.Token, u.ChatID, s.ID)
				subset[j] = l
				listingIdx++
			}
			records := benchutil.ToListingRecords(subset, u.ChatID, s.ID, s.Name)
			allRecords = append(allRecords, records...)
		}
	}

	// Sub-test 1: SaveListings batch insert.
	batchSize := 500
	saveStart := time.Now()
	for i := 0; i < len(allRecords); i += batchSize {
		end := min(i+batchSize, len(allRecords))
		if err := store.SaveListings(ctx, allRecords[i:end]); err != nil {
			return nil, fmt.Errorf("save listings batch: %w", err)
		}
	}
	saveDuration := time.Since(saveStart)
	saveRate := benchutil.Throughput(len(allRecords), saveDuration)

	// Sub-test 2: ListSearchListings pagination.
	sampleSearches := users[0].Searches
	if len(sampleSearches) > 5 {
		sampleSearches = sampleSearches[:5]
	}
	var queryDurations []time.Duration
	for _, s := range sampleSearches {
		for offset := 0; offset < 60; offset += 20 {
			start := time.Now()
			_, err := store.ListSearchListings(ctx, s.ChatID, s.ID, storage.ListingFilter{}, 20, offset, "")
			queryDurations = append(queryDurations, time.Since(start))
			if err != nil {
				return nil, fmt.Errorf("list query: %w", err)
			}
		}
	}
	qP50, qP95, qP99 := benchutil.Percentiles(queryDurations)

	// Sub-test 3: CountSearchListings.
	var countDurations []time.Duration
	for _, u := range users[:min(10, len(users))] {
		for _, s := range u.Searches {
			start := time.Now()
			_, err := store.CountSearchListings(ctx, s.ChatID, s.ID, storage.ListingFilter{})
			countDurations = append(countDurations, time.Since(start))
			if err != nil {
				return nil, fmt.Errorf("count query: %w", err)
			}
		}
	}
	countP50, countP95, countP99 := benchutil.Percentiles(countDurations)

	// Sub-test 4: LookupEnrichmentData.
	lookupTokens := make([]string, min(100, len(listings)))
	for i := range lookupTokens {
		lookupTokens[i] = listings[i].Token
	}
	lookupStart := time.Now()
	_, err = store.LookupEnrichmentData(ctx, lookupTokens)
	lookupDuration := time.Since(lookupStart)
	if err != nil {
		return nil, fmt.Errorf("lookup enrichment: %w", err)
	}

	pass := true
	failReason := ""
	if qP99 > 50*time.Millisecond {
		pass = false
		failReason = fmt.Sprintf("list query p99=%v exceeds 50ms", qP99)
	}

	return &PhaseResult{
		Phase:      "db-queries",
		Pass:       pass,
		FailReason: failReason,
		Metrics: map[string]Metric{
			"save_rate": {
				Value: saveRate,
				Unit:  "records/sec",
				Count: len(allRecords),
			},
			"per_query_us": {
				Value: float64(qP99.Microseconds()),
				Unit:  "microseconds",
				P50:   float64(qP50.Microseconds()),
				P95:   float64(qP95.Microseconds()),
				P99:   float64(qP99.Microseconds()),
				Count: len(queryDurations),
			},
			"count_query_us": {
				Value: float64(countP99.Microseconds()),
				Unit:  "microseconds",
				P50:   float64(countP50.Microseconds()),
				P95:   float64(countP95.Microseconds()),
				P99:   float64(countP99.Microseconds()),
				Count: len(countDurations),
			},
			"enrichment_lookup_ms": {
				Value: float64(lookupDuration.Milliseconds()),
				Unit:  "ms",
				Count: len(lookupTokens),
			},
		},
	}, nil
}

// ─── Phase 5: Market Cache ───────────────────────────────────────────────

func phaseMarketCache(ctx context.Context, env *BenchEnv) (*PhaseResult, error) {
	store, err := openBenchDB(env)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	rng := rand.New(rand.NewPCG(42, 0))
	users := benchutil.GenerateUsers(rng, 10, 3)
	listings := benchutil.GenerateListings(rng, env.Config.listings)

	// Seed minimal data for materialized view.
	for i := range users {
		if err := store.UpsertUser(ctx, users[i].ChatID, users[i].Username); err != nil {
			return nil, fmt.Errorf("upsert user: %w", err)
		}
		for j := range users[i].Searches {
			id, err := store.CreateSearch(ctx, users[i].Searches[j])
			if err != nil {
				return nil, fmt.Errorf("create search: %w", err)
			}
			users[i].Searches[j].ID = id
		}
	}

	// Insert listings for the view.
	for _, u := range users {
		for _, s := range u.Searches {
			records := benchutil.ToListingRecords(listings, u.ChatID, s.ID, s.Name)
			if err := store.SaveListings(ctx, records); err != nil {
				return nil, fmt.Errorf("save listings: %w", err)
			}
		}
	}

	// Sub-test 1: RefreshMarketMedians.
	refreshStart := time.Now()
	if err := store.RefreshMarketMedians(ctx); err != nil {
		return nil, fmt.Errorf("refresh medians: %w", err)
	}
	refreshDuration := time.Since(refreshStart)

	// Sub-test 2: LoadMarketMedians.
	loadStart := time.Now()
	rows, err := store.LoadMarketMedians(ctx)
	if err != nil {
		return nil, fmt.Errorf("load medians: %w", err)
	}
	loadDuration := time.Since(loadStart)

	// Sub-test 3: Build cache.
	medianEntries := make([]scoring.MedianEntry, len(rows))
	for i, r := range rows {
		medianEntries[i] = scoring.MedianEntry{
			Manufacturer: r.Manufacturer,
			Model:        r.Model,
			Year:         r.Year,
			MedianPrice:  r.MedianPrice,
			MedianKm:     r.MedianKm,
			CohortSize:   r.CohortSize,
		}
	}
	buildStart := time.Now()
	mc := scoring.NewMarketCacheFromMedians(medianEntries)
	buildDuration := time.Since(buildStart)

	// Sub-test 4: Lookup throughput.
	lookupCount := 10_000
	var lookupDurations []time.Duration
	for i := range lookupCount {
		l := listings[i%len(listings)]
		start := time.Now()
		mc.Lookup(l.Manufacturer, l.Model, l.Year)
		lookupDurations = append(lookupDurations, time.Since(start))
	}
	lP50, lP95, lP99 := benchutil.Percentiles(lookupDurations)
	lookupTotal := time.Duration(0)
	for _, d := range lookupDurations {
		lookupTotal += d
	}
	lookupTP := benchutil.Throughput(lookupCount, lookupTotal)

	pass := true
	failReason := ""
	if refreshDuration > 2*time.Second {
		pass = false
		failReason = fmt.Sprintf("refresh took %v, exceeds 2s", refreshDuration)
	}

	return &PhaseResult{
		Phase:      "market",
		Pass:       pass,
		FailReason: failReason,
		Metrics: map[string]Metric{
			"refresh_ms": {
				Value: float64(refreshDuration.Milliseconds()),
				Unit:  "ms",
			},
			"load_ms": {
				Value: float64(loadDuration.Milliseconds()),
				Unit:  "ms",
				Count: len(rows),
			},
			"build_us": {
				Value: float64(buildDuration.Microseconds()),
				Unit:  "us",
			},
			"lookup_throughput": {
				Value: lookupTP,
				Unit:  "lookups/sec",
				Count: lookupCount,
			},
			"per_op_us": {
				Value: float64(lP99.Microseconds()),
				Unit:  "microseconds",
				P50:   float64(lP50.Microseconds()),
				P95:   float64(lP95.Microseconds()),
				P99:   float64(lP99.Microseconds()),
				Count: lookupCount,
			},
		},
	}, nil
}

// ─── Phase 6: Broker ─────────────────────────────────────────────────────

func phaseBroker(ctx context.Context, env *BenchEnv) (*PhaseResult, error) {
	cfg, err := loadConfig(env.Config.configPath)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	pub, err := broker.NewPublisher(cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB)
	if err != nil {
		return nil, fmt.Errorf("connect redis: %w", err)
	}
	defer func() { _ = pub.Close() }()

	// All sub-tests use an isolated stream to avoid polluting the
	// production carwatch:alerts stream.
	client := pub.Client()
	testStream := "carwatch:bench-test"
	group := "bench-consumers"
	consumer := "bench-worker-1"

	client.Del(ctx, testStream)
	client.XGroupCreateMkStream(ctx, testStream, group, "0")

	// Sub-test 1: Sequential publish throughput.
	publishCount := 100
	pubDurations := make([]time.Duration, 0, publishCount)
	for i := range publishCount {
		values := map[string]any{
			"data": fmt.Sprintf(`{"chat_id":%d,"search_name":"bench","message":"bench-%d"}`, 100_000+i, i),
		}
		start := time.Now()
		if err := client.XAdd(ctx, &redis.XAddArgs{
			Stream: testStream,
			MaxLen: 100000,
			Approx: true,
			Values: values,
		}).Err(); err != nil {
			return nil, fmt.Errorf("publish: %w", err)
		}
		pubDurations = append(pubDurations, time.Since(start))
	}
	pP50, pP95, pP99 := benchutil.Percentiles(pubDurations)

	// Drain published messages before round-trip test.
	client.Del(ctx, testStream)
	client.XGroupCreateMkStream(ctx, testStream, group, "0")

	// Sub-test 2: Round-trip latency (publish → XREAD).
	rtCount := 50
	rtDurations := make([]time.Duration, 0, rtCount)
	for i := range rtCount {
		msg := fmt.Sprintf("rt-msg-%d", i)
		pubTime := time.Now()
		if err := client.XAdd(ctx, &redis.XAddArgs{
			Stream: testStream,
			Values: map[string]any{"data": msg, "sent_at": pubTime.UnixNano()},
		}).Err(); err != nil {
			return nil, fmt.Errorf("xadd: %w", err)
		}
		res, err := client.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    group,
			Consumer: consumer,
			Streams:  []string{testStream, ">"},
			Count:    1,
			Block:    5 * time.Second,
		}).Result()
		recvTime := time.Now()
		if err != nil {
			return nil, fmt.Errorf("xreadgroup: %w", err)
		}
		if len(res) > 0 && len(res[0].Messages) > 0 {
			rtDurations = append(rtDurations, recvTime.Sub(pubTime))
			client.XAck(ctx, testStream, group, res[0].Messages[0].ID)
		}
	}

	client.Del(ctx, testStream)

	rtP50, rtP95, rtP99 := benchutil.Percentiles(rtDurations)

	pass := true
	failReason := ""
	if rtP99 > 50*time.Millisecond {
		pass = false
		failReason = fmt.Sprintf("round-trip p99=%v exceeds 50ms", rtP99)
	}

	return &PhaseResult{
		Phase:      "broker",
		Pass:       pass,
		FailReason: failReason,
		Metrics: map[string]Metric{
			"publish_us": {
				Value: float64(pP99.Microseconds()),
				Unit:  "microseconds",
				P50:   float64(pP50.Microseconds()),
				P95:   float64(pP95.Microseconds()),
				P99:   float64(pP99.Microseconds()),
				Count: publishCount,
			},
			"round_trip_us": {
				Value: float64(rtP99.Microseconds()),
				Unit:  "microseconds",
				P50:   float64(rtP50.Microseconds()),
				P95:   float64(rtP95.Microseconds()),
				P99:   float64(rtP99.Microseconds()),
				Count: len(rtDurations),
			},
		},
	}, nil
}

// ─── Phase 7: Yad2 ──────────────────────────────────────────────────────

type yad2SearchSpec struct {
	name         string
	manufacturer int
	modelID      int
	yearMin      int
	yearMax      int
	priceMax     int
}

var defaultYad2Searches = []yad2SearchSpec{
	{"Mazda 3", 27, 10332, 2018, 2024, 130000},
	{"Honda Civic", 17, 10236, 2017, 2023, 140000},
	{"Toyota Corolla", 19, 10378, 2018, 2024, 135000},
}

func phaseYad2(ctx context.Context, env *BenchEnv) (*PhaseResult, error) {
	cfg, err := loadConfig(env.Config.configPath)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	fb, err := app.BuildFetchers(cfg, discardLogger())
	if err != nil {
		return nil, fmt.Errorf("build fetchers: %w", err)
	}

	metrics := make(map[string]Metric)
	var allListings []model.RawListing
	totalChallenges := 0
	totalErrors := 0

	// 7a: Search page fetches (3 searches, 1 page each, 3s delay).
	fmt.Println("    7a: Search page fetching...")
	var searchDurations []time.Duration
	for _, spec := range defaultYad2Searches {
		params := model.SourceParams{
			Manufacturer: spec.manufacturer,
			Model:        spec.modelID,
			YearMin:      spec.yearMin,
			YearMax:      spec.yearMax,
			PriceMax:     spec.priceMax,
			Page:         1,
		}
		start := time.Now()
		results, err := fb.Targeted.Fetch(ctx, params)
		elapsed := time.Since(start)
		searchDurations = append(searchDurations, elapsed)

		if err != nil {
			if errors.Is(err, fetcher.ErrChallenge) {
				totalChallenges++
				fmt.Printf("    ⚠ challenge on %s\n", spec.name)
			} else {
				totalErrors++
				fmt.Printf("    ⚠ error on %s: %v\n", spec.name, err)
			}
		} else {
			allListings = append(allListings, results...)
			fmt.Printf("    %s: %d listings in %v\n", spec.name, len(results), elapsed)
		}

		time.Sleep(3 * time.Second)
	}
	sP50, sP95, sP99 := benchutil.Percentiles(searchDurations)
	metrics["search_page_us"] = Metric{
		Value: float64(sP99.Microseconds()),
		Unit:  "microseconds",
		P50:   float64(sP50.Microseconds()),
		P95:   float64(sP95.Microseconds()),
		P99:   float64(sP99.Microseconds()),
		Count: len(searchDurations),
	}

	// 7b: Item page fetches (10 items, 500ms delay).
	if len(allListings) > 0 {
		fmt.Println("    7b: Item page fetching...")
		itemCount := min(10, len(allListings))
		var itemDurations []time.Duration
		for i := range itemCount {
			token := allListings[i].Token
			start := time.Now()
			_, err := fb.Yad2.FetchItem(ctx, token)
			elapsed := time.Since(start)
			itemDurations = append(itemDurations, elapsed)

			if err != nil {
				if errors.Is(err, fetcher.ErrChallenge) {
					totalChallenges++
				} else if !errors.Is(err, fetcher.ErrItemGone) {
					totalErrors++
				}
			}
			time.Sleep(500 * time.Millisecond)
		}
		iP50, iP95, iP99 := benchutil.Percentiles(itemDurations)
		metrics["item_page_us"] = Metric{
			Value: float64(iP99.Microseconds()),
			Unit:  "microseconds",
			P50:   float64(iP50.Microseconds()),
			P95:   float64(iP95.Microseconds()),
			P99:   float64(iP99.Microseconds()),
			Count: len(itemDurations),
		}

		// 7c: Rate limit ramp-down (5 items per delay step).
		fmt.Println("    7c: Rate limit ramp-down...")
		delayStepsMs := []int{500, 300, 200, 100, 50}
		rampTokens := allListings[itemCount:]
		if len(rampTokens) > 25 {
			rampTokens = rampTokens[:25]
		}
		thresholdDelay := time.Duration(0)
		tokenIdx := 0
		for _, ms := range delayStepsMs {
			delay := time.Duration(ms) * time.Millisecond
			challenged := false
			for range 5 {
				if tokenIdx >= len(rampTokens) {
					break
				}
				_, err := fb.Yad2.FetchItem(ctx, rampTokens[tokenIdx].Token)
				tokenIdx++
				if errors.Is(err, fetcher.ErrChallenge) {
					challenged = true
					totalChallenges++
					break
				}
				time.Sleep(delay)
			}
			if challenged {
				thresholdDelay = delay
				fmt.Printf("    Challenge at %dms delay\n", ms)
				break
			}
			fmt.Printf("    %dms delay: OK\n", ms)
		}
		metrics["rate_limit_threshold_ms"] = Metric{
			Value: float64(thresholdDelay.Milliseconds()),
			Unit:  "ms",
		}
	}

	metrics["challenges"] = Metric{Value: float64(totalChallenges), Unit: "count"}
	metrics["errors"] = Metric{Value: float64(totalErrors), Unit: "count"}

	// Cache listings for Phase 8.
	env.CachedListings = allListings

	pass := true
	failReason := ""
	if totalChallenges > 0 {
		pass = false
		failReason = fmt.Sprintf("%d challenges detected", totalChallenges)
	}

	return &PhaseResult{
		Phase:      "yad2",
		Pass:       pass,
		FailReason: failReason,
		Metrics:    metrics,
	}, nil
}

// ─── Phase 8: Full Cycle ─────────────────────────────────────────────────

func phaseFullCycle(ctx context.Context, env *BenchEnv) (*PhaseResult, error) {
	store, err := openBenchDB(env)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	rng := rand.New(rand.NewPCG(42, 0))
	users := benchutil.GenerateUsers(rng, env.Config.users, env.Config.searchesPerUser)

	// Use cached Yad2 listings or generate synthetic.
	var listings []model.RawListing
	if cached, ok := env.CachedListings.([]model.RawListing); ok && len(cached) > 0 {
		listings = cached
		fmt.Printf("    Using %d cached Yad2 listings\n", len(listings))
	} else {
		listings = benchutil.GenerateListings(rng, env.Config.listings)
		fmt.Printf("    Using %d synthetic listings\n", len(listings))
	}

	// Seed users + searches.
	for i := range users {
		if err := store.UpsertUser(ctx, users[i].ChatID, users[i].Username); err != nil {
			return nil, fmt.Errorf("upsert user: %w", err)
		}
		for j := range users[i].Searches {
			id, err := store.CreateSearch(ctx, users[i].Searches[j])
			if err != nil {
				return nil, fmt.Errorf("create search: %w", err)
			}
			users[i].Searches[j].ID = id
		}
	}

	allSearches := benchutil.AllSearches(users)

	// Stage 1: Load percolator.
	perc := percolator.New()
	percLoadStart := time.Now()
	perc.Load(allSearches)
	percLoadDuration := time.Since(percLoadStart)

	// Stage 2: Match all listings.
	matchStart := time.Now()
	type matchPair struct {
		listing model.RawListing
		matches []percolator.MatchResult
	}
	var matched []matchPair
	for _, l := range listings {
		m := perc.Match(l)
		if len(m) > 0 {
			matched = append(matched, matchPair{listing: l, matches: m})
		}
	}
	matchDuration := time.Since(matchStart)

	// Stage 3: Dedup claims.
	claimStart := time.Now()
	claimedCount := 0
	for _, mp := range matched {
		for _, m := range mp.matches {
			claimed, _ := store.ClaimNew(ctx, mp.listing.Token, m.ChatID, m.SearchID)
			if claimed {
				claimedCount++
			}
		}
	}
	claimDuration := time.Since(claimStart)

	// Stage 4: Score + persist.
	persistStart := time.Now()
	persistErrors := 0
	for _, mp := range matched {
		for _, m := range mp.matches {
			fitness := scoring.FitnessScore(scoring.FitnessParams{
				Price:    mp.listing.Price,
				Km:       mp.listing.Km,
				Hand:     mp.listing.Hand,
				Year:     mp.listing.Year,
				PriceMax: m.Search.PriceMax,
				MaxKm:    m.Search.MaxKm,
				MaxHand:  m.Search.MaxHand,
				YearMin:  m.Search.YearMin,
				YearMax:  m.Search.YearMax,
			})
			record := storage.ListingRecord{
				Token:        mp.listing.Token,
				ChatID:       m.ChatID,
				SearchID:     m.SearchID,
				SearchName:   m.SearchName,
				Manufacturer: mp.listing.Manufacturer,
				Model:        mp.listing.Model,
				Year:         mp.listing.Year,
				Price:        mp.listing.Price,
				Km:           mp.listing.Km,
				Hand:         mp.listing.Hand,
				City:         mp.listing.City,
				FitnessScore: &fitness,
				FirstSeenAt:  time.Now(),
			}
			if err := store.SaveListing(ctx, record); err != nil {
				persistErrors++
			}
		}
	}
	persistDuration := time.Since(persistStart)

	totalCycle := percLoadDuration + matchDuration + claimDuration + persistDuration

	pass := true
	failReason := ""
	if totalCycle > 30*time.Second {
		pass = false
		failReason = fmt.Sprintf("cycle %v exceeds 30s", totalCycle)
	}
	if persistErrors > 0 && pass {
		pass = false
		failReason = fmt.Sprintf("%d persist errors", persistErrors)
	}

	return &PhaseResult{
		Phase:      "cycle",
		Pass:       pass,
		FailReason: failReason,
		Metrics: map[string]Metric{
			"cycle_ms": {
				Value: float64(totalCycle.Milliseconds()),
				Unit:  "ms",
			},
			"percolator_load_ms": {
				Value: float64(percLoadDuration.Milliseconds()),
				Unit:  "ms",
			},
			"match_ms": {
				Value: float64(matchDuration.Milliseconds()),
				Unit:  "ms",
				Count: len(listings),
			},
			"claim_ms": {
				Value: float64(claimDuration.Milliseconds()),
				Unit:  "ms",
				Count: claimedCount,
			},
			"persist_ms": {
				Value:  float64(persistDuration.Milliseconds()),
				Unit:   "ms",
				Errors: persistErrors,
			},
			"matched_listings": {
				Value: float64(len(matched)),
				Unit:  "listings",
			},
			"total_claims": {
				Value: float64(claimedCount),
				Unit:  "claims",
			},
		},
	}, nil
}

// ─── DB helpers ──────────────────────────────────────────────────────────

// openBenchDB returns a shared store for all DB phases. The store is
// created once (isolated schema) and reused. Cleanup runs in main()
// after all phases complete — individual phases must NOT close the store.
func openBenchDB(env *BenchEnv) (*postgres.Store, error) {
	if env.DBStore != nil {
		return env.DBStore.(*postgres.Store), nil
	}

	cfg, err := loadConfig(env.Config.configPath)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		dsn = cfg.Storage.DSN
	}

	schema := fmt.Sprintf("bench_%d", time.Now().UnixNano())
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open for schema: %w", err)
	}
	if _, err := db.Exec("CREATE SCHEMA " + schema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("create schema: %w", err)
	}
	_ = db.Close()

	schemaDSN := dsn
	if strings.Contains(schemaDSN, "?") {
		schemaDSN += "&search_path=" + schema
	} else {
		schemaDSN += "?search_path=" + schema
	}

	store, err := postgres.New(schemaDSN, cfg.Storage.MigrationsPath)
	if err != nil {
		return nil, fmt.Errorf("open store: %w", err)
	}

	env.DBStore = store
	env.DBCleanup = func() {
		_ = store.Close()
		db2, err := sql.Open("pgx", dsn)
		if err != nil {
			return
		}
		defer func() { _ = db2.Close() }()
		_, _ = db2.Exec("DROP SCHEMA " + schema + " CASCADE")
	}

	return store, nil
}

func loadConfig(path string) (*config.Config, error) {
	return config.Load(path)
}
