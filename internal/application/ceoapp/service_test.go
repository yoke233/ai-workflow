package ceoapp

import (
	"context"
	"testing"

	"github.com/yoke233/zhanggui/internal/application/orchestrateapp"
	"github.com/yoke233/zhanggui/internal/application/requirementapp"
	"github.com/yoke233/zhanggui/internal/core"
)

type fakeRequirementService struct {
	analyzeInput      requirementapp.AnalyzeInput
	createThreadInput requirementapp.CreateThreadInput
	analyzeResult     *requirementapp.AnalyzeResult
	createThreadRes   *requirementapp.CreateThreadResult
}

func (f *fakeRequirementService) Analyze(_ context.Context, input requirementapp.AnalyzeInput) (*requirementapp.AnalyzeResult, error) {
	f.analyzeInput = input
	return f.analyzeResult, nil
}

func (f *fakeRequirementService) CreateThread(_ context.Context, input requirementapp.CreateThreadInput) (*requirementapp.CreateThreadResult, error) {
	f.createThreadInput = input
	return f.createThreadRes, nil
}

type fakeTaskService struct {
	createTaskInput    orchestrateapp.CreateTaskInput
	decomposeTaskInput orchestrateapp.DecomposeTaskInput
	createTaskRes      *orchestrateapp.CreateTaskResult
	decomposeTaskRes   *orchestrateapp.DecomposeTaskResult
}

func (f *fakeTaskService) CreateTask(_ context.Context, input orchestrateapp.CreateTaskInput) (*orchestrateapp.CreateTaskResult, error) {
	f.createTaskInput = input
	return f.createTaskRes, nil
}

func (f *fakeTaskService) DecomposeTask(_ context.Context, input orchestrateapp.DecomposeTaskInput) (*orchestrateapp.DecomposeTaskResult, error) {
	f.decomposeTaskInput = input
	return f.decomposeTaskRes, nil
}

func TestServiceSubmitChoosesDirectExecutionForSimpleRequirement(t *testing.T) {
	t.Parallel()

	requirements := &fakeRequirementService{
		analyzeResult: &requirementapp.AnalyzeResult{
			Analysis: requirementapp.AnalysisResult{
				Summary: "OTP login",
				MatchedProjects: []requirementapp.MatchedProject{
					{ProjectID: 11, ProjectName: "backend"},
				},
				Complexity:           "medium",
				SuggestedMeetingMode: "direct",
			},
		},
	}
	tasks := &fakeTaskService{
		createTaskRes:    &orchestrateapp.CreateTaskResult{WorkItem: &core.WorkItem{ID: 42}, Created: true},
		decomposeTaskRes: &orchestrateapp.DecomposeTaskResult{WorkItemID: 42, ActionCount: 2},
	}
	svc := New(Config{Requirements: requirements, Tasks: tasks})

	result, err := svc.Submit(context.Background(), SubmitInput{
		Description: "Add OTP login support",
		Context:     "Backend only",
		OwnerID:     "alice",
	})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	if result.Mode != ModeDirectExecution {
		t.Fatalf("Mode = %q, want %q", result.Mode, ModeDirectExecution)
	}
	if result.WorkItemID != 42 {
		t.Fatalf("WorkItemID = %d, want 42", result.WorkItemID)
	}
	if result.ActionCount != 2 {
		t.Fatalf("ActionCount = %d, want 2", result.ActionCount)
	}
	if result.NextStep != "run_work_item" {
		t.Fatalf("NextStep = %q, want run_work_item", result.NextStep)
	}
	if requirements.createThreadInput.Description != "" {
		t.Fatalf("unexpected thread creation: %+v", requirements.createThreadInput)
	}
	if tasks.createTaskInput.ProjectID == nil || *tasks.createTaskInput.ProjectID != 11 {
		t.Fatalf("ProjectID = %v, want 11", tasks.createTaskInput.ProjectID)
	}
	if tasks.decomposeTaskInput.WorkItemID != 42 {
		t.Fatalf("DecomposeTask.WorkItemID = %d, want 42", tasks.decomposeTaskInput.WorkItemID)
	}
}

func TestServiceSubmitChoosesDiscussionThreadForCrossProjectRequirement(t *testing.T) {
	t.Parallel()

	requirements := &fakeRequirementService{
		analyzeResult: &requirementapp.AnalyzeResult{
			Analysis: requirementapp.AnalysisResult{
				Summary: "OTP rollout",
				MatchedProjects: []requirementapp.MatchedProject{
					{ProjectID: 11, ProjectName: "backend"},
					{ProjectID: 12, ProjectName: "frontend"},
				},
				Complexity:           "high",
				SuggestedMeetingMode: "group_chat",
			},
			SuggestedThread: requirementapp.SuggestedThread{
				Title: "Discuss OTP rollout",
			},
		},
		createThreadRes: &requirementapp.CreateThreadResult{
			Thread:   &core.Thread{ID: 77, Title: "Discuss OTP rollout"},
			AgentIDs: []string{"lead", "worker"},
		},
	}
	tasks := &fakeTaskService{}
	svc := New(Config{Requirements: requirements, Tasks: tasks})

	result, err := svc.Submit(context.Background(), SubmitInput{
		Description: "Roll out OTP across backend and frontend",
		OwnerID:     "alice",
	})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	if result.Mode != ModeDiscussion {
		t.Fatalf("Mode = %q, want %q", result.Mode, ModeDiscussion)
	}
	if result.Thread == nil || result.Thread.ID != 77 {
		t.Fatalf("Thread = %+v, want ID 77", result.Thread)
	}
	if result.Status != "discussion_started" {
		t.Fatalf("Status = %q, want discussion_started", result.Status)
	}
	if result.NextStep != "discussion_in_thread" {
		t.Fatalf("NextStep = %q, want discussion_in_thread", result.NextStep)
	}
	if tasks.createTaskInput.Title != "" {
		t.Fatalf("unexpected task creation: %+v", tasks.createTaskInput)
	}
}

func TestServiceSubmitChoosesDiscussionThreadForConcurrentMeetingMode(t *testing.T) {
	t.Parallel()

	requirements := &fakeRequirementService{
		analyzeResult: &requirementapp.AnalyzeResult{
			Analysis: requirementapp.AnalysisResult{
				Summary: "Concurrent OTP design",
				MatchedProjects: []requirementapp.MatchedProject{
					{ProjectID: 11, ProjectName: "backend"},
				},
				Complexity:           "medium",
				SuggestedMeetingMode: "concurrent",
			},
			SuggestedThread: requirementapp.SuggestedThread{
				Title: "Concurrent OTP design",
			},
		},
		createThreadRes: &requirementapp.CreateThreadResult{
			Thread: &core.Thread{ID: 88, Title: "Concurrent OTP design"},
		},
	}
	tasks := &fakeTaskService{}
	svc := New(Config{Requirements: requirements, Tasks: tasks})

	result, err := svc.Submit(context.Background(), SubmitInput{
		Description: "Discuss OTP design concurrently",
	})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	if result.Mode != ModeDiscussion {
		t.Fatalf("Mode = %q, want %q", result.Mode, ModeDiscussion)
	}
	if result.Thread == nil || result.Thread.ID != 88 {
		t.Fatalf("Thread = %+v, want ID 88", result.Thread)
	}
	if tasks.createTaskInput.Title != "" {
		t.Fatalf("unexpected direct task creation: %+v", tasks.createTaskInput)
	}
}

func TestServiceSubmitDefaultsOwnerForDiscussionThreads(t *testing.T) {
	t.Parallel()

	requirements := &fakeRequirementService{
		analyzeResult: &requirementapp.AnalyzeResult{
			Analysis: requirementapp.AnalysisResult{
				Summary: "Cross project OTP",
				MatchedProjects: []requirementapp.MatchedProject{
					{ProjectID: 11, ProjectName: "backend"},
					{ProjectID: 12, ProjectName: "frontend"},
				},
				Complexity:           "high",
				SuggestedMeetingMode: "group_chat",
			},
			SuggestedThread: requirementapp.SuggestedThread{
				Title: "Cross project OTP",
			},
		},
		createThreadRes: &requirementapp.CreateThreadResult{
			Thread: &core.Thread{ID: 91, Title: "Cross project OTP"},
		},
	}
	svc := New(Config{Requirements: requirements, Tasks: &fakeTaskService{}})

	_, err := svc.Submit(context.Background(), SubmitInput{
		Description: "Cross project OTP rollout",
	})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	if requirements.createThreadInput.OwnerID != "human" {
		t.Fatalf("OwnerID = %q, want human", requirements.createThreadInput.OwnerID)
	}
}

func TestServiceSubmitReturnsStableStatusPayload(t *testing.T) {
	t.Parallel()

	requirements := &fakeRequirementService{
		analyzeResult: &requirementapp.AnalyzeResult{
			Analysis: requirementapp.AnalysisResult{
				Summary: "Backend OTP",
				MatchedProjects: []requirementapp.MatchedProject{
					{ProjectID: 11, ProjectName: "backend"},
				},
				Complexity:           "low",
				SuggestedMeetingMode: "direct",
			},
		},
	}
	tasks := &fakeTaskService{
		createTaskRes:    &orchestrateapp.CreateTaskResult{WorkItem: &core.WorkItem{ID: 5}, Created: true},
		decomposeTaskRes: &orchestrateapp.DecomposeTaskResult{WorkItemID: 5, ActionCount: 2},
	}
	svc := New(Config{Requirements: requirements, Tasks: tasks})

	result, err := svc.Submit(context.Background(), SubmitInput{Description: "Add backend OTP"})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	if result.Status != "direct_ready" {
		t.Fatalf("Status = %q, want direct_ready", result.Status)
	}
	if result.Analysis == nil || result.Analysis.Summary != "Backend OTP" {
		t.Fatalf("Analysis = %+v", result.Analysis)
	}
}
