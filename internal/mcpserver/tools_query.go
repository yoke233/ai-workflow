package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yoke233/ai-workflow/internal/core"
)

func registerQueryTools(server *mcp.Server, store core.Store) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "query_projects",
		Description: "List projects with optional name filter",
	}, queryProjectsHandler(store))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "query_project_detail",
		Description: "Get detailed information about a project",
	}, queryProjectDetailHandler(store))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "query_issues",
		Description: "List issues for a project with optional filters",
	}, queryIssuesHandler(store))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "query_issue_detail",
		Description: "Get issue detail including changes and review records",
	}, queryIssueDetailHandler(store))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "query_runs",
		Description: "List runs for a project with optional filters",
	}, queryRunsHandler(store))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "query_run_detail",
		Description: "Get run detail including checkpoints",
	}, queryRunDetailHandler(store))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "query_run_events",
		Description: "List events for a run",
	}, queryRunEventsHandler(store))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "query_project_stats",
		Description: "Get aggregated statistics for a project",
	}, queryProjectStatsHandler(store))
}

// --- Input types ---

type QueryProjectsInput struct {
	NameContains string `json:"name_contains,omitempty" jsonschema:"Filter projects by name substring"`
}

type QueryProjectDetailInput struct {
	ProjectID string `json:"project_id" jsonschema:"Project ID"`
}

type QueryIssuesInput struct {
	ProjectID string `json:"project_id" jsonschema:"Project ID"`
	Status    string `json:"status,omitempty" jsonschema:"Filter by issue status"`
	State     string `json:"state,omitempty" jsonschema:"Filter by issue state (open/closed)"`
	SessionID string `json:"session_id,omitempty" jsonschema:"Filter by chat session ID"`
	ParentID  string `json:"parent_id,omitempty" jsonschema:"Filter by parent issue ID (for child issues)"`
	Limit     int    `json:"limit,omitempty" jsonschema:"Max results to return"`
	Offset    int    `json:"offset,omitempty" jsonschema:"Number of results to skip"`
}

type QueryIssueDetailInput struct {
	IssueID string `json:"issue_id" jsonschema:"Issue ID"`
}

type QueryRunsInput struct {
	ProjectID  string `json:"project_id" jsonschema:"Project ID"`
	Status     string `json:"status,omitempty" jsonschema:"Filter by run status"`
	Conclusion string `json:"conclusion,omitempty" jsonschema:"Filter: success/failure/timed_out/cancelled"`
	IssueID    string `json:"issue_id,omitempty" jsonschema:"Filter by associated issue ID"`
	Limit      int    `json:"limit,omitempty" jsonschema:"Max results to return"`
	Offset     int    `json:"offset,omitempty" jsonschema:"Number of results to skip"`
}

type QueryRunDetailInput struct {
	RunID string `json:"run_id" jsonschema:"Run ID"`
}

type QueryRunEventsInput struct {
	RunID     string `json:"run_id" jsonschema:"Run ID"`
	EventType string `json:"event_type,omitempty" jsonschema:"Filter: stage_start/stage_failed/agent_output/..."`
	Stage     string `json:"stage,omitempty" jsonschema:"Filter by stage name"`
	Limit     int    `json:"limit,omitempty" jsonschema:"Max events to return"`
}

type QueryProjectStatsInput struct {
	ProjectID string `json:"project_id" jsonschema:"Project ID"`
}

// --- Output types ---

type ProjectStats struct {
	TotalIssues   int     `json:"total_issues"`
	OpenIssues    int     `json:"open_issues"`
	ClosedIssues  int     `json:"closed_issues"`
	TotalRuns     int     `json:"total_runs"`
	CompletedRuns int     `json:"completed_runs"`
	SuccessRate   float64 `json:"success_rate"`
}

type IssueDetail struct {
	Issue   core.Issue          `json:"issue"`
	Changes []core.IssueChange  `json:"changes"`
	Reviews []core.ReviewRecord `json:"reviews"`
}

type RunDetail struct {
	Run         core.Run          `json:"run"`
	Checkpoints []core.Checkpoint `json:"checkpoints"`
}

// --- Handlers ---

func queryProjectsHandler(store core.Store) func(context.Context, *mcp.CallToolRequest, QueryProjectsInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, in QueryProjectsInput) (*mcp.CallToolResult, any, error) {
		projects, err := store.ListProjects(core.ProjectFilter{
			NameContains: in.NameContains,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("list projects: %w", err)
		}
		if projects == nil {
			projects = []core.Project{}
		}
		return jsonResult(projects)
	}
}

func queryProjectDetailHandler(store core.Store) func(context.Context, *mcp.CallToolRequest, QueryProjectDetailInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, in QueryProjectDetailInput) (*mcp.CallToolResult, any, error) {
		if in.ProjectID == "" {
			return errorResult("project_id is required")
		}
		project, err := store.GetProject(in.ProjectID)
		if err != nil {
			return nil, nil, fmt.Errorf("get project: %w", err)
		}
		if project == nil {
			return errorResult("project not found: " + in.ProjectID)
		}
		return jsonResult(project)
	}
}

func queryIssuesHandler(store core.Store) func(context.Context, *mcp.CallToolRequest, QueryIssuesInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, in QueryIssuesInput) (*mcp.CallToolResult, any, error) {
		if in.ProjectID == "" {
			return errorResult("project_id is required")
		}
		issues, _, err := store.ListIssues(in.ProjectID, core.IssueFilter{
			Status:    in.Status,
			State:     in.State,
			SessionID: in.SessionID,
			ParentID:  in.ParentID,
			Limit:     in.Limit,
			Offset:    in.Offset,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("list issues: %w", err)
		}
		if issues == nil {
			issues = []core.Issue{}
		}
		return jsonResult(issues)
	}
}

func queryIssueDetailHandler(store core.Store) func(context.Context, *mcp.CallToolRequest, QueryIssueDetailInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, in QueryIssueDetailInput) (*mcp.CallToolResult, any, error) {
		if in.IssueID == "" {
			return errorResult("issue_id is required")
		}
		issue, err := store.GetIssue(in.IssueID)
		if err != nil {
			return nil, nil, fmt.Errorf("get issue: %w", err)
		}
		if issue == nil {
			return errorResult("issue not found: " + in.IssueID)
		}
		changes, err := store.GetIssueChanges(in.IssueID)
		if err != nil {
			return nil, nil, fmt.Errorf("get issue changes: %w", err)
		}
		if changes == nil {
			changes = []core.IssueChange{}
		}
		reviews, err := store.GetReviewRecords(in.IssueID)
		if err != nil {
			return nil, nil, fmt.Errorf("get review records: %w", err)
		}
		if reviews == nil {
			reviews = []core.ReviewRecord{}
		}
		return jsonResult(IssueDetail{
			Issue:   *issue,
			Changes: changes,
			Reviews: reviews,
		})
	}
}

func queryRunsHandler(store core.Store) func(context.Context, *mcp.CallToolRequest, QueryRunsInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, in QueryRunsInput) (*mcp.CallToolResult, any, error) {
		if in.ProjectID == "" {
			return errorResult("project_id is required")
		}
		runs, err := store.ListRuns(in.ProjectID, core.RunFilter{
			Status:     core.RunStatus(in.Status),
			Conclusion: core.RunConclusion(in.Conclusion),
			IssueID:    in.IssueID,
			Limit:      in.Limit,
			Offset:     in.Offset,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("list runs: %w", err)
		}
		if runs == nil {
			runs = []core.Run{}
		}
		return jsonResult(runs)
	}
}

func queryRunDetailHandler(store core.Store) func(context.Context, *mcp.CallToolRequest, QueryRunDetailInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, in QueryRunDetailInput) (*mcp.CallToolResult, any, error) {
		if in.RunID == "" {
			return errorResult("run_id is required")
		}
		run, err := store.GetRun(in.RunID)
		if err != nil {
			return nil, nil, fmt.Errorf("get run: %w", err)
		}
		if run == nil {
			return errorResult("run not found: " + in.RunID)
		}
		checkpoints, err := store.GetCheckpoints(in.RunID)
		if err != nil {
			return nil, nil, fmt.Errorf("get checkpoints: %w", err)
		}
		if checkpoints == nil {
			checkpoints = []core.Checkpoint{}
		}
		return jsonResult(RunDetail{
			Run:         *run,
			Checkpoints: checkpoints,
		})
	}
}

func queryRunEventsHandler(store core.Store) func(context.Context, *mcp.CallToolRequest, QueryRunEventsInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, in QueryRunEventsInput) (*mcp.CallToolResult, any, error) {
		if in.RunID == "" {
			return errorResult("run_id is required")
		}
		events, err := store.ListRunEvents(in.RunID)
		if err != nil {
			return nil, nil, fmt.Errorf("list run events: %w", err)
		}
		if in.EventType != "" || in.Stage != "" {
			filtered := events[:0]
			for _, e := range events {
				if in.EventType != "" && e.EventType != in.EventType {
					continue
				}
				if in.Stage != "" && e.Stage != in.Stage {
					continue
				}
				filtered = append(filtered, e)
			}
			events = filtered
		}
		if in.Limit > 0 && len(events) > in.Limit {
			events = events[:in.Limit]
		}
		if events == nil {
			events = []core.RunEvent{}
		}
		return jsonResult(events)
	}
}

func queryProjectStatsHandler(store core.Store) func(context.Context, *mcp.CallToolRequest, QueryProjectStatsInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, in QueryProjectStatsInput) (*mcp.CallToolResult, any, error) {
		if in.ProjectID == "" {
			return errorResult("project_id is required")
		}
		issues, totalIssues, err := store.ListIssues(in.ProjectID, core.IssueFilter{})
		if err != nil {
			return nil, nil, fmt.Errorf("list issues for stats: %w", err)
		}
		openCount := 0
		for _, iss := range issues {
			if iss.State == core.IssueStateOpen {
				openCount++
			}
		}

		runs, err := store.ListRuns(in.ProjectID, core.RunFilter{})
		if err != nil {
			return nil, nil, fmt.Errorf("list runs for stats: %w", err)
		}
		completedCount := 0
		successCount := 0
		for _, r := range runs {
			if r.Status == core.StatusCompleted {
				completedCount++
				if r.Conclusion == core.ConclusionSuccess {
					successCount++
				}
			}
		}

		var successRate float64
		if completedCount > 0 {
			successRate = float64(successCount) / float64(completedCount)
		}

		return jsonResult(ProjectStats{
			TotalIssues:   totalIssues,
			OpenIssues:    openCount,
			ClosedIssues:  totalIssues - openCount,
			TotalRuns:     len(runs),
			CompletedRuns: completedCount,
			SuccessRate:   successRate,
		})
	}
}

// --- System info ---

type SystemInfoInput struct{}

type SystemInfoOutput struct {
	ConfigDir    string            `json:"config_dir"`
	ConfigFile   string            `json:"config_file"`
	DefaultsFile string            `json:"defaults_file"`
	DBPath       string            `json:"db_path"`
	ServerAddr   string            `json:"server_addr,omitempty"`
	DevMode      bool              `json:"dev_mode"`
	SourceRoot   string            `json:"source_root,omitempty"`
	EditableFiles map[string]string `json:"editable_files"`
}

func registerSystemInfoTool(server *mcp.Server, opts Options) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_system_info",
		Description: "Get ai-workflow system configuration paths and runtime info. Use this to find config files you can read/modify.",
	}, systemInfoHandler(opts))
}

func systemInfoHandler(opts Options) func(context.Context, *mcp.CallToolRequest, SystemInfoInput) (*mcp.CallToolResult, any, error) {
	return func(_ context.Context, _ *mcp.CallToolRequest, _ SystemInfoInput) (*mcp.CallToolResult, any, error) {
		configFile := ""
		defaultsFile := "configs/defaults.yaml"
		if opts.ConfigDir != "" {
			configFile = opts.ConfigDir + "/config.yaml"
		}
		if opts.SourceRoot != "" {
			defaultsFile = opts.SourceRoot + "/configs/defaults.yaml"
		}
		return jsonResult(SystemInfoOutput{
			ConfigDir:    opts.ConfigDir,
			ConfigFile:   configFile,
			DefaultsFile: defaultsFile,
			DBPath:       opts.DBPath,
			ServerAddr:   opts.ServerAddr,
			DevMode:      opts.DevMode,
			SourceRoot:   opts.SourceRoot,
			EditableFiles: map[string]string{
				"project_config": configFile,
				"defaults":       defaultsFile,
			},
		})
	}
}

// --- Helpers ---

func jsonResult(v any) (*mcp.CallToolResult, any, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal result: %w", err)
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(data)}},
	}, nil, nil
}

func errorResult(msg string) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: msg}},
		IsError: true,
	}, nil, nil
}
