package logbuf

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"
)

func TestBufferEviction(t *testing.T) {
	b := New(3)
	for i := 0; i < 5; i++ {
		b.Add(Entry{Message: string(rune('0' + i))})
	}
	entries := b.Entries()
	if len(entries) != 3 {
		t.Fatalf("len = %d, want 3", len(entries))
	}
	// newest first
	if entries[0].Message != "4" || entries[1].Message != "3" || entries[2].Message != "2" {
		t.Errorf("wrong entries: %v", entries)
	}
}

func TestBufferClear(t *testing.T) {
	b := New(5)
	b.Add(Entry{Message: "x"})
	b.Clear()
	if got := len(b.Entries()); got != 0 {
		t.Errorf("len after clear = %d, want 0", got)
	}
}

func TestHandlerCapturesWarn(t *testing.T) {
	buf := New(10)
	h := NewHandler(slog.NewTextHandler(io.Discard, nil), buf)

	_ = h.Handle(context.Background(), slog.NewRecord(time.Now(), slog.LevelInfo, "info", 0))
	_ = h.Handle(context.Background(), slog.NewRecord(time.Now(), slog.LevelWarn, "warn", 0))
	_ = h.Handle(context.Background(), slog.NewRecord(time.Now(), slog.LevelError, "error", 0))

	entries := buf.Entries()
	if len(entries) != 2 {
		t.Fatalf("len = %d, want 2", len(entries))
	}
	if entries[0].Message != "error" || entries[1].Message != "warn" {
		t.Errorf("entries = %v", entries)
	}
}
