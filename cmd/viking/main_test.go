package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestNormalizeBaseURL(t *testing.T) {
	t.Parallel()

	got := normalizeBaseURL(" http://127.0.0.1:8088/ ")
	if got != "http://127.0.0.1:8088" {
		t.Fatalf("normalizeBaseURL() = %q, want %q", got, "http://127.0.0.1:8088")
	}
}

func TestBuildTargetURIs_Default(t *testing.T) {
	t.Parallel()

	got := buildTargetURIs("proj-a", "chat")
	wantContains := []string{
		"viking://resources/shared/",
		"viking://resources/projects/proj-a/",
		"viking://memory/projects/proj-a/",
	}
	for _, want := range wantContains {
		if !containsString(got, want) {
			t.Fatalf("buildTargetURIs(chat) missing %q, got=%v", want, got)
		}
	}
}

func TestBuildTargetURIs_Backend(t *testing.T) {
	t.Parallel()

	got := buildTargetURIs("proj-a", "implement_backend")
	want := "viking://resources/projects/proj-a/backend/"
	if !containsString(got, want) {
		t.Fatalf("buildTargetURIs(implement_backend) missing %q, got=%v", want, got)
	}
}

func TestRunPlanRequiresProject(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := run([]string{"plan"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error when project missing")
	}
	if !strings.Contains(err.Error(), "project is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunPlanOutput(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := run([]string{"plan", "--project", "proj-a", "--mode", "review", "--role", "reviewer"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run(plan) error: %v", err)
	}

	out := stdout.String()
	checks := []string{
		"project=proj-a mode=review",
		"viking://resources/projects/proj-a/api/",
		"memory_policy: load-only (reader)",
	}
	for _, check := range checks {
		if !strings.Contains(out, check) {
			t.Fatalf("plan output missing %q, output=%q", check, out)
		}
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
