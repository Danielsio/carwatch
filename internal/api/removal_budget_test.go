package api

import "testing"

func TestRemovalBudget_CapsPerChatPerDay(t *testing.T) {
	b := newRemovalBudget()
	const chat = int64(1)

	// Draw down most of the budget in normal-sized chunks.
	if got, capped := b.take(chat, removalBudgetPerDay-5); got != removalBudgetPerDay-5 || capped {
		t.Fatalf("first take: got %d capped=%v", got, capped)
	}
	// Ask for more than remains: only the remainder is allowed, and it is capped.
	got, capped := b.take(chat, 20)
	if got != 5 || !capped {
		t.Fatalf("over-budget take: got %d capped=%v, want 5 capped", got, capped)
	}
	// Budget exhausted: nothing more today.
	if got, capped := b.take(chat, 1); got != 0 || !capped {
		t.Fatalf("exhausted take: got %d capped=%v, want 0 capped", got, capped)
	}
}

func TestRemovalBudget_IsPerChat(t *testing.T) {
	b := newRemovalBudget()
	if got, _ := b.take(1, removalBudgetPerDay); got != removalBudgetPerDay {
		t.Fatalf("chat 1 should get its full budget, got %d", got)
	}
	// A different chat has its own, untouched budget.
	if got, capped := b.take(2, 10); got != 10 || capped {
		t.Fatalf("chat 2 budget leaked from chat 1: got %d capped=%v", got, capped)
	}
}

func TestRemovalBudget_ResetsOnANewDay(t *testing.T) {
	b := newRemovalBudget()
	const chat = int64(1)
	b.take(chat, removalBudgetPerDay) // exhaust today

	// Simulate the clock rolling to tomorrow.
	b.epoch--

	got, capped := b.take(chat, 10)
	if got != 10 || capped {
		t.Fatalf("budget did not reset on a new day: got %d capped=%v", got, capped)
	}
}

func TestRemovalBudget_ZeroAndNegativeAreNoOps(t *testing.T) {
	b := newRemovalBudget()
	if got, capped := b.take(1, 0); got != 0 || capped {
		t.Fatalf("take(0) should be a no-op, got %d capped=%v", got, capped)
	}
	if got, capped := b.take(1, -5); got != 0 || capped {
		t.Fatalf("take(negative) should be a no-op, got %d capped=%v", got, capped)
	}
	// The no-ops must not have consumed any budget.
	if got, _ := b.take(1, removalBudgetPerDay); got != removalBudgetPerDay {
		t.Fatalf("no-op takes consumed budget, only %d left", got)
	}
}
