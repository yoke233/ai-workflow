package main

import (
	"strings"
	"testing"
)

func TestParseExecutorArgs_SupportsEqualsSyntax(t *testing.T) {
	opts, err := parseExecutorArgs([]string{
		"--nats-url=nats://127.0.0.1:4222",
		"--agents=claude,codex",
		"--max-concurrent=4",
	})
	if err != nil {
		t.Fatalf("parseExecutorArgs returned error: %v", err)
	}
	if opts.natsURL != "nats://127.0.0.1:4222" {
		t.Fatalf("natsURL = %q, want %q", opts.natsURL, "nats://127.0.0.1:4222")
	}
	if len(opts.agentTypes) != 2 || opts.agentTypes[0] != "claude" || opts.agentTypes[1] != "codex" {
		t.Fatalf("agentTypes = %#v, want [claude codex]", opts.agentTypes)
	}
	if opts.maxConcurrent != 4 {
		t.Fatalf("maxConcurrent = %d, want 4", opts.maxConcurrent)
	}
}

func TestParseExecutorArgs_InvalidMaxConcurrent(t *testing.T) {
	_, err := parseExecutorArgs([]string{"--max-concurrent=0"})
	if err == nil {
		t.Fatal("expected error for invalid max-concurrent")
	}
	if !strings.Contains(err.Error(), "invalid value for --max-concurrent") {
		t.Fatalf("unexpected error: %v", err)
	}
}
