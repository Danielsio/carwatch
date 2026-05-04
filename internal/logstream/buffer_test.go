package logstream

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func makeEntry(msg string) LogEntry {
	return LogEntry{Time: time.Now(), Level: "INFO", Message: msg, Component: "test"}
}

func TestNewBuffer_PanicsOnZero(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for zero capacity")
		}
	}()
	NewBuffer(0)
}

func TestNewBuffer_PanicsOnNegative(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for negative capacity")
		}
	}()
	NewBuffer(-1)
}

func TestBuffer_EmptyRecent(t *testing.T) {
	b := NewBuffer(10)
	if got := b.Recent(5); got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
	if b.Len() != 0 {
		t.Fatalf("expected len 0, got %d", b.Len())
	}
}

func TestBuffer_WriteThenRecent(t *testing.T) {
	b := NewBuffer(5)
	for i := range 3 {
		b.Write(makeEntry(fmt.Sprintf("m%d", i)))
	}

	if b.Len() != 3 {
		t.Fatalf("expected len 3, got %d", b.Len())
	}

	got := b.Recent(2)
	if len(got) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(got))
	}
	if got[0].Message != "m1" || got[1].Message != "m2" {
		t.Fatalf("unexpected entries: %v", got)
	}
}

func TestBuffer_RecentAll(t *testing.T) {
	b := NewBuffer(5)
	for i := range 3 {
		b.Write(makeEntry(fmt.Sprintf("m%d", i)))
	}

	got := b.Recent(0)
	if len(got) != 3 {
		t.Fatalf("expected 3 entries for Recent(0), got %d", len(got))
	}
}

func TestBuffer_Wrap(t *testing.T) {
	b := NewBuffer(3)
	for i := range 5 {
		b.Write(makeEntry(fmt.Sprintf("m%d", i)))
	}

	if b.Len() != 3 {
		t.Fatalf("expected len 3 after wrap, got %d", b.Len())
	}

	got := b.Recent(0)
	if len(got) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(got))
	}
	// Should have m2, m3, m4 (oldest two overwritten)
	for i, want := range []string{"m2", "m3", "m4"} {
		if got[i].Message != want {
			t.Errorf("entry[%d]: got %q, want %q", i, got[i].Message, want)
		}
	}
}

func TestBuffer_RecentExceedsStored(t *testing.T) {
	b := NewBuffer(10)
	b.Write(makeEntry("only"))
	got := b.Recent(100)
	if len(got) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(got))
	}
}

func TestBuffer_Concurrent(t *testing.T) {
	b := NewBuffer(100)
	var wg sync.WaitGroup
	for g := range 10 {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := range 50 {
				b.Write(makeEntry(fmt.Sprintf("g%d-m%d", id, i)))
			}
		}(g)
	}

	// Read while writes are happening
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range 20 {
			_ = b.Recent(10)
		}
	}()

	wg.Wait()
	if b.Len() != 100 {
		t.Fatalf("expected len 100, got %d", b.Len())
	}
}
