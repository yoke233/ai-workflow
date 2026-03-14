package sqlite

import (
	"context"
	"fmt"
	"time"
)

func (s *Store) DeleteActionResourcesByWorkItem(ctx context.Context, workItemID int64) error {
	if s == nil || s.orm == nil {
		return fmt.Errorf("store is not initialized")
	}
	subQuery := s.orm.WithContext(ctx).
		Model(&ActionModel{}).
		Select("id").
		Where("issue_id = ?", workItemID)
	return s.orm.WithContext(ctx).
		Where("action_id IN (?)", subQuery).
		Delete(&ActionResourceModel{}).Error
}

func (s *Store) DeleteRunsByWorkItem(ctx context.Context, workItemID int64) error {
	if s == nil || s.orm == nil {
		return fmt.Errorf("store is not initialized")
	}
	return s.orm.WithContext(ctx).
		Where("issue_id = ?", workItemID).
		Delete(&RunModel{}).Error
}

func (s *Store) DeleteActionSignalsByWorkItem(ctx context.Context, workItemID int64) error {
	if s == nil || s.orm == nil {
		return fmt.Errorf("store is not initialized")
	}
	return s.orm.WithContext(ctx).
		Where("issue_id = ?", workItemID).
		Delete(&ActionSignalModel{}).Error
}

func (s *Store) DeleteAgentContextsByWorkItem(ctx context.Context, workItemID int64) error {
	if s == nil || s.orm == nil {
		return fmt.Errorf("store is not initialized")
	}
	return s.orm.WithContext(ctx).
		Where("issue_id = ?", workItemID).
		Delete(&AgentContextModel{}).Error
}

func (s *Store) DeleteEventsByWorkItem(ctx context.Context, workItemID int64) error {
	if s == nil || s.orm == nil {
		return fmt.Errorf("store is not initialized")
	}
	return s.orm.WithContext(ctx).
		Where("issue_id = ?", workItemID).
		Delete(&EventModel{}).Error
}

func (s *Store) DeleteJournalByWorkItem(ctx context.Context, workItemID int64) error {
	if s == nil || s.orm == nil {
		return fmt.Errorf("store is not initialized")
	}
	return s.orm.WithContext(ctx).
		Where("work_item_id = ?", workItemID).
		Delete(&JournalModel{}).Error
}

func (s *Store) DeleteResourceBindingsByWorkItem(ctx context.Context, workItemID int64) error {
	if s == nil || s.orm == nil {
		return fmt.Errorf("store is not initialized")
	}
	return s.orm.WithContext(ctx).
		Where("issue_id = ?", workItemID).
		Delete(&ResourceBindingModel{}).Error
}

func (s *Store) DeleteActionsByWorkItem(ctx context.Context, workItemID int64) error {
	if s == nil || s.orm == nil {
		return fmt.Errorf("store is not initialized")
	}
	return s.orm.WithContext(ctx).
		Where("issue_id = ?", workItemID).
		Delete(&ActionModel{}).Error
}

func (s *Store) DetachFeatureEntriesByWorkItem(ctx context.Context, workItemID int64) error {
	if s == nil || s.orm == nil {
		return fmt.Errorf("store is not initialized")
	}
	now := time.Now().UTC()
	subQuery := s.orm.WithContext(ctx).
		Model(&ActionModel{}).
		Select("id").
		Where("issue_id = ?", workItemID)
	return s.orm.WithContext(ctx).
		Model(&FeatureEntryModel{}).
		Where("issue_id = ? OR step_id IN (?)", workItemID, subQuery).
		Updates(map[string]any{
			"issue_id":   nil,
			"step_id":    nil,
			"updated_at": now,
		}).Error
}
