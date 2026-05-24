package logstream

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// TeeHandler wraps an slog.Handler, forwarding all records to it while also
// capturing records whose "component" attribute matches the allow-list and
// publishing them to a Hub.
type TeeHandler struct {
	inner      slog.Handler
	hub        *Hub
	components map[string]bool
	// preAttrs are attributes added via WithAttrs
	preAttrs []slog.Attr
	// group prefix from WithGroup
	groups []string
}

// NewTeeHandler wraps inner and publishes matching records to hub.
// Only records with a "component" attr matching one of the given components
// are captured; all records are forwarded to inner regardless.
func NewTeeHandler(inner slog.Handler, hub *Hub, components ...string) *TeeHandler {
	m := make(map[string]bool, len(components))
	for _, c := range components {
		m[c] = true
	}
	return &TeeHandler{
		inner:      inner,
		hub:        hub,
		components: m,
	}
}

func (h *TeeHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *TeeHandler) Handle(ctx context.Context, r slog.Record) error {
	component := h.componentFromPre()
	attrs := make(map[string]string)

	groupPrefix := ""
	if len(h.groups) > 0 {
		groupPrefix = strings.Join(h.groups, ".") + "."
	}

	// Collect pre-set attrs (from WithAttrs).
	// "component" is a well-known key used for filtering, never prefixed.
	for _, a := range h.preAttrs {
		if a.Key == "component" {
			component = a.Value.String()
		} else {
			key := a.Key
			if groupPrefix != "" {
				key = groupPrefix + key
			}
			attrs[key] = attrValueString(a.Value)
		}
	}

	// Collect record-level attrs
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == "component" {
			component = a.Value.String()
		} else {
			key := a.Key
			if groupPrefix != "" {
				key = groupPrefix + key
			}
			attrs[key] = attrValueString(a.Value)
		}
		return true
	})

	if h.components[component] {
		ts := r.Time
		if ts.IsZero() {
			ts = time.Now()
		}
		h.hub.Publish(LogEntry{
			Time:      ts,
			Level:     r.Level.String(),
			Message:   r.Message,
			Component: component,
			Attrs:     attrs,
		})
	}

	return h.inner.Handle(ctx, r)
}

func (h *TeeHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newPre := make([]slog.Attr, len(h.preAttrs), len(h.preAttrs)+len(attrs))
	copy(newPre, h.preAttrs)
	newPre = append(newPre, attrs...)
	return &TeeHandler{
		inner:      h.inner.WithAttrs(attrs),
		hub:        h.hub,
		components: h.components,
		preAttrs:   newPre,
		groups:     h.groups,
	}
}

func (h *TeeHandler) WithGroup(name string) slog.Handler {
	newGroups := make([]string, len(h.groups), len(h.groups)+1)
	copy(newGroups, h.groups)
	newGroups = append(newGroups, name)
	return &TeeHandler{
		inner:      h.inner.WithGroup(name),
		hub:        h.hub,
		components: h.components,
		preAttrs:   h.preAttrs,
		groups:     newGroups,
	}
}

func (h *TeeHandler) componentFromPre() string {
	for _, a := range h.preAttrs {
		if a.Key == "component" {
			return a.Value.String()
		}
	}
	return ""
}

func attrValueString(v slog.Value) string {
	if v.Kind() == slog.KindTime {
		return v.Time().Format(time.RFC3339)
	}
	return fmt.Sprint(v.Any())
}
