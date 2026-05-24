package logstream

import "sync"

// Hub fans out log entries to SSE subscribers and stores them in a ring buffer.
type Hub struct {
	buf  *Buffer
	mu   sync.RWMutex
	subs map[*subscriber]struct{}
}

type subscriber struct {
	ch chan LogEntry
}

// NewHub creates a hub with a ring buffer of the given capacity.
func NewHub(bufCap int) *Hub {
	return &Hub{
		buf:  NewBuffer(bufCap),
		subs: make(map[*subscriber]struct{}),
	}
}

// Publish writes an entry to the buffer and broadcasts it to all subscribers.
func (h *Hub) Publish(e LogEntry) {
	h.buf.Write(e)

	h.mu.RLock()
	defer h.mu.RUnlock()
	for s := range h.subs {
		select {
		case s.ch <- e:
		default:
			// subscriber too slow, drop
		}
	}
}

// Subscribe returns a channel that receives new entries and an unsubscribe function.
// The caller must call unsubscribe when done.
func (h *Hub) Subscribe() (<-chan LogEntry, func()) {
	s := &subscriber{ch: make(chan LogEntry, 64)}
	h.mu.Lock()
	h.subs[s] = struct{}{}
	h.mu.Unlock()

	unsub := func() {
		h.mu.Lock()
		delete(h.subs, s)
		h.mu.Unlock()
	}
	return s.ch, unsub
}

// Recent returns the last n entries from the ring buffer.
func (h *Hub) Recent(n int) []LogEntry {
	return h.buf.Recent(n)
}
