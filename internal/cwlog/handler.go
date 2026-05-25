package cwlog

import (
	"context"
	"log/slog"
)

// ContextHandler is an slog.Handler that extracts well-known values from
// context.Context and injects them as record attributes before forwarding
// to its inner handler. It composes with any slog.Handler (TeeHandler,
// JSONHandler, etc.).
type ContextHandler struct {
	inner slog.Handler
}

// NewContextHandler wraps inner with automatic context-key extraction.
func NewContextHandler(inner slog.Handler) *ContextHandler {
	return &ContextHandler{inner: inner}
}

func (h *ContextHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *ContextHandler) Handle(ctx context.Context, r slog.Record) error {
	if v, ok := ctx.Value(keyCycleID).(uint64); ok && v != 0 {
		r.AddAttrs(slog.Uint64("cycle_id", v))
	}
	if v, ok := ctx.Value(keySearchID).(int64); ok && v != 0 {
		r.AddAttrs(slog.Int64("search_id", v))
	}
	if v, ok := ctx.Value(keyChatID).(int64); ok && v != 0 {
		r.AddAttrs(slog.Int64("chat_id", v))
	}
	if v, ok := ctx.Value(keyRequestID).(string); ok && v != "" {
		r.AddAttrs(slog.String("request_id", v))
	}
	if v, ok := ctx.Value(keyComponent).(string); ok && v != "" {
		r.AddAttrs(slog.String("component", v))
	}
	return h.inner.Handle(ctx, r)
}

func (h *ContextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &ContextHandler{inner: h.inner.WithAttrs(attrs)}
}

func (h *ContextHandler) WithGroup(name string) slog.Handler {
	return &ContextHandler{inner: h.inner.WithGroup(name)}
}
