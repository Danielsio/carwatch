package yad2

import (
	"context"
	"log/slog"
	"sync/atomic"
	"testing"
)

type mockHTTPDoer struct {
	closed atomic.Bool
}

func (m *mockHTTPDoer) Get(_ context.Context, _ string) (*HTTPResult, error) {
	return &HTTPResult{StatusCode: 200}, nil
}

func (m *mockHTTPDoer) Close() {
	m.closed.Store(true)
}

func TestClientPool_GetReturnsSameClient(t *testing.T) {
	pool := NewClientPool([]string{"ua1"}, slog.Default())
	defer pool.Close()

	pool.mu.Lock()
	mock := &mockHTTPDoer{}
	pool.clients["proxy1"] = mock
	pool.mu.Unlock()

	c1, err := pool.Get("proxy1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	c2, err := pool.Get("proxy1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	if c1 != c2 {
		t.Error("Get should return the same client for the same proxy")
	}
}

func TestClientPool_EvictRemovesClient(t *testing.T) {
	pool := NewClientPool([]string{"ua1"}, slog.Default())
	defer pool.Close()

	mock := &mockHTTPDoer{}
	pool.mu.Lock()
	pool.clients["proxy1"] = mock
	pool.mu.Unlock()

	pool.Evict("proxy1")

	if !mock.closed.Load() {
		t.Error("Evict should close the client")
	}

	pool.mu.Lock()
	_, ok := pool.clients["proxy1"]
	pool.mu.Unlock()
	if ok {
		t.Error("Evict should remove the client from the map")
	}
}

func TestClientPool_EvictNonexistent(t *testing.T) {
	pool := NewClientPool([]string{"ua1"}, slog.Default())
	defer pool.Close()

	pool.Evict("nonexistent")
}

func TestClientPool_CloseAll(t *testing.T) {
	pool := NewClientPool([]string{"ua1"}, slog.Default())

	m1, m2 := &mockHTTPDoer{}, &mockHTTPDoer{}
	pool.mu.Lock()
	pool.clients["proxy1"] = m1
	pool.clients["proxy2"] = m2
	pool.mu.Unlock()

	pool.Close()

	if !m1.closed.Load() || !m2.closed.Load() {
		t.Error("Close should close all clients")
	}

	pool.mu.Lock()
	if len(pool.clients) != 0 {
		t.Error("Close should clear the clients map")
	}
	pool.mu.Unlock()
}
