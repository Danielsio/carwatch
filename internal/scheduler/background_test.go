package scheduler

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/dsionov/carwatch/internal/storage"
)

// blockingMarketStore blocks inside RefreshMarketMedians until released, so a
// test can observe the background refresh goroutine still running at shutdown.
type blockingMarketStore struct {
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (m *blockingMarketStore) RefreshMarketMedians(_ context.Context) error {
	m.once.Do(func() { close(m.entered) })
	<-m.release
	return nil
}

func (m *blockingMarketStore) LoadMarketMedians(_ context.Context) ([]storage.MarketMedianRow, error) {
	return nil, nil
}

// TestScheduler_WaitsForBackgroundGoroutine verifies that the market-refresh
// goroutine is tracked by bgWG, so a returning Run (which defers bgWG.Wait)
// cannot complete — and let the caller close the store — while a refresh is
// still in flight. (F2)
func TestScheduler_WaitsForBackgroundGoroutine(t *testing.T) {
	market := &blockingMarketStore{
		entered: make(chan struct{}),
		release: make(chan struct{}),
	}
	s := &Scheduler{
		logger:         testLogger(),
		marketCacheTTL: time.Hour,
		stores:         Stores{Market: market},
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.startBackgroundTasks(ctx)

	// Wait until the goroutine is running and blocked inside the refresh.
	select {
	case <-market.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("background refresh goroutine never started")
	}

	// Signal shutdown. The goroutine is still blocked inside RefreshMarketMedians,
	// so bgWG.Wait must not return yet.
	cancel()
	done := make(chan struct{})
	go func() {
		s.bgWG.Wait()
		close(done)
	}()
	select {
	case <-done:
		t.Fatal("bgWG.Wait returned while background refresh was still running")
	case <-time.After(50 * time.Millisecond):
	}

	// Let the refresh finish; Wait must now return promptly.
	close(market.release)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("bgWG.Wait did not return after background goroutine exited")
	}
}

// TestScheduler_StartBackgroundTasks_NoMarketStore verifies the no-op path:
// with no market store, no goroutine is started and Wait returns immediately.
func TestScheduler_StartBackgroundTasks_NoMarketStore(t *testing.T) {
	s := &Scheduler{logger: testLogger(), marketCacheTTL: time.Hour}
	s.startBackgroundTasks(context.Background())

	done := make(chan struct{})
	go func() {
		s.bgWG.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("bgWG.Wait blocked even though no background task was started")
	}
}
