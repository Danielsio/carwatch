package logstream

import (
	"testing"
	"time"
)

func TestHub_PublishAndSubscribe(t *testing.T) {
	h := NewHub(100)
	ch, unsub := h.Subscribe()
	defer unsub()

	e := LogEntry{
		Time:      time.Now(),
		Level:     "INFO",
		Message:   "hello",
		Component: "yad2",
	}
	h.Publish(e)

	select {
	case got := <-ch:
		if got.Message != "hello" {
			t.Fatalf("got message %q, want %q", got.Message, "hello")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for published entry")
	}
}

func TestHub_Unsubscribe(t *testing.T) {
	h := NewHub(100)
	ch, unsub := h.Subscribe()
	unsub()

	h.Publish(makeEntry("after-unsub"))

	select {
	case <-ch:
		t.Fatal("should not receive after unsubscribe")
	case <-time.After(50 * time.Millisecond):
		// expected
	}
}

func TestHub_MultipleSubscribers(t *testing.T) {
	h := NewHub(100)
	ch1, unsub1 := h.Subscribe()
	defer unsub1()
	ch2, unsub2 := h.Subscribe()
	defer unsub2()

	h.Publish(makeEntry("multi"))

	for i, ch := range []<-chan LogEntry{ch1, ch2} {
		select {
		case got := <-ch:
			if got.Message != "multi" {
				t.Errorf("subscriber %d: got %q, want %q", i, got.Message, "multi")
			}
		case <-time.After(time.Second):
			t.Errorf("subscriber %d: timed out", i)
		}
	}
}

func TestHub_Recent(t *testing.T) {
	h := NewHub(10)
	h.Publish(makeEntry("a"))
	h.Publish(makeEntry("b"))

	got := h.Recent(5)
	if len(got) != 2 {
		t.Fatalf("expected 2, got %d", len(got))
	}
}

func TestHub_SlowSubscriberDrops(t *testing.T) {
	h := NewHub(100)
	ch, unsub := h.Subscribe()
	defer unsub()

	// Fill the subscriber channel buffer (capacity 64)
	for i := range 100 {
		h.Publish(makeEntry(time.Now().String() + string(rune(i))))
	}

	// Should have received up to buffer cap, rest dropped
	count := 0
	for {
		select {
		case <-ch:
			count++
		default:
			goto done
		}
	}
done:
	if count != 64 {
		t.Fatalf("expected 64 buffered entries, got %d", count)
	}
}
