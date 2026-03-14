package threadapp

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/yoke233/ai-workflow/internal/core"
)

type Config struct {
	Store   Store
	Tx      Tx
	Runtime Runtime
}

type Service struct {
	store   Store
	tx      Tx
	runtime Runtime
}

func New(cfg Config) *Service {
	return &Service{
		store:   cfg.Store,
		tx:      cfg.Tx,
		runtime: cfg.Runtime,
	}
}

func (s *Service) CreateThread(ctx context.Context, input CreateThreadInput) (*CreateThreadResult, error) {
	thread := &core.Thread{
		Title:    strings.TrimSpace(input.Title),
		Status:   core.ThreadActive,
		OwnerID:  strings.TrimSpace(input.OwnerID),
		Summary:  strings.TrimSpace(input.Summary),
		Metadata: cloneMetadata(input.Metadata),
	}
	if thread.Title == "" {
		return nil, newError(CodeMissingTitle, "title is required", nil)
	}

	participants := buildParticipants(thread.OwnerID, input.ParticipantUserIDs)
	if err := s.createThreadAggregate(ctx, thread, participants); err != nil {
		return nil, err
	}

	return &CreateThreadResult{
		Thread:       thread,
		Participants: participants,
	}, nil
}

func (s *Service) DeleteThread(ctx context.Context, threadID int64) error {
	if _, err := s.store.GetThread(ctx, threadID); err != nil {
		if errors.Is(err, core.ErrNotFound) {
			return newError(CodeThreadNotFound, "thread not found", err)
		}
		return err
	}

	if s.runtime != nil {
		if err := s.runtime.CleanupThread(ctx, threadID); err != nil {
			return newError(CodeCleanupThreadFailed, err.Error(), err)
		}
	}

	return s.deleteThreadAggregate(ctx, threadID)
}

func (s *Service) LinkThreadWorkItem(ctx context.Context, input LinkThreadWorkItemInput) (*core.ThreadWorkItemLink, error) {
	if _, err := s.store.GetThread(ctx, input.ThreadID); err != nil {
		if errors.Is(err, core.ErrNotFound) {
			return nil, newError(CodeThreadNotFound, "thread not found", err)
		}
		return nil, err
	}
	if input.WorkItemID <= 0 {
		return nil, newError(CodeMissingWorkItemID, "work_item_id is required", nil)
	}
	if _, err := s.store.GetWorkItem(ctx, input.WorkItemID); err != nil {
		if errors.Is(err, core.ErrNotFound) {
			return nil, newError(CodeWorkItemNotFound, "work item not found", err)
		}
		return nil, err
	}

	link := &core.ThreadWorkItemLink{
		ThreadID:     input.ThreadID,
		WorkItemID:   input.WorkItemID,
		RelationType: strings.TrimSpace(input.RelationType),
		IsPrimary:    input.IsPrimary,
	}
	if link.RelationType == "" {
		link.RelationType = "related"
	}

	id, err := s.store.CreateThreadWorkItemLink(ctx, link)
	if err != nil {
		return nil, err
	}
	link.ID = id
	return link, nil
}

func (s *Service) UnlinkThreadWorkItem(ctx context.Context, threadID, workItemID int64) error {
	if err := s.store.DeleteThreadWorkItemLink(ctx, threadID, workItemID); err != nil {
		if errors.Is(err, core.ErrNotFound) {
			return newError(CodeLinkNotFound, "link not found", err)
		}
		return err
	}
	return nil
}

func (s *Service) CreateWorkItemFromThread(ctx context.Context, input CreateWorkItemFromThreadInput) (*CreateWorkItemFromThreadResult, error) {
	thread, err := s.store.GetThread(ctx, input.ThreadID)
	if err != nil {
		if errors.Is(err, core.ErrNotFound) {
			return nil, newError(CodeThreadNotFound, "thread not found", err)
		}
		return nil, err
	}

	var workItem *core.WorkItem
	var link *core.ThreadWorkItemLink
	if s.tx != nil {
		err := s.tx.InTx(ctx, func(ctx context.Context, txStore TxStore) error {
			var err error
			workItem, link, err = createLinkedWorkItemFromThreadData(ctx, txStore, thread, input.WorkItemTitle, input.WorkItemBody, input.ProjectID)
			return err
		})
		if err != nil {
			return nil, err
		}
	} else {
		workItem, link, err = createLinkedWorkItemFromThreadData(ctx, s.store, thread, input.WorkItemTitle, input.WorkItemBody, input.ProjectID)
		if err != nil {
			return nil, err
		}
	}

	return &CreateWorkItemFromThreadResult{
		Thread:   thread,
		WorkItem: workItem,
		Link:     link,
	}, nil
}

func (s *Service) CrystallizeChatSession(ctx context.Context, input CrystallizeChatSessionInput) (*CrystallizeChatSessionResult, error) {
	thread := &core.Thread{
		Title:   strings.TrimSpace(input.ThreadTitle),
		Status:  core.ThreadActive,
		OwnerID: strings.TrimSpace(input.OwnerID),
		Summary: strings.TrimSpace(input.ThreadSummary),
		Metadata: map[string]any{
			"source_chat_session_id": strings.TrimSpace(input.SessionID),
		},
	}
	if thread.Title == "" {
		return nil, newError(CodeMissingTitle, "title is required", nil)
	}

	participants := buildParticipants(thread.OwnerID, input.ParticipantUserIDs)
	var workItem *core.WorkItem

	if s.tx != nil {
		err := s.tx.InTx(ctx, func(ctx context.Context, txStore TxStore) error {
			if err := persistThreadWithParticipants(ctx, txStore, thread, participants); err != nil {
				return err
			}
			if !input.CreateWorkItem {
				return nil
			}
			var err error
			workItem, _, err = createLinkedWorkItemFromThreadData(ctx, txStore, thread, input.WorkItemTitle, input.WorkItemBody, input.ProjectID)
			return err
		})
		if err != nil {
			return nil, err
		}
		return &CrystallizeChatSessionResult{
			Thread:       thread,
			WorkItem:     workItem,
			Participants: participants,
		}, nil
	}

	if err := s.createThreadAggregate(ctx, thread, participants); err != nil {
		return nil, err
	}

	if input.CreateWorkItem {
		var err error
		workItem, _, err = createLinkedWorkItemFromThreadData(ctx, s.store, thread, input.WorkItemTitle, input.WorkItemBody, input.ProjectID)
		if err != nil {
			if rollbackErr := deleteThreadAggregateData(ctx, s.store, thread.ID); rollbackErr != nil {
				return nil, fmt.Errorf("%w; rollback failed: %v", err, rollbackErr)
			}
			return nil, err
		}
	}

	return &CrystallizeChatSessionResult{
		Thread:       thread,
		WorkItem:     workItem,
		Participants: participants,
	}, nil
}

func (s *Service) createThreadAggregate(ctx context.Context, thread *core.Thread, participants []*core.ThreadMember) error {
	if s.tx != nil {
		return s.tx.InTx(ctx, func(ctx context.Context, txStore TxStore) error {
			return persistThreadWithParticipants(ctx, txStore, thread, participants)
		})
	}

	if err := persistThreadWithParticipants(ctx, s.store, thread, participants); err != nil {
		if thread.ID > 0 {
			if rollbackErr := deleteThreadAggregateData(ctx, s.store, thread.ID); rollbackErr != nil {
				return fmt.Errorf("%w; rollback failed: %v", err, rollbackErr)
			}
		}
		return err
	}
	return nil
}

func (s *Service) deleteThreadAggregate(ctx context.Context, threadID int64) error {
	if s.tx != nil {
		return s.tx.InTx(ctx, func(ctx context.Context, txStore TxStore) error {
			return deleteThreadAggregateData(ctx, txStore, threadID)
		})
	}
	return deleteThreadAggregateData(ctx, s.store, threadID)
}

func persistThreadWithParticipants(ctx context.Context, store TxStore, thread *core.Thread, participants []*core.ThreadMember) error {
	threadID, err := store.CreateThread(ctx, thread)
	if err != nil {
		return err
	}
	thread.ID = threadID

	for _, participant := range participants {
		if participant == nil {
			continue
		}
		participant.ThreadID = thread.ID
		id, err := store.AddThreadMember(ctx, participant)
		if err != nil {
			return err
		}
		participant.ID = id
	}
	return nil
}

func deleteThreadAggregateData(ctx context.Context, store TxStore, threadID int64) error {
	if err := store.DeleteThreadWorkItemLinksByThread(ctx, threadID); err != nil {
		return err
	}
	if err := store.DeleteThreadMessagesByThread(ctx, threadID); err != nil {
		return err
	}
	if err := store.DeleteThreadMembersByThread(ctx, threadID); err != nil {
		return err
	}
	if err := store.DeleteThread(ctx, threadID); err != nil {
		if errors.Is(err, core.ErrNotFound) {
			return newError(CodeThreadNotFound, "thread not found", err)
		}
		return err
	}
	return nil
}

func createLinkedWorkItemFromThreadData(ctx context.Context, store TxStore, thread *core.Thread, title string, body string, projectID *int64) (*core.WorkItem, *core.ThreadWorkItemLink, error) {
	if thread == nil {
		return nil, nil, errors.New("thread is required")
	}

	title = strings.TrimSpace(title)
	if title == "" {
		return nil, nil, newError(CodeMissingTitle, "title is required", nil)
	}

	body = strings.TrimSpace(body)
	summary := strings.TrimSpace(thread.Summary)
	bodyFromSummary := false
	if body == "" {
		if summary == "" {
			return nil, nil, newError(CodeMissingThreadSummary, "please generate or fill in summary first", nil)
		}
		body = summary
		bodyFromSummary = true
	}

	sourceType := "thread_manual"
	if bodyFromSummary {
		sourceType = "thread_summary"
	}

	workItem := &core.WorkItem{
		Title:     title,
		Body:      body,
		Status:    core.WorkItemOpen,
		Priority:  core.PriorityMedium,
		ProjectID: projectID,
		Metadata: map[string]any{
			"source_thread_id":  thread.ID,
			"source_type":       sourceType,
			"body_from_summary": bodyFromSummary,
		},
	}

	id, err := store.CreateWorkItem(ctx, workItem)
	if err != nil {
		return nil, nil, err
	}
	workItem.ID = id

	link := &core.ThreadWorkItemLink{
		ThreadID:     thread.ID,
		WorkItemID:   id,
		RelationType: "drives",
		IsPrimary:    true,
	}
	linkID, err := store.CreateThreadWorkItemLink(ctx, link)
	if err != nil {
		if rollbackErr := store.DeleteWorkItem(ctx, id); rollbackErr != nil {
			return nil, nil, fmt.Errorf("%w; rollback failed: %v", err, rollbackErr)
		}
		return nil, nil, err
	}
	link.ID = linkID

	return workItem, link, nil
}

func buildParticipants(ownerID string, memberIDs []string) []*core.ThreadMember {
	participants := make([]*core.ThreadMember, 0, len(memberIDs)+1)
	seen := make(map[string]bool)

	ownerID = strings.TrimSpace(ownerID)
	if ownerID != "" {
		participants = append(participants, &core.ThreadMember{
			Kind:   core.ThreadMemberKindHuman,
			UserID: ownerID,
			Role:   "owner",
		})
		seen[ownerID] = true
	}

	for _, participantID := range memberIDs {
		participantID = strings.TrimSpace(participantID)
		if participantID == "" || seen[participantID] {
			continue
		}
		participants = append(participants, &core.ThreadMember{
			Kind:   core.ThreadMemberKindHuman,
			UserID: participantID,
			Role:   "member",
		})
		seen[participantID] = true
	}

	return participants
}

func cloneMetadata(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
