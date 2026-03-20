package core

import (
	"context"
	"strings"
	"time"
)

type ProposalStatus string

const (
	ProposalDraft    ProposalStatus = "draft"
	ProposalOpen     ProposalStatus = "open"
	ProposalApproved ProposalStatus = "approved"
	ProposalRejected ProposalStatus = "rejected"
	ProposalRevised  ProposalStatus = "revised"
	ProposalMerged   ProposalStatus = "merged"
)

func (s ProposalStatus) Valid() bool {
	switch s {
	case ProposalDraft, ProposalOpen, ProposalApproved, ProposalRejected, ProposalRevised, ProposalMerged:
		return true
	default:
		return false
	}
}

func ParseProposalStatus(raw string) (ProposalStatus, bool) {
	status := ProposalStatus(strings.TrimSpace(raw))
	return status, status.Valid()
}

func CanTransitionProposalStatus(from, to ProposalStatus) bool {
	if !from.Valid() || !to.Valid() {
		return false
	}
	if from == to {
		return true
	}
	switch from {
	case ProposalDraft:
		return to == ProposalOpen || to == ProposalRejected
	case ProposalOpen:
		return to == ProposalApproved || to == ProposalRejected
	case ProposalRejected:
		return to == ProposalRevised
	case ProposalRevised:
		return to == ProposalOpen || to == ProposalRejected
	case ProposalApproved:
		return to == ProposalMerged
	case ProposalMerged:
		return false
	default:
		return false
	}
}

type ProposalWorkItemDraft struct {
	TempID    string           `json:"temp_id"`
	ProjectID *int64           `json:"project_id,omitempty"`
	Title     string           `json:"title"`
	Body      string           `json:"body"`
	Priority  WorkItemPriority `json:"priority"`
	DependsOn []string         `json:"depends_on,omitempty"`
	Labels    []string         `json:"labels,omitempty"`
}

type ThreadProposal struct {
	ID              int64                   `json:"id"`
	ThreadID        int64                   `json:"thread_id"`
	Title           string                  `json:"title"`
	Summary         string                  `json:"summary"`
	Content         string                  `json:"content"`
	ProposedBy      string                  `json:"proposed_by"`
	Status          ProposalStatus          `json:"status"`
	ReviewedBy      *string                 `json:"reviewed_by,omitempty"`
	ReviewedAt      *time.Time              `json:"reviewed_at,omitempty"`
	ReviewNote      string                  `json:"review_note,omitempty"`
	WorkItemDrafts  []ProposalWorkItemDraft `json:"work_item_drafts,omitempty"`
	SourceMessageID *int64                  `json:"source_message_id,omitempty"`
	InitiativeID    *int64                  `json:"initiative_id,omitempty"`
	Metadata        map[string]any          `json:"metadata,omitempty"`
	CreatedAt       time.Time               `json:"created_at"`
	UpdatedAt       time.Time               `json:"updated_at"`
}

type ProposalFilter struct {
	ThreadID *int64
	Status   *ProposalStatus
	Limit    int
	Offset   int
}

type ProposalStore interface {
	CreateThreadProposal(ctx context.Context, proposal *ThreadProposal) (int64, error)
	GetThreadProposal(ctx context.Context, id int64) (*ThreadProposal, error)
	ListThreadProposals(ctx context.Context, filter ProposalFilter) ([]*ThreadProposal, error)
	UpdateThreadProposal(ctx context.Context, proposal *ThreadProposal) error
	DeleteThreadProposal(ctx context.Context, id int64) error
}
