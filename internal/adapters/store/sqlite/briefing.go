package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/yoke233/ai-workflow/internal/core"
)

func (s *Store) CreateBriefing(ctx context.Context, b *core.Briefing) (int64, error) {
	refs, err := marshalJSON(b.ContextRefs)
	if err != nil {
		return 0, fmt.Errorf("marshal context_refs: %w", err)
	}
	constraints, err := marshalJSON(b.Constraints)
	if err != nil {
		return 0, fmt.Errorf("marshal constraints: %w", err)
	}
	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO briefings (step_id, objective, context_refs, constraints, created_at)
		 VALUES (?, ?, ?, ?, ?)`,
		b.StepID, b.Objective, refs, constraints, now,
	)
	if err != nil {
		return 0, fmt.Errorf("insert briefing: %w", err)
	}
	id, _ := res.LastInsertId()
	b.ID = id
	b.CreatedAt = now
	return id, nil
}

func (s *Store) GetBriefing(ctx context.Context, id int64) (*core.Briefing, error) {
	b := &core.Briefing{}
	var refs, constraints sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT id, step_id, objective, context_refs, constraints, created_at
		 FROM briefings WHERE id = ?`, id,
	).Scan(&b.ID, &b.StepID, &b.Objective, &refs, &constraints, &b.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, core.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get briefing %d: %w", id, err)
	}
	if refs.Valid {
		_ = json.Unmarshal([]byte(refs.String), &b.ContextRefs)
	}
	if constraints.Valid {
		_ = json.Unmarshal([]byte(constraints.String), &b.Constraints)
	}
	return b, nil
}

func (s *Store) GetBriefingByStep(ctx context.Context, stepID int64) (*core.Briefing, error) {
	b := &core.Briefing{}
	var refs, constraints sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT id, step_id, objective, context_refs, constraints, created_at
		 FROM briefings WHERE step_id = ? ORDER BY id DESC LIMIT 1`, stepID,
	).Scan(&b.ID, &b.StepID, &b.Objective, &refs, &constraints, &b.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, core.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get briefing for step %d: %w", stepID, err)
	}
	if refs.Valid {
		_ = json.Unmarshal([]byte(refs.String), &b.ContextRefs)
	}
	if constraints.Valid {
		_ = json.Unmarshal([]byte(constraints.String), &b.Constraints)
	}
	return b, nil
}

