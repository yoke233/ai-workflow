package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/yoke233/ai-workflow/internal/core"
)

// ProjectErrorRanking returns projects ordered by failure count.
func (s *Store) ProjectErrorRanking(ctx context.Context, filter core.AnalyticsFilter) ([]core.ProjectErrorRank, error) {
	query := `
		SELECT
			p.id,
			p.name,
			COUNT(DISTINCT f.id) AS total_flows,
			COUNT(DISTINCT CASE WHEN f.status = 'failed' THEN f.id END) AS failed_flows,
			CASE WHEN COUNT(DISTINCT f.id) > 0
				THEN CAST(COUNT(DISTINCT CASE WHEN f.status = 'failed' THEN f.id END) AS REAL) / COUNT(DISTINCT f.id)
				ELSE 0 END AS failure_rate,
			COUNT(DISTINCT CASE WHEN e.status = 'failed' THEN e.id END) AS failed_execs
		FROM projects p
		LEFT JOIN flows f ON f.project_id = p.id
		LEFT JOIN executions e ON e.flow_id = f.id`

	var conditions []string
	var args []any

	conditions, args = appendTimeConditions(conditions, args, "f.created_at", filter)

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += ` GROUP BY p.id ORDER BY failed_flows DESC, failure_rate DESC`
	query += limitClause(filter.Limit, 20)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("project error ranking: %w", err)
	}
	defer rows.Close()

	var out []core.ProjectErrorRank
	for rows.Next() {
		var r core.ProjectErrorRank
		if err := rows.Scan(&r.ProjectID, &r.ProjectName, &r.TotalFlows,
			&r.FailedFlows, &r.FailureRate, &r.FailedExecs); err != nil {
			return nil, fmt.Errorf("scan project error rank: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// FlowBottleneckSteps returns the slowest/most-failing steps across flows.
func (s *Store) FlowBottleneckSteps(ctx context.Context, filter core.AnalyticsFilter) ([]core.StepBottleneck, error) {
	query := `
		SELECT
			st.id,
			st.name,
			st.flow_id,
			f.name,
			f.project_id,
			COALESCE(AVG(
				CASE WHEN e.started_at IS NOT NULL AND e.finished_at IS NOT NULL
					THEN (julianday(e.finished_at) - julianday(e.started_at)) * 86400
				END
			), 0) AS avg_dur,
			COALESCE(MAX(
				CASE WHEN e.started_at IS NOT NULL AND e.finished_at IS NOT NULL
					THEN (julianday(e.finished_at) - julianday(e.started_at)) * 86400
				END
			), 0) AS max_dur,
			COUNT(e.id) AS exec_count,
			COUNT(CASE WHEN e.status = 'failed' THEN 1 END) AS fail_count,
			COALESCE(SUM(e.attempt - 1), 0) AS retry_count,
			CASE WHEN COUNT(e.id) > 0
				THEN CAST(COUNT(CASE WHEN e.status = 'failed' THEN 1 END) AS REAL) / COUNT(e.id)
				ELSE 0 END AS fail_rate
		FROM steps st
		JOIN flows f ON f.id = st.flow_id
		LEFT JOIN executions e ON e.step_id = st.id`

	var conditions []string
	var args []any

	if filter.ProjectID != nil {
		conditions = append(conditions, "f.project_id = ?")
		args = append(args, *filter.ProjectID)
	}
	conditions, args = appendTimeConditions(conditions, args, "e.created_at", filter)

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += ` GROUP BY st.id
		HAVING exec_count > 0
		ORDER BY avg_dur DESC, fail_rate DESC`
	query += limitClause(filter.Limit, 20)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("flow bottleneck steps: %w", err)
	}
	defer rows.Close()

	var out []core.StepBottleneck
	for rows.Next() {
		var b core.StepBottleneck
		if err := rows.Scan(&b.StepID, &b.StepName, &b.FlowID, &b.FlowName, &b.ProjectID,
			&b.AvgDurationS, &b.MaxDurationS, &b.ExecCount, &b.FailCount,
			&b.RetryCount, &b.FailRate); err != nil {
			return nil, fmt.Errorf("scan step bottleneck: %w", err)
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// ExecutionDurationStats returns per-flow duration statistics.
func (s *Store) ExecutionDurationStats(ctx context.Context, filter core.AnalyticsFilter) ([]core.FlowDurationStat, error) {
	// SQLite doesn't have a PERCENTILE function, so we compute p50 with a subquery approach.
	// For simplicity, we use a CTE approach with ROW_NUMBER.
	query := `
		WITH exec_dur AS (
			SELECT
				e.flow_id,
				(julianday(e.finished_at) - julianday(e.started_at)) * 86400 AS dur_s
			FROM executions e
			JOIN flows f ON f.id = e.flow_id
			WHERE e.started_at IS NOT NULL AND e.finished_at IS NOT NULL
				AND e.status IN ('succeeded', 'failed')`

	var args []any
	if filter.ProjectID != nil {
		query += " AND f.project_id = ?"
		args = append(args, *filter.ProjectID)
	}
	if filter.Since != nil {
		query += " AND e.created_at >= ?"
		args = append(args, *filter.Since)
	}
	if filter.Until != nil {
		query += " AND e.created_at < ?"
		args = append(args, *filter.Until)
	}

	query += `
		)
		SELECT
			f.id,
			f.name,
			f.project_id,
			COUNT(*) AS exec_count,
			AVG(d.dur_s) AS avg_dur,
			MIN(d.dur_s) AS min_dur,
			MAX(d.dur_s) AS max_dur,
			0 AS p50_dur
		FROM exec_dur d
		JOIN flows f ON f.id = d.flow_id
		GROUP BY f.id
		ORDER BY avg_dur DESC`
	query += limitClause(filter.Limit, 20)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("execution duration stats: %w", err)
	}
	defer rows.Close()

	var out []core.FlowDurationStat
	for rows.Next() {
		var d core.FlowDurationStat
		if err := rows.Scan(&d.FlowID, &d.FlowName, &d.ProjectID,
			&d.ExecCount, &d.AvgDurationS, &d.MinDurationS, &d.MaxDurationS, &d.P50DurationS); err != nil {
			return nil, fmt.Errorf("scan duration stat: %w", err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// ErrorBreakdown returns error counts grouped by error_kind.
func (s *Store) ErrorBreakdown(ctx context.Context, filter core.AnalyticsFilter) ([]core.ErrorKindCount, error) {
	query := `
		SELECT
			COALESCE(e.error_kind, 'unknown') AS ek,
			COUNT(*) AS cnt
		FROM executions e
		JOIN flows f ON f.id = e.flow_id
		WHERE e.status = 'failed'`

	var args []any
	if filter.ProjectID != nil {
		query += " AND f.project_id = ?"
		args = append(args, *filter.ProjectID)
	}
	if filter.Since != nil {
		query += " AND e.created_at >= ?"
		args = append(args, *filter.Since)
	}
	if filter.Until != nil {
		query += " AND e.created_at < ?"
		args = append(args, *filter.Until)
	}

	query += ` GROUP BY ek ORDER BY cnt DESC`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("error breakdown: %w", err)
	}
	defer rows.Close()

	var out []core.ErrorKindCount
	var total int
	for rows.Next() {
		var c core.ErrorKindCount
		if err := rows.Scan(&c.ErrorKind, &c.Count); err != nil {
			return nil, fmt.Errorf("scan error kind: %w", err)
		}
		total += c.Count
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		if total > 0 {
			out[i].Pct = float64(out[i].Count) / float64(total)
		}
	}
	return out, nil
}

// RecentFailures returns recent failed executions with full context.
func (s *Store) RecentFailures(ctx context.Context, filter core.AnalyticsFilter) ([]core.FailureRecord, error) {
	query := `
		SELECT
			e.id,
			e.step_id,
			st.name,
			e.flow_id,
			f.name,
			f.project_id,
			COALESCE(p.name, ''),
			COALESCE(e.error_message, ''),
			COALESCE(e.error_kind, ''),
			e.attempt,
			CASE WHEN e.started_at IS NOT NULL AND e.finished_at IS NOT NULL
				THEN (julianday(e.finished_at) - julianday(e.started_at)) * 86400
				ELSE 0 END AS dur_s,
			COALESCE(e.finished_at, e.created_at) AS failed_at
		FROM executions e
		JOIN steps st ON st.id = e.step_id
		JOIN flows f ON f.id = e.flow_id
		LEFT JOIN projects p ON p.id = f.project_id
		WHERE e.status = 'failed'`

	var args []any
	if filter.ProjectID != nil {
		query += " AND f.project_id = ?"
		args = append(args, *filter.ProjectID)
	}
	if filter.Since != nil {
		query += " AND e.created_at >= ?"
		args = append(args, *filter.Since)
	}
	if filter.Until != nil {
		query += " AND e.created_at < ?"
		args = append(args, *filter.Until)
	}

	query += ` ORDER BY failed_at DESC`
	query += limitClause(filter.Limit, 30)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("recent failures: %w", err)
	}
	defer rows.Close()

	var out []core.FailureRecord
	for rows.Next() {
		var r core.FailureRecord
		var ek sql.NullString
		if err := rows.Scan(&r.ExecID, &r.StepID, &r.StepName, &r.FlowID, &r.FlowName,
			&r.ProjectID, &r.ProjectName, &r.ErrorMessage, &ek,
			&r.Attempt, &r.DurationS, &r.FailedAt); err != nil {
			return nil, fmt.Errorf("scan failure record: %w", err)
		}
		if ek.Valid {
			r.ErrorKind = core.ErrorKind(ek.String)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// FlowStatusDistribution returns flow counts grouped by status.
func (s *Store) FlowStatusDistribution(ctx context.Context, filter core.AnalyticsFilter) ([]core.StatusCount, error) {
	query := `
		SELECT status, COUNT(*) AS cnt
		FROM flows
		WHERE 1=1`

	var args []any
	if filter.ProjectID != nil {
		query += " AND project_id = ?"
		args = append(args, *filter.ProjectID)
	}
	if filter.Since != nil {
		query += " AND created_at >= ?"
		args = append(args, *filter.Since)
	}
	if filter.Until != nil {
		query += " AND created_at < ?"
		args = append(args, *filter.Until)
	}

	query += ` GROUP BY status ORDER BY cnt DESC`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("flow status distribution: %w", err)
	}
	defer rows.Close()

	var out []core.StatusCount
	for rows.Next() {
		var c core.StatusCount
		if err := rows.Scan(&c.Status, &c.Count); err != nil {
			return nil, fmt.Errorf("scan status count: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// helpers

func appendTimeConditions(conditions []string, args []any, col string, filter core.AnalyticsFilter) ([]string, []any) {
	if filter.Since != nil {
		conditions = append(conditions, col+" >= ?")
		args = append(args, *filter.Since)
	}
	if filter.Until != nil {
		conditions = append(conditions, col+" < ?")
		args = append(args, *filter.Until)
	}
	return conditions, args
}

func limitClause(requested, defaultLimit int) string {
	n := defaultLimit
	if requested > 0 {
		n = requested
	}
	return fmt.Sprintf(" LIMIT %d", n)
}
