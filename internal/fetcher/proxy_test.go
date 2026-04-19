package fetcher

import (
	"testing"
)

func TestProxyPool_RoundRobin(t *testing.T) {
	pool := NewProxyPool([]string{"a", "b", "c"})

	got := pool.Next()
	if got != "a" {
		t.Errorf("first = %q, want a", got)
	}
	got = pool.Next()
	if got != "b" {
		t.Errorf("second = %q, want b", got)
	}
	got = pool.Next()
	if got != "c" {
		t.Errorf("third = %q, want c", got)
	}
	got = pool.Next()
	if got != "a" {
		t.Errorf("fourth (wrap) = %q, want a", got)
	}
}

func TestProxyPool_SkipsUnhealthy(t *testing.T) {
	pool := NewProxyPool([]string{"a", "b", "c"})

	pool.MarkUnhealthy("b")

	pool.Next() // a
	got := pool.Next()
	if got != "c" {
		t.Errorf("should skip unhealthy b, got %q", got)
	}
}

func TestProxyPool_Empty(t *testing.T) {
	pool := NewProxyPool(nil)
	got := pool.Next()
	if got != "" {
		t.Errorf("empty pool should return empty string, got %q", got)
	}
}

func TestProxyPool_MarkHealthy(t *testing.T) {
	pool := NewProxyPool([]string{"a", "b"})
	pool.MarkUnhealthy("a")
	pool.MarkHealthy("a")

	got := pool.Next()
	if got != "a" {
		t.Errorf("re-healthied proxy should be usable, got %q", got)
	}
}
