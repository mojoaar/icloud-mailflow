package logbuf

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"
)

// Entry is a captured warning/error log record.
type Entry struct {
	Time    time.Time
	Level   slog.Level
	Message string
	Attrs   string
}

// Buffer is a fixed-capacity ring buffer of recent warning/error entries.
type Buffer struct {
	mu      sync.Mutex
	entries []Entry
	max     int
}

func New(max int) *Buffer {
	return &Buffer{max: max}
}

func (b *Buffer) Add(e Entry) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.entries = append(b.entries, e)
	if len(b.entries) > b.max {
		b.entries = b.entries[len(b.entries)-b.max:]
	}
}

// Entries returns a copy of the buffer, newest first.
func (b *Buffer) Entries() []Entry {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]Entry, len(b.entries))
	for i, e := range b.entries {
		out[len(b.entries)-1-i] = e
	}
	return out
}

func (b *Buffer) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.entries = nil
}

type handler struct {
	next slog.Handler
	buf  *Buffer
}

// NewHandler wraps next and records warning/error records into buf.
func NewHandler(next slog.Handler, buf *Buffer) slog.Handler {
	return &handler{next: next, buf: buf}
}

func (h *handler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *handler) Handle(ctx context.Context, r slog.Record) error {
	if r.Level >= slog.LevelWarn {
		var sb strings.Builder
		r.Attrs(func(a slog.Attr) bool {
			if sb.Len() > 0 {
				sb.WriteString(" ")
			}
			sb.WriteString(a.String())
			return true
		})
		h.buf.Add(Entry{
			Time:    r.Time,
			Level:   r.Level,
			Message: r.Message,
			Attrs:   sb.String(),
		})
	}
	return h.next.Handle(ctx, r)
}

func (h *handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &handler{next: h.next.WithAttrs(attrs), buf: h.buf}
}

func (h *handler) WithGroup(name string) slog.Handler {
	return &handler{next: h.next.WithGroup(name), buf: h.buf}
}
