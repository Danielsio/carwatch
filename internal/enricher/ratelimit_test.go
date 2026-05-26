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
