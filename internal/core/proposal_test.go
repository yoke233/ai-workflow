package core

import "testing"

func TestProposalValidate(t *testing.T) {
	valid := DecomposeProposal{
		ID:      "prop-20260309-abcd",
		Summary: "用户注册系统",
		Items: []ProposalItem{
			{TempID: "A", Title: "设计 DB schema", Body: "...", DependsOn: nil},
			{TempID: "B", Title: "实现注册 API", Body: "...", DependsOn: []string{"A"}},
		},
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	empty := valid
	empty.Items = nil
	if err := empty.Validate(); err == nil {
		t.Fatal("expected error for empty items")
	}

	dup := valid
	dup.Items = append(dup.Items, ProposalItem{TempID: "A", Title: "dup"})
	if err := dup.Validate(); err == nil {
		t.Fatal("expected error for duplicate temp_id")
	}

	badDep := DecomposeProposal{
		ID:      "prop-xxx",
		Summary: "test",
		Items: []ProposalItem{
			{TempID: "A", Title: "task A", DependsOn: []string{"Z"}},
		},
	}
	if err := badDep.Validate(); err == nil {
		t.Fatal("expected error for missing dependency")
	}
}

func TestProposalDetectCycle(t *testing.T) {
	cyclic := DecomposeProposal{
		ID:      "prop-xxx",
		Summary: "test",
		Items: []ProposalItem{
			{TempID: "A", Title: "A", DependsOn: []string{"B"}},
			{TempID: "B", Title: "B", DependsOn: []string{"A"}},
		},
	}
	if err := cyclic.Validate(); err == nil {
		t.Fatal("expected error for cyclic dependency")
	}
}
