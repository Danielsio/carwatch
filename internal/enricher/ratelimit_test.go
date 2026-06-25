package enricher

import (
	"context"
	"testing"
	"time"
)

func TestAdaptiveRateLimiter_BaseDelay(t *testing.T) {
	rl := NewAdaptiveRateLimiter(10*time.Millisecond, 1*time.Second, 5*time.Second)

	if d := rl.CurrentDelay(); d != 10*time.Millisecond {
		t.Errorf("initial delay = %v, want 10ms", d)
	}
}

func TestAdaptiveRateLimiter_ChallengeDoublesDelay(t *testing.T) {
	rl := NewAdaptiveRateLimiter(10*time.Millisecond, 1*time.Second, 5*time.Second)

	// Build up success buffer to keep success rate above 50%
	for range 5 {
		rl.RecordSuccess()
	}

	rl.RecordChallenge()
	if d := rl.CurrentDelay(); d != 20*time.Millisecond {
		t.Errorf("after 1 challenge delay = %v, want 20ms", d)
	}

	rl.RecordChallenge()
	if d := rl.CurrentDelay(); d != 40*time.Millisecond {
		t.Errorf("after 2 challenges delay = %v, want 40ms", d)
	}
}

func TestAdaptiveRateLimiter_DelayCapAtMax(t *testing.T) {
	rl := NewAdaptiveRateLimiter(100*time.Millisecond, 500*time.Millisecond, 5*time.Second)

	for range 10 {
		rl.RecordSuccess()
		rl.RecordChallenge()
	}
	if d := rl.CurrentDelay(); d > 500*time.Millisecond {
		t.Errorf("delay %v exceeds max 500ms", d)
	}
}

func TestAdaptiveRateLimiter_SuccessHalvesDelay(t *testing.T) {
	rl := NewAdaptiveRateLimiter(10*time.Millisecond, 1*time.Second, 5*time.Second)

	rl.RecordSuccess()
	rl.RecordSuccess()
	rl.RecordChallenge()
	rl.RecordSuccess()

	d := rl.CurrentDelay()
	if d != 10*time.Millisecond {
		t.Errorf("after challenge then success delay = %v, want 10ms (halved to base)", d)
	}
}

func TestAdaptiveRateLimiter_CooldownOnLowSuccessRate(t *testing.T) {
	rl := NewAdaptiveRateLimiter(10*time.Millisecond, 1*time.Second, 100*time.Millisecond)

	rl.RecordChallenge()
	rl.RecordChallenge()
	rl.RecordChallenge()

	if !rl.InCooldown() {
		t.Error("should be in cooldown after 3 consecutive challenges (0% success rate)")
	}
}

func TestAdaptiveRateLimiter_CooldownExits(t *testing.T) {
	rl := NewAdaptiveRateLimiter(10*time.Millisecond, 1*time.Second, 50*time.Millisecond)

	rl.RecordChallenge()
	rl.RecordChallenge()
	rl.RecordChallenge()

	if !rl.InCooldown() {
		t.Fatal("should be in cooldown")
	}

	time.Sleep(60 * time.Millisecond)

	if rl.InCooldown() {
		t.Error("should have exited cooldown after duration")
	}
}

func TestAdaptiveRateLimiter_WaitRespectsContext(t *testing.T) {
	rl := NewAdaptiveRateLimiter(1*time.Second, 10*time.Second, 5*time.Second)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if rl.Wait(ctx) {
		t.Error("Wait should return false when context is cancelled")
	}
}

func TestAdaptiveRateLimiter_WaitCompletesNormally(t *testing.T) {
	rl := NewAdaptiveRateLimiter(10*time.Millisecond, 1*time.Second, 5*time.Second)

	start := time.Now()
	ok := rl.Wait(context.Background())
	elapsed := time.Since(start)

	if !ok {
		t.Error("Wait should return true")
	}
	if elapsed < 5*time.Millisecond {
		t.Errorf("Wait returned too quickly: %v", elapsed)
	}
}

func TestAdaptiveRateLimiter_NoCooldownWithHighSuccessRate(t *testing.T) {
	rl := NewAdaptiveRateLimiter(10*time.Millisecond, 1*time.Second, 100*time.Millisecond)

	rl.RecordSuccess()
	rl.RecordSuccess()
	rl.RecordSuccess()
	rl.RecordChallenge()

	if rl.InCooldown() {
		t.Error("should not enter cooldown with 75% success rate")
	}
}

func TestAdaptiveRateLimiter_ProgressiveCooldown(t *testing.T) {
	rl := NewAdaptiveRateLimiter(10*time.Millisecond, 1*time.Second, 50*time.Millisecond)

	// First cooldown trigger (3 challenges)
	rl.RecordChallenge()
	rl.RecordChallenge()
	rl.RecordChallenge()

	if !rl.InCooldown() {
		t.Fatal("should be in cooldown after first trigger")
	}

	// Verify first cooldown is short (~50ms) by checking it exits within reasonable time
	time.Sleep(100 * time.Millisecond)
	if rl.InCooldown() {
		t.Fatal("should have exited first cooldown (base 50ms) within 100ms")
	}

	// Second cooldown trigger
	rl.RecordChallenge()
	rl.RecordChallenge()
	rl.RecordChallenge()

	if !rl.InCooldown() {
		t.Fatal("should be in cooldown after second trigger")
	}

	// Verify second cooldown is longer (~100ms, doubled)
	// Should still be active after 50ms
	time.Sleep(50 * time.Millisecond)
	if !rl.InCooldown() {
		t.Error("second cooldown should still be active at 50ms (expected ~100ms)")
	}

	// But should exit within 200ms total
	time.Sleep(150 * time.Millisecond)
	if rl.InCooldown() {
		t.Fatal("should have exited second cooldown (doubled to ~100ms) within 200ms")
	}

	// Third cooldown trigger
	rl.RecordChallenge()
	rl.RecordChallenge()
	rl.RecordChallenge()

	if !rl.InCooldown() {
		t.Fatal("should be in cooldown after third trigger")
	}

	// Verify third cooldown is even longer (~200ms, quadrupled)
	// Should still be active after 150ms
	time.Sleep(150 * time.Millisecond)
	if !rl.InCooldown() {
		t.Error("third cooldown should still be active at 150ms (expected ~200ms)")
	}

	// But should exit within 300ms total
	time.Sleep(150 * time.Millisecond)
	if rl.InCooldown() {
		t.Fatal("should have exited third cooldown (quadrupled to ~200ms) within 300ms")
	}
}

func TestAdaptiveRateLimiter_CooldownResetsOnSuccess(t *testing.T) {
	rl := NewAdaptiveRateLimiter(10*time.Millisecond, 1*time.Second, 50*time.Millisecond)

	// Trigger consecutive cooldowns to escalate to level 2
	// First cooldown
	rl.RecordChallenge()
	rl.RecordChallenge()
	rl.RecordChallenge()
	time.Sleep(100 * time.Millisecond) // wait for 50ms cooldown to expire

	// Second cooldown
	rl.RecordChallenge()
	rl.RecordChallenge()
	rl.RecordChallenge()

	if !rl.InCooldown() {
		t.Fatal("should be in cooldown (second escalation)")
	}

	// Wait for second cooldown to expire (100ms)
	time.Sleep(200 * time.Millisecond)

	if rl.InCooldown() {
		t.Fatal("should have exited second cooldown")
	}

	// Record success (should reset consecutive counter)
	rl.RecordSuccess()

	if count := rl.ConsecutiveCooldowns(); count != 0 {
		t.Fatalf("consecutive cooldowns should be 0 after success, got %d", count)
	}

	// Build success buffer to ensure the next set of challenges triggers cooldown
	rl.RecordSuccess()
	rl.RecordSuccess()

	// Now trigger a new cooldown - should be at base level again
	rl.RecordChallenge()
	rl.RecordChallenge()
	rl.RecordChallenge()
	rl.RecordChallenge() // 4 challenges to overcome 3 successes

	if !rl.InCooldown() {
		t.Fatal("should be in cooldown after challenges")
	}

	count := rl.ConsecutiveCooldowns()
	t.Logf("consecutive cooldowns after new trigger: %d", count)

	// Should be back to base duration (~50ms), not escalated (100ms)
	// If it was still escalated, it would be >50ms
	time.Sleep(100 * time.Millisecond)
	if rl.InCooldown() {
		t.Errorf("cooldown should have reset to base (~50ms), still active after 100ms suggests escalation wasn't reset (consecutive=%d)", count)
	}
}

func TestAdaptiveRateLimiter_CooldownCapsAtMax(t *testing.T) {
	rl := NewAdaptiveRateLimiter(10*time.Millisecond, 1*time.Second, 100*time.Millisecond)

	// Trigger many consecutive cooldowns (max would be 400ms)
	for range 10 {
		if rl.InCooldown() {
			// Wait for cooldown to expire before next trigger
			time.Sleep(500 * time.Millisecond)
		}

		rl.RecordChallenge()
		rl.RecordChallenge()
		rl.RecordChallenge()
	}

	// Verify duration never exceeds max (400ms)
	cooldownEnd := rl.cooldownUntil
	maxExpected := time.Now().Add(400 * time.Millisecond)
	if cooldownEnd.After(maxExpected.Add(50 * time.Millisecond)) {
		t.Errorf("cooldown duration exceeds max: %v > ~400ms", time.Until(cooldownEnd))
	}

	// Verify it actually reached the max (not stuck at some lower value)
	minExpected := time.Now().Add(350 * time.Millisecond)
	if cooldownEnd.Before(minExpected) {
		t.Errorf("cooldown duration should reach max: %v < ~400ms", time.Until(cooldownEnd))
	}
}
