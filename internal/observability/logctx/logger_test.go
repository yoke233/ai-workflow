package logctx

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/user/ai-workflow/internal/observability"
)

func TestLogger_InjectsContextAttrs(t *testing.T) {
	var buf bytes.Buffer
	logger := New(Options{
		Writer: &buf,
		Format: "json",
		Level:  slog.LevelDebug,
	})

	ctx := context.Background()
	ctx = WithField(ctx, "conn_id", "conn-1")
	ctx = WithField(ctx, "session_id", "sess-1")

	logger.InfoContext(ctx, "hello", "event", "acp.test")

	line := strings.TrimSpace(buf.String())
	if line == "" {
		t.Fatal("expected log output")
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(line), &payload); err != nil {
		t.Fatalf("unmarshal log json: %v", err)
	}
	if payload["conn_id"] != "conn-1" {
		t.Fatalf("expected conn_id=conn-1, got %v", payload["conn_id"])
	}
	if payload["session_id"] != "sess-1" {
		t.Fatalf("expected session_id=sess-1, got %v", payload["session_id"])
	}
	if payload["event"] != "acp.test" {
		t.Fatalf("expected event=acp.test, got %v", payload["event"])
	}
}

func TestLogger_DefaultTraceExtractor(t *testing.T) {
	var buf bytes.Buffer
	logger := New(Options{
		Writer: &buf,
		Format: "json",
	})

	ctx := observability.WithTraceID(context.Background(), "trace-test-1")
	logger.InfoContext(ctx, "trace check")

	line := strings.TrimSpace(buf.String())
	if line == "" {
		t.Fatal("expected log output")
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(line), &payload); err != nil {
		t.Fatalf("unmarshal log json: %v", err)
	}
	if payload["trace_id"] != "trace-test-1" {
		t.Fatalf("expected trace_id=trace-test-1, got %v", payload["trace_id"])
	}
}

func TestParseLevel(t *testing.T) {
	cases := []struct {
		in      string
		wantErr bool
		want    slog.Level
	}{
		{in: "", want: slog.LevelInfo},
		{in: "info", want: slog.LevelInfo},
		{in: "DEBUG", want: slog.LevelDebug},
		{in: "warn", want: slog.LevelWarn},
		{in: "error", want: slog.LevelError},
		{in: "bad", wantErr: true},
	}

	for _, tc := range cases {
		got, err := ParseLevel(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Fatalf("ParseLevel(%q): expected error", tc.in)
			}
			continue
		}
		if err != nil {
			t.Fatalf("ParseLevel(%q): unexpected error: %v", tc.in, err)
		}
		if got != tc.want {
			t.Fatalf("ParseLevel(%q)=%v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestLogger_CustomExtractor(t *testing.T) {
	var buf bytes.Buffer
	logger := New(Options{
		Writer: &buf,
		Format: "json",
		Extractors: []ContextExtractor{
			func(context.Context) []slog.Attr {
				return []slog.Attr{slog.String("tenant_id", "tenant-7")}
			},
		},
	})

	logger.InfoContext(context.Background(), "custom extractor")

	line := strings.TrimSpace(buf.String())
	if line == "" {
		t.Fatal("expected log output")
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(line), &payload); err != nil {
		t.Fatalf("unmarshal log json: %v", err)
	}
	if payload["tenant_id"] != "tenant-7" {
		t.Fatalf("expected tenant_id=tenant-7, got %v", payload["tenant_id"])
	}
}
