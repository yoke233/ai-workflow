package logctx

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/user/ai-workflow/internal/observability"
)

// ContextExtractor extracts context-level structured fields.
type ContextExtractor func(ctx context.Context) []slog.Attr

// Options configures logger construction.
type Options struct {
	Writer      io.Writer
	Format      string // text|json
	Level       slog.Leveler
	AddSource   bool
	ReplaceAttr func(groups []string, a slog.Attr) slog.Attr
	BaseAttrs   []slog.Attr
	Extractors  []ContextExtractor

	// WrapHandler allows future integration (e.g. fanout, sampling, shipping).
	// It receives the base handler and returns the wrapped handler.
	WrapHandler func(slog.Handler) slog.Handler
}

// ParseLevel parses log level string into slog.Level.
func ParseLevel(raw string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "info":
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("unsupported log level: %q", raw)
	}
}

// New creates a context-aware slog logger.
func New(opts Options) *slog.Logger {
	writer := opts.Writer
	if writer == nil {
		writer = os.Stderr
	}
	level := opts.Level
	if level == nil {
		level = slog.LevelInfo
	}

	baseOptions := &slog.HandlerOptions{
		AddSource:   opts.AddSource,
		Level:       level,
		ReplaceAttr: opts.ReplaceAttr,
	}

	format := strings.ToLower(strings.TrimSpace(opts.Format))
	var handler slog.Handler
	switch format {
	case "", "text":
		handler = slog.NewTextHandler(writer, baseOptions)
	case "json":
		handler = slog.NewJSONHandler(writer, baseOptions)
	default:
		// Keep behavior resilient for callers who pass unknown values.
		handler = slog.NewTextHandler(writer, baseOptions)
	}

	if len(opts.BaseAttrs) > 0 {
		handler = handler.WithAttrs(opts.BaseAttrs)
	}
	if opts.WrapHandler != nil {
		handler = opts.WrapHandler(handler)
	}

	return slog.New(&contextHandler{
		next:       handler,
		extractors: defaultExtractors(opts.Extractors),
	})
}

// TraceIDExtractor injects trace_id from observability trace context.
func TraceIDExtractor() ContextExtractor {
	return func(ctx context.Context) []slog.Attr {
		traceID := observability.TraceID(ctx)
		if traceID == "" {
			return nil
		}
		return []slog.Attr{slog.String("trace_id", traceID)}
	}
}

func defaultExtractors(extra []ContextExtractor) []ContextExtractor {
	out := make([]ContextExtractor, 0, len(extra)+1)
	out = append(out, TraceIDExtractor())
	out = append(out, extra...)
	return out
}

type contextHandler struct {
	next       slog.Handler
	extractors []ContextExtractor
}

func (h *contextHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *contextHandler) Handle(ctx context.Context, r slog.Record) error {
	clone := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
	r.Attrs(func(a slog.Attr) bool {
		clone.AddAttrs(a)
		return true
	})

	if attrs := ContextAttrs(ctx); len(attrs) > 0 {
		clone.AddAttrs(attrs...)
	}
	for _, extractor := range h.extractors {
		if extractor == nil {
			continue
		}
		attrs := extractor(ctx)
		if len(attrs) == 0 {
			continue
		}
		clone.AddAttrs(attrs...)
	}

	return h.next.Handle(ctx, clone)
}

func (h *contextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &contextHandler{
		next:       h.next.WithAttrs(attrs),
		extractors: h.extractors,
	}
}

func (h *contextHandler) WithGroup(name string) slog.Handler {
	return &contextHandler{
		next:       h.next.WithGroup(name),
		extractors: h.extractors,
	}
}
