package logger

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"testing"
)

type captureHandler struct {
	mu       sync.Mutex
	messages []string
}

func (h *captureHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }

func (h *captureHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.messages = append(h.messages, r.Message)
	return nil
}

func (h *captureHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }
func (h *captureHandler) WithGroup(_ string) slog.Handler     { return h }

func (h *captureHandler) Messages() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]string, len(h.messages))
	copy(out, h.messages)
	return out
}

func TestPrintln_InsertsSpacesLikeLogPrintln(t *testing.T) {
	h := &captureHandler{}
	prev := base
	base = slog.New(h)
	t.Cleanup(func() { base = prev })

	Println("a", "b")

	msgs := h.Messages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 log record, got %d", len(msgs))
	}
	if msgs[0] != "a b" {
		t.Fatalf("expected message %q, got %q", "a b", msgs[0])
	}
	if strings.HasSuffix(msgs[0], "\n") {
		t.Fatalf("expected no trailing newline, got %q", msgs[0])
	}
}

func TestPrintln_NoArgs_DoesNotLog(t *testing.T) {
	h := &captureHandler{}
	prev := base
	base = slog.New(h)
	t.Cleanup(func() { base = prev })

	Println()

	if len(h.Messages()) != 0 {
		t.Fatalf("expected no log records")
	}
}

