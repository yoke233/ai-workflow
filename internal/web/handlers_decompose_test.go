package web

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/yoke233/ai-workflow/internal/core"
	"github.com/yoke233/ai-workflow/internal/teamleader"
)

type stubDecomposePlanner struct {
	planFn func(ctx context.Context, projectID, prompt string) (*core.DecomposeProposal, error)
}

func (s *stubDecomposePlanner) Plan(ctx context.Context, projectID, prompt string) (*core.DecomposeProposal, error) {
	return s.planFn(ctx, projectID, prompt)
}

type stubProposalIssueCreator struct {
	createIssuesFn func(ctx context.Context, input teamleader.CreateIssuesInput) ([]*core.Issue, error)
}

func (s *stubProposalIssueCreator) CreateIssues(ctx context.Context, input teamleader.CreateIssuesInput) ([]*core.Issue, error) {
	return s.createIssuesFn(ctx, input)
}

func TestDecomposeAPI_ReturnsProposal(t *testing.T) {
	store := newTestStore(t)
	project := core.Project{ID: "proj-decompose", Name: "proj-decompose", RepoPath: filepath.Join(t.TempDir(), "repo")}
	if err := store.CreateProject(&project); err != nil {
		t.Fatalf("seed project: %v", err)
	}

	srv := NewServer(Config{
		Store: store,
		DecomposePlanner: &stubDecomposePlanner{planFn: func(_ context.Context, projectID, prompt string) (*core.DecomposeProposal, error) {
			if projectID != project.ID {
				t.Fatalf("projectID = %q", projectID)
			}
			if prompt != "?????" {
				t.Fatalf("prompt = %q", prompt)
			}
			return &core.DecomposeProposal{
				ID:        projectID + "-prop",
				ProjectID: projectID,
				Prompt:    prompt,
				Summary:   "??????",
				Items:     []core.ProposalItem{{TempID: "A", Title: "?? schema"}},
			}, nil
		}},
	})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	rawBody, _ := json.Marshal(map[string]any{"prompt": "?????"})
	resp, err := http.Post(ts.URL+"/api/v1/projects/"+project.ID+"/decompose", "application/json", bytes.NewReader(rawBody))
	if err != nil {
		t.Fatalf("POST decompose: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var proposal core.DecomposeProposal
	if err := json.NewDecoder(resp.Body).Decode(&proposal); err != nil {
		t.Fatalf("decode proposal: %v", err)
	}
	if proposal.ProjectID != project.ID {
		t.Fatalf("proposal project_id = %q", proposal.ProjectID)
	}
	if len(proposal.Items) != 1 || proposal.Items[0].TempID != "A" {
		t.Fatalf("proposal items = %#v", proposal.Items)
	}
}

func TestConfirmDecomposeAPI_ResolvesDependenciesViaCreator(t *testing.T) {
	store := newTestStore(t)
	project := core.Project{ID: "proj-confirm", Name: "proj-confirm", RepoPath: filepath.Join(t.TempDir(), "repo")}
	if err := store.CreateProject(&project); err != nil {
		t.Fatalf("seed project: %v", err)
	}

	creatorCalled := false
	srv := NewServer(Config{
		Store: store,
		ProposalIssueCreator: &stubProposalIssueCreator{createIssuesFn: func(_ context.Context, input teamleader.CreateIssuesInput) ([]*core.Issue, error) {
			creatorCalled = true
			if input.ProjectID != project.ID {
				t.Fatalf("projectID = %q", input.ProjectID)
			}
			if len(input.Issues) != 2 {
				t.Fatalf("issues len = %d", len(input.Issues))
			}
			if got := input.Issues[1].DependsOn; len(got) != 1 || got[0] != "issue-a" {
				t.Fatalf("resolved depends_on = %#v", got)
			}
			return []*core.Issue{
				{ID: "issue-a", Title: input.Issues[0].Title},
				{ID: "issue-b", Title: input.Issues[1].Title},
			}, nil
		}},
	})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	rawBody, _ := json.Marshal(map[string]any{
		"proposal_id": "prop-1",
		"issues": []map[string]any{
			{"temp_id": "A", "title": "?? schema", "body": "", "depends_on": []string{}, "labels": []string{}},
			{"temp_id": "B", "title": "?? API", "body": "", "depends_on": []string{"A"}, "labels": []string{}},
		},
		"issue_ids": map[string]string{"A": "issue-a", "B": "issue-b"},
	})
	resp, err := http.Post(ts.URL+"/api/v1/projects/"+project.ID+"/decompose/confirm", "application/json", bytes.NewReader(rawBody))
	if err != nil {
		t.Fatalf("POST confirm: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	if !creatorCalled {
		t.Fatal("expected proposal issue creator to be called")
	}
	var body struct {
		CreatedIssues []struct {
			TempID  string `json:"temp_id"`
			IssueID string `json:"issue_id"`
		} `json:"created_issues"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.CreatedIssues) != 2 || body.CreatedIssues[1].IssueID != "issue-b" {
		t.Fatalf("created issues = %#v", body.CreatedIssues)
	}
}
