package sqlite

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/yoke233/zhanggui/internal/core"
	"gorm.io/gorm"
)

func (s *Store) CreateThreadProposal(ctx context.Context, proposal *core.ThreadProposal) (int64, error) {
	if s == nil || s.orm == nil {
		return 0, fmt.Errorf("store is not initialized")
	}
	if proposal == nil {
		return 0, fmt.Errorf("proposal is nil")
	}

	title := strings.TrimSpace(proposal.Title)
	if title == "" {
		return 0, fmt.Errorf("title is required")
	}
	if proposal.Status == "" {
		proposal.Status = core.ProposalDraft
	}
	if !proposal.Status.Valid() {
		return 0, fmt.Errorf("invalid proposal status %q", proposal.Status)
	}

	now := time.Now().UTC()
	model := threadProposalModelFromCore(proposal)
	model.Title = title
	model.Summary = strings.TrimSpace(proposal.Summary)
	model.Content = strings.TrimSpace(proposal.Content)
	model.ProposedBy = strings.TrimSpace(proposal.ProposedBy)
	model.CreatedAt = now
	model.UpdatedAt = now

	if err := s.orm.WithContext(ctx).Create(model).Error; err != nil {
		return 0, err
	}
	proposal.ID = model.ID
	proposal.Title = title
	proposal.Summary = model.Summary
	proposal.Content = model.Content
	proposal.ProposedBy = model.ProposedBy
	proposal.CreatedAt = now
	proposal.UpdatedAt = now
	return model.ID, nil
}

func (s *Store) GetThreadProposal(ctx context.Context, id int64) (*core.ThreadProposal, error) {
	if s == nil || s.orm == nil {
		return nil, fmt.Errorf("store is not initialized")
	}

	var model ThreadProposalModel
	err := s.orm.WithContext(ctx).First(&model, id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, core.ErrNotFound
		}
		return nil, err
	}
	return model.toCore(), nil
}

func (s *Store) ListThreadProposals(ctx context.Context, filter core.ProposalFilter) ([]*core.ThreadProposal, error) {
	if s == nil || s.orm == nil {
		return nil, fmt.Errorf("store is not initialized")
	}

	query := s.orm.WithContext(ctx).Model(&ThreadProposalModel{})
	if filter.ThreadID != nil {
		query = query.Where("thread_id = ?", *filter.ThreadID)
	}
	if filter.Status != nil {
		query = query.Where("status = ?", string(*filter.Status))
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	var models []ThreadProposalModel
	if err := query.Order("id DESC").Limit(limit).Offset(offset).Find(&models).Error; err != nil {
		return nil, err
	}
	out := make([]*core.ThreadProposal, 0, len(models))
	for i := range models {
		out = append(out, models[i].toCore())
	}
	return out, nil
}

func (s *Store) UpdateThreadProposal(ctx context.Context, proposal *core.ThreadProposal) error {
	if s == nil || s.orm == nil {
		return fmt.Errorf("store is not initialized")
	}
	if proposal == nil {
		return fmt.Errorf("proposal is nil")
	}
	title := strings.TrimSpace(proposal.Title)
	if title == "" {
		return fmt.Errorf("title is required")
	}
	if !proposal.Status.Valid() {
		return fmt.Errorf("invalid proposal status %q", proposal.Status)
	}

	now := time.Now().UTC()
	result := s.orm.WithContext(ctx).Model(&ThreadProposalModel{}).
		Where("id = ?", proposal.ID).
		Updates(map[string]any{
			"thread_id":         proposal.ThreadID,
			"title":             title,
			"summary":           strings.TrimSpace(proposal.Summary),
			"content":           strings.TrimSpace(proposal.Content),
			"proposed_by":       strings.TrimSpace(proposal.ProposedBy),
			"status":            string(proposal.Status),
			"reviewed_by":       proposal.ReviewedBy,
			"reviewed_at":       proposal.ReviewedAt,
			"review_note":       proposal.ReviewNote,
			"work_item_drafts":  JSONField[[]core.ProposalWorkItemDraft]{Data: proposal.WorkItemDrafts},
			"source_message_id": proposal.SourceMessageID,
			"initiative_id":     proposal.InitiativeID,
			"metadata":          JSONField[map[string]any]{Data: proposal.Metadata},
			"updated_at":        now,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return core.ErrNotFound
	}
	proposal.Title = title
	proposal.UpdatedAt = now
	return nil
}

func (s *Store) DeleteThreadProposal(ctx context.Context, id int64) error {
	if s == nil || s.orm == nil {
		return fmt.Errorf("store is not initialized")
	}

	result := s.orm.WithContext(ctx).Delete(&ThreadProposalModel{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return core.ErrNotFound
	}
	return nil
}
