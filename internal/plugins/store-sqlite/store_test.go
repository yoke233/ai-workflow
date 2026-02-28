package storesqlite

import (
	"testing"

	"github.com/user/ai-workflow/internal/core"
)

func TestProjectCRUD(t *testing.T) {
	s, err := New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	p := &core.Project{ID: "test-1", Name: "Test", RepoPath: "/tmp/test"}
	if err := s.CreateProject(p); err != nil {
		t.Fatal(err)
	}

	got, err := s.GetProject("test-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Test" {
		t.Errorf("expected Test, got %s", got.Name)
	}

	got.Name = "Updated"
	if err := s.UpdateProject(got); err != nil {
		t.Fatal(err)
	}

	got2, _ := s.GetProject("test-1")
	if got2.Name != "Updated" {
		t.Errorf("expected Updated, got %s", got2.Name)
	}

	if err := s.DeleteProject("test-1"); err != nil {
		t.Fatal(err)
	}
	_, err = s.GetProject("test-1")
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestPipelineSaveAndGet(t *testing.T) {
	s, err := New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	_ = s.CreateProject(&core.Project{ID: "proj-1", Name: "P", RepoPath: "/tmp/p"})

	pipe := &core.Pipeline{
		ID:        "20260228-aabbccddeeff",
		ProjectID: "proj-1",
		Name:      "test-pipe",
		Template:  "standard",
		Status:    core.StatusCreated,
		Stages:    []core.StageConfig{{Name: core.StageImplement, Agent: "claude"}},
		Artifacts: map[string]string{},

		MaxTotalRetries: 5,
	}
	if err := s.SavePipeline(pipe); err != nil {
		t.Fatal(err)
	}

	got, err := s.GetPipeline("20260228-aabbccddeeff")
	if err != nil {
		t.Fatal(err)
	}
	if got.Template != "standard" {
		t.Errorf("expected standard, got %s", got.Template)
	}
}
