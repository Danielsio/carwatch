// Command enrich-bench benchmarks Yad2 item page fetch speed at various delays
// to find the optimal rate limiting sweet spot. Uses the same config, user agents,
// and proxy settings as production.
//
// Usage:
//
//	enrich-bench --config config.yaml --ramp
//	enrich-bench --config config.yaml --delay 200ms --count 200
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"os"
	"os/signal"
	"time"

	"github.com/dsionov/carwatch/internal/app"
	"github.com/dsionov/carwatch/internal/fetcher"
	"github.com/dsionov/carwatch/internal/fetcher/yad2"
	"github.com/dsionov/carwatch/internal/model"
)

type searchSpec struct {
	name         string
	manufacturer int
	model        int
}

var searches = []searchSpec{
	{"Mazda 3", 27, 10332},
	{"Honda Civic", 17, 10236},
	{"Toyota Corolla", 19, 10378},
	{"Hyundai i30", 21, 10271},
}

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file (uses same user agents/proxy as production)")
	delay := flag.Duration("delay", 200*time.Millisecond, "delay between fetches")
	count := flag.Int("count", 200, "number of unique tokens to fetch")
	ramp := flag.Bool("ramp", false, "auto ramp-down test (overrides --delay/--count)")
	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	cfg, err := app.LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}

	fb, err := app.BuildFetchers(cfg, discardLogger())
	if err != nil {
		fmt.Fprintf(os.Stderr, "build fetchers: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "collecting tokens from %d searches...\n", len(searches))
	tokens := collectTokens(ctx, fb.Yad2, *count)
	if len(tokens) == 0 {
		fmt.Fprintln(os.Stderr, "no tokens collected, aborting")
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "collected %d unique tokens\n\n", len(tokens))

	if *ramp {
		runRamp(ctx, fb.Yad2, tokens)
	} else {
		if len(tokens) > *count {
			tokens = tokens[:*count]
		}
		results := runSustained(ctx, fb.Yad2, tokens, *delay)
		printTotal(results, *delay)
	}
}

func collectTokens(ctx context.Context, f *yad2.Yad2Fetcher, target int) []string {
	seen := make(map[string]bool)
	var tokens []string

	for _, s := range searches {
		if len(tokens) >= target {
			break
		}
		raw, err := f.Fetch(ctx, model.SourceParams{
			Manufacturer: s.manufacturer,
			Model:        s.model,
			PriceMax:     120000,
			YearMin:      2015,
			YearMax:      2023,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "  %s: fetch error: %v\n", s.name, err)
			continue
		}
		added := 0
		for _, l := range raw {
			if !seen[l.Token] {
				seen[l.Token] = true
				tokens = append(tokens, l.Token)
				added++
			}
		}
		fmt.Fprintf(os.Stderr, "  %s: %d new tokens (total: %d)\n", s.name, added, len(tokens))
	}

	rand.Shuffle(len(tokens), func(i, j int) {
		tokens[i], tokens[j] = tokens[j], tokens[i]
	})
	return tokens
}

type fetchResult struct {
	token     string
	fetchMs   int64
	challenge bool
	err       error
	wallTime  time.Time
}

func runSustained(ctx context.Context, f *yad2.Yad2Fetcher, tokens []string, delay time.Duration) []fetchResult {
	fmt.Printf("=== Sustained Benchmark ===\n")
	fmt.Printf("Delay: %v  Count: %d\n\n", delay, len(tokens))

	results := make([]fetchResult, 0, len(tokens))
	start := time.Now()
	windowStart := start
	var wSucceeded, wChallenges, wErrors int
	var wFetchTotal int64
	windowNum := 0

	for i, token := range tokens {
		if ctx.Err() != nil {
			break
		}

		if i > 0 && delay > 0 {
			select {
			case <-ctx.Done():
				return results
			case <-time.After(delay):
			}
		}

		fetchStart := time.Now()
		_, err := f.FetchItem(ctx, token)
		fetchMs := time.Since(fetchStart).Milliseconds()

		r := fetchResult{token: token, fetchMs: fetchMs, wallTime: fetchStart}
		if err != nil {
			if errors.Is(err, fetcher.ErrChallenge) {
				r.challenge = true
				wChallenges++
			} else {
				r.err = err
				wErrors++
			}
		} else {
			wSucceeded++
			wFetchTotal += fetchMs
		}
		results = append(results, r)

		if time.Since(windowStart) >= 30*time.Second {
			total := wSucceeded + wChallenges + wErrors
			var avgMs float64
			if wSucceeded > 0 {
				avgMs = float64(wFetchTotal) / float64(wSucceeded)
			}
			elapsed := time.Since(windowStart).Seconds()
			fmt.Printf("  %d:%02d-%d:%02d  fetched=%-3d  challenges=%-2d  errors=%-2d  avg=%3.0fms  rate=%.1f/s\n",
				windowNum*30/60, windowNum*30%60,
				(windowNum+1)*30/60, (windowNum+1)*30%60,
				total, wChallenges, wErrors, avgMs, float64(total)/elapsed)

			windowNum++
			windowStart = time.Now()
			wSucceeded, wChallenges, wErrors, wFetchTotal = 0, 0, 0, 0
		}
	}

	// Flush remaining window.
	if total := wSucceeded + wChallenges + wErrors; total > 0 {
		var avgMs float64
		if wSucceeded > 0 {
			avgMs = float64(wFetchTotal) / float64(wSucceeded)
		}
		elapsed := time.Since(windowStart).Seconds()
		fmt.Printf("  %d:%02d-end   fetched=%-3d  challenges=%-2d  errors=%-2d  avg=%3.0fms  rate=%.1f/s\n",
			windowNum*30/60, windowNum*30%60,
			total, wChallenges, wErrors, avgMs, float64(total)/elapsed)
	}

	return results
}

func runRamp(ctx context.Context, f *yad2.Yad2Fetcher, allTokens []string) {
	delays := []time.Duration{
		1000 * time.Millisecond,
		500 * time.Millisecond,
		250 * time.Millisecond,
		200 * time.Millisecond,
		150 * time.Millisecond,
		100 * time.Millisecond,
		50 * time.Millisecond,
		0,
	}

	tokensPerStep := 30
	fmt.Printf("=== Ramp-Down Test ===\n")
	fmt.Printf("Tokens per step: %d  Steps: %d\n\n", tokensPerStep, len(delays))

	offset := 0
	var sweetSpot time.Duration
	for _, d := range delays {
		if ctx.Err() != nil {
			break
		}
		if offset+tokensPerStep > len(allTokens) {
			fmt.Printf("--- not enough unique tokens for delay=%v, stopping ---\n", d)
			break
		}

		stepTokens := allTokens[offset : offset+tokensPerStep]
		offset += tokensPerStep

		fmt.Printf("--- Delay: %v (tokens %d-%d) ---\n", d, offset-tokensPerStep+1, offset)

		results := fetchBatch(ctx, f, stepTokens, d)
		s := summarize(results)

		fmt.Printf("  Succeeded: %d/%d  Challenges: %d  Errors: %d  Avg fetch: %.0fms  Throughput: %.1f/s\n\n",
			s.succeeded, len(results), s.challenges, s.errors, s.avgFetchMs, s.throughput)

		if s.challenges > 0 {
			fmt.Printf("*** CHALLENGES at %v — stopping ramp ***\n", d)
			break
		}
		sweetSpot = d

		fmt.Printf("  cooling down 5s...\n\n")
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
	}

	fmt.Printf("=== RESULT ===\n")
	fmt.Printf("Lowest delay with 0 challenges: %v\n", sweetSpot)
	recommended := sweetSpot + 50*time.Millisecond
	if sweetSpot == 0 {
		recommended = 100 * time.Millisecond
	}
	fmt.Printf("Recommended base_delay:          %v (with safety buffer)\n", recommended)
	fmt.Printf("\nNext step: run sustained test at the recommended delay:\n")
	fmt.Printf("  enrich-bench --config config.yaml --delay %v --count 200\n", recommended)
}

func fetchBatch(ctx context.Context, f *yad2.Yad2Fetcher, tokens []string, delay time.Duration) []fetchResult {
	results := make([]fetchResult, 0, len(tokens))

	for i, token := range tokens {
		if ctx.Err() != nil {
			break
		}
		if i > 0 && delay > 0 {
			select {
			case <-ctx.Done():
				return results
			case <-time.After(delay):
			}
		}

		fetchStart := time.Now()
		_, err := f.FetchItem(ctx, token)
		fetchMs := time.Since(fetchStart).Milliseconds()

		r := fetchResult{token: token, fetchMs: fetchMs}
		if err != nil {
			if errors.Is(err, fetcher.ErrChallenge) {
				r.challenge = true
			} else {
				r.err = err
			}
		}
		results = append(results, r)

		status := "ok"
		if r.challenge {
			status = "CHALLENGE"
		} else if r.err != nil {
			status = "ERR"
		}
		fmt.Printf("    #%-2d  fetch=%4dms  %s\n", i+1, fetchMs, status)
	}
	return results
}

type summary struct {
	succeeded  int
	challenges int
	errors     int
	avgFetchMs float64
	throughput float64
}

func summarize(results []fetchResult) summary {
	s := summary{}
	var totalFetch int64
	for _, r := range results {
		if r.challenge {
			s.challenges++
		} else if r.err != nil {
			s.errors++
		} else {
			s.succeeded++
			totalFetch += r.fetchMs
		}
	}
	if s.succeeded > 0 {
		s.avgFetchMs = float64(totalFetch) / float64(s.succeeded)
	}
	if len(results) > 0 && totalFetch > 0 {
		s.throughput = float64(s.succeeded) / (float64(totalFetch) / 1000.0)
	}
	return s
}

func printTotal(results []fetchResult, delay time.Duration) {
	s := summarize(results)
	total := len(results)

	if total == 0 {
		fmt.Println("\nNo results.")
		return
	}

	wallTime := results[len(results)-1].wallTime.Sub(results[0].wallTime) + time.Duration(results[len(results)-1].fetchMs)*time.Millisecond

	fmt.Printf("\n=== TOTAL ===\n")
	fmt.Printf("Succeeded: %d/%d  Challenges: %d  Errors: %d\n", s.succeeded, total, s.challenges, s.errors)
	fmt.Printf("Avg fetch: %.0fms  Wall time: %s  Throughput: %.1f listings/sec\n",
		s.avgFetchMs, wallTime.Round(time.Second), float64(s.succeeded)/wallTime.Seconds())

	if s.challenges > 0 {
		fmt.Printf("\n⚠ Challenges detected — delay %v is too aggressive\n", delay)
	} else {
		fmt.Printf("\n✓ No challenges — delay %v is safe\n", delay)
	}
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError + 1}))
}
