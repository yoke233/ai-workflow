package core

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type InitiativeStatus string

const (
	InitiativeDraft     InitiativeStatus = "draft"
	InitiativeProposed  InitiativeStatus = "proposed"
	InitiativeApproved  InitiativeStatus = "approved"
	InitiativeExecuting InitiativeStatus = "executing"
	InitiativeBlocked   InitiativeStatus = "blocked"
	InitiativeDone      InitiativeStatus = "done"
	InitiativeFailed    InitiativeStatus = "failed"
	InitiativeCancelled InitiativeStatus = "cancelled"
)

func (s InitiativeStatus) Valid() bool {
	switch s {
	case InitiativeDraft, InitiativeProposed, InitiativeApproved, InitiativeExecuting, InitiativeBlocked, InitiativeDone, InitiativeFailed, InitiativeCancelled:
		return true
	default:
		return false
	}
}

func ParseInitiativeStatus(raw string) (InitiativeStatus, error) {
	s := InitiativeStatus(strings.TrimSpace(raw))
	if !s.Valid() {
		return "", fmt.Errorf("invalid initiative status %q", raw)
	}
	return s, nil
}

type Initiative struct {
	ID          int64            `json:"id"`
	Title       string           `json:"title"`
	Description string           `json:"description"`
	Status      InitiativeStatus `json:"status"`
	CreatedBy   string           `json:"created_by"`
	ApprovedBy  *string          `json:"approved_by,omitempty"`
	ApprovedAt  *time.Time       `json:"approved_at,omitempty"`
	ReviewNote  string           `json:"review_note,omitempty"`
	Metadata    map[string]any   `json:"metadata,omitempty"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
}

type InitiativeItem struct {
	ID           int64     `json:"id"`
	InitiativeID int64     `json:"initiative_id"`
	WorkItemID   int64     `json:"work_item_id"`
	Role         string    `json:"role,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

type ThreadInitiativeLink struct {
	ID           int64     `json:"id"`
	ThreadID     int64     `json:"thread_id"`
	InitiativeID int64     `json:"initiative_id"`
	RelationType string    `json:"relation_type"`
	CreatedAt    time.Time `json:"created_at"`
}

type InitiativeFilter struct {
	Status *InitiativeStatus
	Limit  int
	Offset int
}

type InitiativeProgress struct {
	Total     int `json:"total"`
	Pending   int `json:"pending"`
	Running   int `json:"running"`
	Blocked   int `json:"blocked"`
	Done      int `json:"done"`
	Failed    int `json:"failed"`
	Cancelled int `json:"cancelled"`
}

type InitiativeStore interface {
	CreateInitiative(ctx context.Context, initiative *Initiative) (int64, error)
	GetInitiative(ctx context.Context, id int64) (*Initiative, error)
	ListInitiatives(ctx context.Context, filter InitiativeFilter) ([]*Initiative, error)
	UpdateInitiative(ctx context.Context, initiative *Initiative) error
	DeleteInitiative(ctx context.Context, id int64) error

	CreateInitiativeItem(ctx context.Context, item *InitiativeItem) (int64, error)
	ListInitiativeItems(ctx context.Context, initiativeID int64) ([]*InitiativeItem, error)
	ListInitiativeItemsByWorkItem(ctx context.Context, workItemID int64) ([]*InitiativeItem, error)
	UpdateInitiativeItem(ctx context.Context, item *InitiativeItem) error
	DeleteInitiativeItem(ctx context.Context, initiativeID int64, workItemID int64) error
	DeleteInitiativeItemsByInitiative(ctx context.Context, initiativeID int64) error

	CreateThreadInitiativeLink(ctx context.Context, link *ThreadInitiativeLink) (int64, error)
	ListThreadsByInitiative(ctx context.Context, initiativeID int64) ([]*ThreadInitiativeLink, error)
	ListInitiativesByThread(ctx context.Context, threadID int64) ([]*ThreadInitiativeLink, error)
	DeleteThreadInitiativeLink(ctx context.Context, initiativeID int64, threadID int64) error
	DeleteThreadInitiativeLinksByInitiative(ctx context.Context, initiativeID int64) error
}

func CanTransitionInitiativeStatus(from, to InitiativeStatus) bool {
	if !from.Valid() || !to.Valid() {
		return false
	}
	if from == to {
		return true
	}
	switch from {
	case InitiativeDraft:
		return to == InitiativeProposed || to == InitiativeCancelled
	case InitiativeProposed:
		return to == InitiativeDraft || to == InitiativeApproved || to == InitiativeCancelled
	case InitiativeApproved:
		return to == InitiativeExecuting || to == InitiativeCancelled
	case InitiativeExecuting:
		return to == InitiativeBlocked || to == InitiativeDone || to == InitiativeFailed || to == InitiativeCancelled
	case InitiativeBlocked:
		return to == InitiativeExecuting || to == InitiativeDone || to == InitiativeFailed || to == InitiativeCancelled
	case InitiativeDone, InitiativeFailed, InitiativeCancelled:
		return false
	default:
		return false
	}
}
