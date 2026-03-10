package engine

import (
	"strings"
	"testing"

	"github.com/yoke233/ai-workflow/internal/v2/core"
)

func TestRenderBriefingSnapshot_IncludesContextRefs(t *testing.T) {
	briefing := &core.Briefing{
		Objective: "Review and update the implementation.",
		ContextRefs: []core.ContextRef{
			{
				Type:   core.CtxUpstreamArtifact,
				RefID:  101,
				Label:  "implement output",
				Inline: "Ran `go test ./...` and updated README.md.",
			},
			{
				Type:   core.CtxUpstreamArtifact,
				RefID:  102,
				Inline: "Opened PR #12.",
			},
		},
	}

	got := renderBriefingSnapshot(briefing)
	if !strings.Contains(got, "Review and update the implementation.") {
		t.Fatalf("expected objective in snapshot, got: %q", got)
	}
	if !strings.Contains(got, "# Context") {
		t.Fatalf("expected context header in snapshot, got: %q", got)
	}
	if !strings.Contains(got, "## implement output") {
		t.Fatalf("expected explicit context label, got: %q", got)
	}
	if !strings.Contains(got, "Ran `go test ./...` and updated README.md.") {
		t.Fatalf("expected upstream artifact inline content, got: %q", got)
	}
	if !strings.Contains(got, "## upstream_artifact:102") {
		t.Fatalf("expected fallback context label, got: %q", got)
	}
}

func TestRenderBriefingSnapshot_RespectsTotalBudget(t *testing.T) {
	longObjective := strings.Repeat("o", maxBriefingTotalChars+500)
	longRef := strings.Repeat("x", maxBriefingRefChars+500)
	briefing := &core.Briefing{
		Objective: longObjective,
		ContextRefs: []core.ContextRef{
			{
				Type:   core.CtxUpstreamArtifact,
				RefID:  201,
				Label:  "upstream",
				Inline: longRef,
			},
		},
	}

	got := renderBriefingSnapshot(briefing)
	if len(got) > maxBriefingTotalChars {
		t.Fatalf("snapshot length=%d exceeds budget=%d", len(got), maxBriefingTotalChars)
	}
	if strings.Contains(got, "# Context") {
		t.Fatalf("expected no context when objective already exhausted budget, got length=%d", len(got))
	}
	if !strings.Contains(got, "[truncated]") {
		t.Fatalf("expected truncated marker in snapshot, got: %q", got)
	}
}
