package main

import (
	"strings"
	"testing"
)

func TestCLI_PipelineActionCommand(t *testing.T) {
	err := runWithArgs([]string{"pipeline", "action"})
	if err == nil {
		t.Fatal("expected usage error for missing pipeline action args")
	}
	if !strings.Contains(err.Error(), "usage: ai-flow pipeline action") {
		t.Fatalf("expected pipeline action usage error, got %v", err)
	}
}

func TestCLI_SchedulerCommand(t *testing.T) {
	err := runWithArgs([]string{"scheduler"})
	if err == nil {
		t.Fatal("expected usage error for missing scheduler subcommand")
	}
	if !strings.Contains(err.Error(), "usage: ai-flow scheduler <run|once>") {
		t.Fatalf("expected scheduler usage error, got %v", err)
	}
}

func TestCLI_ProjectScanCommand(t *testing.T) {
	err := runWithArgs([]string{"project", "scan"})
	if err == nil {
		t.Fatal("expected usage error for missing project scan root")
	}
	if !strings.Contains(err.Error(), "usage: ai-flow project scan <root>") {
		t.Fatalf("expected project scan usage error, got %v", err)
	}
}
