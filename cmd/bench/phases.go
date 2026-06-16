package main

import (
	"context"
	"fmt"
	"time"

	"github.com/dsionov/carwatch/internal/benchutil"
)

// Phase describes a single benchmark phase.
type Phase struct {
	Name        string
	Description string
	NeedsYad2   bool
	NeedsDB     bool
	NeedsRedis  bool
	Run         func(ctx context.Context, env *BenchEnv) (*PhaseResult, error)
}

// BenchEnv holds shared resources for all phases.
type BenchEnv struct {
	Config         *benchConfig
	ProfileEnabled bool
	ProfileDir     string

	// Populated lazily by phases that need them.
	DBStore    interface{} // *postgres.Store, set by DB phases
	DBCleanup  func()

	// Cached results passed between phases.
	CachedListings interface{} // []model.RawListing from Yad2 phase
}

var phaseRegistry []Phase

func registerPhase(p Phase) {
	phaseRegistry = append(phaseRegistry, p)
}

// runPhases executes selected phases serially with cooldowns.
func runPhases(ctx context.Context, env *BenchEnv, selected map[string]bool) ([]PhaseResult, error) {
	// Build the list of phases to run so we know which is last.
	var toRun []struct {
		index int
		phase Phase
	}
	for i, p := range phaseRegistry {
		if len(selected) > 0 && !selected[p.Name] {
			continue
		}
		toRun = append(toRun, struct {
			index int
			phase Phase
		}{i, p})
	}

	var results []PhaseResult
	for runIdx, entry := range toRun {
		p := entry.phase
		fmt.Printf("\n▶ Phase %d: %s — %s\n", entry.index+1, p.Name, p.Description)

		var prof *benchutil.Profile
		if env.ProfileEnabled {
			var err error
			prof, err = benchutil.StartProfile(env.ProfileDir, p.Name)
			if err != nil {
				fmt.Printf("  ⚠ profile start failed: %v\n", err)
			}
		}

		start := time.Now()
		result, err := p.Run(ctx, env)
		elapsed := time.Since(start)

		if prof != nil {
			profDir, _ := prof.Stop()
			if result != nil {
				result.ProfilePath = profDir
			}
		}

		if err != nil {
			fmt.Printf("  ✗ ERROR: %v (%.1fs)\n", err, elapsed.Seconds())
			results = append(results, PhaseResult{
				Phase:      p.Name,
				DurationMs: elapsed.Milliseconds(),
				Pass:       false,
				FailReason: err.Error(),
				Metrics:    map[string]Metric{},
			})
			continue
		}

		result.DurationMs = elapsed.Milliseconds()
		status := "PASS"
		if !result.Pass {
			status = "FAIL: " + result.FailReason
		}
		fmt.Printf("  ✓ %s (%.1fs)\n", status, elapsed.Seconds())

		results = append(results, *result)

		// Cooldown between phases (skip after the last selected phase).
		if runIdx < len(toRun)-1 {
			cooldown := env.Config.cooldown
			if p.NeedsYad2 {
				cooldown = env.Config.yad2Cooldown
			}
			if cooldown > 0 {
				if err := waitCooldown(ctx, cooldown); err != nil {
					return results, err
				}
			}
		}
	}
	return results, nil
}

func waitCooldown(ctx context.Context, d time.Duration) error {
	fmt.Printf("  ⏳ cooldown %s...\n", d)
	select {
	case <-time.After(d):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
