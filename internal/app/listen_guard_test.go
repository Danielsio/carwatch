package app

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/dsionov/carwatch/internal/health"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// A failed bind must be reported, not merely logged: a service whose listener
// is dead serves nothing, yet `restart: unless-stopped` never restarts a
// merely-unhealthy container, so the process would linger as a zombie.
func TestBuildHealthServer_OccupiedPortReportsListenError(t *testing.T) {
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	defer func() { _ = occupied.Close() }()

	srv, errCh := BuildHealthServer(occupied.Addr().String(), health.New(), discardLogger())
	defer func() { _ = srv.Close() }()

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected a listen error on an occupied port, got nil")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("listen error was never reported")
	}
}

// A clean Shutdown closes the channel with no error, so a guard does not
// mistake an orderly stop for a listener failure.
func TestBuildHealthServer_CleanShutdownReportsNoError(t *testing.T) {
	srv, errCh := BuildHealthServer("127.0.0.1:0", health.New(), discardLogger())

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		t.Fatalf("shutdown: %v", err)
	}

	select {
	case err, ok := <-errCh:
		if ok && err != nil {
			t.Fatalf("clean shutdown reported an error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("channel was not closed on clean shutdown")
	}
}

// The guard must unwind the service: a listener failure cancels the context the
// blocking loop (scheduler, consumer, poller) runs under.
func TestGuardListeners_CancelsContextOnListenFailure(t *testing.T) {
	errCh := make(chan error, 1)
	guard := GuardListeners(context.Background(), errCh)
	defer guard.Stop()

	if guard.Context().Err() != nil {
		t.Fatal("context cancelled before any failure")
	}

	want := errors.New("bind: address already in use")
	errCh <- want

	select {
	case <-guard.Context().Done():
	case <-time.After(5 * time.Second):
		t.Fatal("guarded context was not cancelled by the listener failure")
	}
	if got := guard.Err(); !errors.Is(got, want) {
		t.Fatalf("guard.Err() = %v, want %v", got, want)
	}
}

func TestListenGuard_Wrap(t *testing.T) {
	listenErr := errors.New("health server (0.0.0.0:8081): bind failed")
	runErr := errors.New("scheduler exploded")

	t.Run("listener failure wins over the cancellation it caused", func(t *testing.T) {
		errCh := make(chan error, 1)
		errCh <- listenErr
		guard := GuardListeners(context.Background(), errCh)
		defer guard.Stop()
		<-guard.Context().Done()

		// The blocking loop returns context.Canceled because *we* cancelled it;
		// the real cause is the listener, and that is what the process must
		// exit with so Docker restarts it.
		if got := guard.Wrap(context.Canceled); !errors.Is(got, listenErr) {
			t.Fatalf("Wrap(context.Canceled) = %v, want the listen error", got)
		}
	})

	t.Run("signal-driven shutdown is a clean exit", func(t *testing.T) {
		guard := GuardListeners(context.Background())
		defer guard.Stop()

		// Previously every SIGTERM returned context.Canceled up to main, which
		// logged "fatal" and exited 1 — a graceful stop looked like a crash.
		if got := guard.Wrap(context.Canceled); got != nil {
			t.Fatalf("Wrap(context.Canceled) = %v, want nil for a clean shutdown", got)
		}
	})

	t.Run("a real run error still propagates", func(t *testing.T) {
		guard := GuardListeners(context.Background())
		defer guard.Stop()

		if got := guard.Wrap(runErr); !errors.Is(got, runErr) {
			t.Fatalf("Wrap(runErr) = %v, want %v", got, runErr)
		}
	})

	t.Run("nil stays nil", func(t *testing.T) {
		guard := GuardListeners(context.Background())
		defer guard.Stop()

		if got := guard.Wrap(nil); got != nil {
			t.Fatalf("Wrap(nil) = %v, want nil", got)
		}
	})
}
