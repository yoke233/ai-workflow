package mcpserver

import (
	"testing"
)

func TestResolveContextScope_TeamLeader(t *testing.T) {
	scope := ResolveContextScope("team_leader", "proj-42", "issue-7")

	wantRead := []string{
		"viking://resources/proj-42/docs/",
		"viking://resources/shared/",
	}
	wantWrite := []string{
		"viking://resources/proj-42/specs/",
		"viking://resources/proj-42/docs/",
	}
	assertPrefixes(t, "ReadPrefixes", scope.ReadPrefixes, wantRead)
	assertPrefixes(t, "WritePrefixes", scope.WritePrefixes, wantWrite)
}

func TestResolveContextScope_Reviewer(t *testing.T) {
	scope := ResolveContextScope("reviewer", "proj-42", "issue-7")

	wantRead := []string{
		"viking://resources/proj-42/specs/issue-7/",
		"viking://resources/proj-42/docs/",
	}
	assertPrefixes(t, "ReadPrefixes", scope.ReadPrefixes, wantRead)
	assertPrefixes(t, "WritePrefixes", scope.WritePrefixes, nil)
}

func TestResolveContextScope_Decomposer(t *testing.T) {
	scope := ResolveContextScope("decomposer", "proj-42", "issue-7")

	wantRead := []string{
		"viking://resources/proj-42/specs/issue-7/",
		"viking://resources/proj-42/docs/",
	}
	wantWrite := []string{
		"viking://resources/proj-42/specs/",
	}
	assertPrefixes(t, "ReadPrefixes", scope.ReadPrefixes, wantRead)
	assertPrefixes(t, "WritePrefixes", scope.WritePrefixes, wantWrite)
}

func TestResolveContextScope_Worker(t *testing.T) {
	scope := ResolveContextScope("worker", "proj-42", "issue-7")

	wantRead := []string{
		"viking://resources/proj-42/specs/issue-7/",
	}
	assertPrefixes(t, "ReadPrefixes", scope.ReadPrefixes, wantRead)
	assertPrefixes(t, "WritePrefixes", scope.WritePrefixes, nil)
}

func TestResolveContextScope_Aggregator(t *testing.T) {
	scope := ResolveContextScope("aggregator", "proj-42", "issue-7")

	wantRead := []string{
		"viking://resources/proj-42/specs/",
		"viking://resources/proj-42/archive/",
	}
	wantWrite := []string{
		"viking://resources/proj-42/specs/",
		"viking://resources/proj-42/archive/",
	}
	assertPrefixes(t, "ReadPrefixes", scope.ReadPrefixes, wantRead)
	assertPrefixes(t, "WritePrefixes", scope.WritePrefixes, wantWrite)
}

func TestResolveContextScope_UnknownRole(t *testing.T) {
	scope := ResolveContextScope("unknown_role", "proj-42", "issue-7")

	if len(scope.ReadPrefixes) != 0 {
		t.Errorf("expected empty ReadPrefixes, got %v", scope.ReadPrefixes)
	}
	if len(scope.WritePrefixes) != 0 {
		t.Errorf("expected empty WritePrefixes, got %v", scope.WritePrefixes)
	}
}

func assertPrefixes(t *testing.T, label string, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("%s: got %d prefixes %v, want %d %v", label, len(got), got, len(want), want)
		return
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("%s[%d]: got %q, want %q", label, i, got[i], want[i])
		}
	}
}
