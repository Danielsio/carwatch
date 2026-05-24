package logstream

import "sync"

// Buffer is a thread-safe circular buffer of log entries.
type Buffer struct {
	mu    sync.RWMutex
	items []LogEntry
	pos   int
	full  bool
	cap   int
}

// NewBuffer creates a ring buffer that holds the last cap entries.
// Panics if cap is not positive.
func NewBuffer(cap int) *Buffer {
	if cap <= 0 {
		panic("logstream: buffer capacity must be positive")
	}
	return &Buffer{
		items: make([]LogEntry, cap),
		cap:   cap,
	}
}

// Write appends an entry, overwriting the oldest if full.
func (b *Buffer) Write(e LogEntry) {
	b.mu.Lock()
	b.items[b.pos] = e
	b.pos = (b.pos + 1) % b.cap
	if !b.full && b.pos == 0 {
		b.full = true
	}
	b.mu.Unlock()
}

// Recent returns the last n entries in chronological order.
// If n <= 0 or exceeds the stored count, all stored entries are returned.
func (b *Buffer) Recent(n int) []LogEntry {
	b.mu.RLock()
	defer b.mu.RUnlock()

	count := b.pos
	if b.full {
		count = b.cap
	}
	if n <= 0 || n > count {
		n = count
	}
	if n == 0 {
		return nil
	}

	out := make([]LogEntry, n)
	start := b.pos - n
	if start < 0 {
		start += b.cap
	}
	for i := range n {
		out[i] = b.items[(start+i)%b.cap]
	}
	return out
}

// Len returns the number of entries currently stored.
func (b *Buffer) Len() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.full {
		return b.cap
	}
	return b.pos
}
