package observability

import (
	"context"
	"testing"
	"time"
)

func TestTraceContext_FromWebhookDeliveryID(t *testing.T) {
	traceID := TraceIDFromWebhook("", "delivery-abc")
	if traceID != "delivery-abc" {
		t.Fatalf("TraceIDFromWebhook() = %q, want %q", traceID, "delivery-abc")
	}

	ctx := WithTraceID(context.Background(), traceID)
	if got := TraceID(ctx); got != "delivery-abc" {
		t.Fatalf("TraceID(ctx) = %q, want %q", got, "delivery-abc")
	}
}

func TestTraceContext_PropagatesToPipelineEvents(t *testing.T) {
	data := map[string]string{
		"op": "stage_start",
	}
	out := EventDataWithTrace(data, "trace-pipeline-1")
	if out["trace_id"] != "trace-pipeline-1" {
		t.Fatalf("expected trace_id propagated, got %q", out["trace_id"])
	}
	if out["op"] != "stage_start" {
		t.Fatalf("expected existing fields preserved, got %+v", out)
	}
}

func TestStructuredLog_ContainsTraceAndOperation(t *testing.T) {
	fields := StructuredLogFields(StructuredLogInput{
		TraceID:     "trace-log-1",
		ProjectID:   "proj-1",
		PipelineID:  "pipe-1",
		IssueNumber: 42,
		Operation:   "dispatch_webhook",
		Latency:     123 * time.Millisecond,
	})

	if fields["trace_id"] != "trace-log-1" {
		t.Fatalf("expected trace_id field, got %+v", fields)
	}
	if fields["op"] != "dispatch_webhook" {
		t.Fatalf("expected op field, got %+v", fields)
	}
	if fields["latency_ms"] != int64(123) {
		t.Fatalf("expected latency_ms=123, got %+v", fields["latency_ms"])
	}
}
