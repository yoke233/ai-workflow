package logctx

import (
	"context"
	"log/slog"
	"strings"
)

type contextAttrsKey struct{}

// WithAttr appends a single slog attribute to context.
func WithAttr(ctx context.Context, attr slog.Attr) context.Context {
	return WithAttrs(ctx, attr)
}

// WithField appends a key/value field to context.
func WithField(ctx context.Context, key string, value any) context.Context {
	trimmed := strings.TrimSpace(key)
	if trimmed == "" {
		return ctx
	}
	return WithAttr(ctx, slog.Any(trimmed, value))
}

// WithAttrs appends attributes to context. Existing attributes are preserved.
func WithAttrs(ctx context.Context, attrs ...slog.Attr) context.Context {
	if len(attrs) == 0 {
		return ctx
	}
	existing := ContextAttrs(ctx)
	merged := make([]slog.Attr, 0, len(existing)+len(attrs))
	merged = append(merged, existing...)
	merged = append(merged, attrs...)
	return context.WithValue(ctx, contextAttrsKey{}, merged)
}

// ContextAttrs returns a copy of attributes stored in context.
func ContextAttrs(ctx context.Context) []slog.Attr {
	if ctx == nil {
		return nil
	}
	raw := ctx.Value(contextAttrsKey{})
	attrs, _ := raw.([]slog.Attr)
	if len(attrs) == 0 {
		return nil
	}
	out := make([]slog.Attr, len(attrs))
	copy(out, attrs)
	return out
}
