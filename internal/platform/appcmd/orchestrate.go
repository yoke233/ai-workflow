package appcmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/yoke233/zhanggui/internal/adapters/llm"
	llmplanning "github.com/yoke233/zhanggui/internal/adapters/planning/llm"
	"github.com/yoke233/zhanggui/internal/adapters/store/sqlite"
	"github.com/yoke233/zhanggui/internal/application/ceoapp"
	"github.com/yoke233/zhanggui/internal/application/orchestrateapp"
	"github.com/yoke233/zhanggui/internal/application/planning"
	"github.com/yoke233/zhanggui/internal/application/requirementapp"
	"github.com/yoke233/zhanggui/internal/application/threadapp"
	"github.com/yoke233/zhanggui/internal/application/workitemapp"
	"github.com/yoke233/zhanggui/internal/core"
	"github.com/yoke233/zhanggui/internal/platform/bootstrap"
	"github.com/yoke233/zhanggui/internal/platform/config"
	"github.com/yoke233/zhanggui/internal/skills"
)

type orchestrateCLIOptions struct {
	Action            string
	Title             string
	Body              string
	Priority          string
	Labels            []string
	DedupeKey         string
	SourceGoalRef     string
	SourceSession     string
	ProjectID         *int64
	WorkItemID        int64
	DeliverableID     int64
	Objective         string
	OverwriteExisting bool
	Profile           string
	Reason            string
	ActorProfile      string
	ThreadTitle       string
	InviteProfiles    []string
	InviteHumans      []string
	ForceNew          bool
	Description       string
	Context           string
	OwnerID           string
	JSON              bool
}

type orchestrateService interface {
	CreateTask(ctx context.Context, input orchestrateapp.CreateTaskInput) (*orchestrateapp.CreateTaskResult, error)
	FollowUpTask(ctx context.Context, input orchestrateapp.FollowUpTaskInput) (*orchestrateapp.FollowUpTaskResult, error)
	AdoptDeliverable(ctx context.Context, input orchestrateapp.AdoptDeliverableInput) (*orchestrateapp.AdoptDeliverableResult, error)
	ReassignTask(ctx context.Context, input orchestrateapp.ReassignTaskInput) (*orchestrateapp.ReassignTaskResult, error)
	DecomposeTask(ctx context.Context, input orchestrateapp.DecomposeTaskInput) (*orchestrateapp.DecomposeTaskResult, error)
	EscalateThread(ctx context.Context, input orchestrateapp.EscalateThreadInput) (*orchestrateapp.EscalateThreadResult, error)
}

type ceoService interface {
	Submit(ctx context.Context, input ceoapp.SubmitInput) (*ceoapp.SubmitResult, error)
}

type orchestrateRuntime struct {
	service orchestrateService
	ceo     ceoService
	close   func() error
}

type orchestrateResult struct {
	OK                  bool                `json:"ok"`
	Action              string              `json:"action"`
	Summary             string              `json:"summary"`
	WorkItemID          int64               `json:"work_item_id,omitempty"`
	ThreadID            int64               `json:"thread_id,omitempty"`
	Created             bool                `json:"created,omitempty"`
	Status              core.WorkItemStatus `json:"status,omitempty"`
	Blocked             bool                `json:"blocked,omitempty"`
	ActiveProfile       string              `json:"active_profile,omitempty"`
	Mode                string              `json:"mode,omitempty"`
	RecommendedNextStep string              `json:"recommended_next_step,omitempty"`
	ActionCount         int                 `json:"action_count,omitempty"`
	LatestRunSummary    string              `json:"latest_run_summary,omitempty"`
	LatestSummarySource string              `json:"latest_summary_source,omitempty"`
	Profile             string              `json:"profile,omitempty"`
	Reason              string              `json:"reason,omitempty"`
	OldProfile          string              `json:"old_profile,omitempty"`
	NewProfile          string              `json:"new_profile,omitempty"`
	DeliverableID       int64               `json:"deliverable_id,omitempty"`
	FinalDeliverableID  *int64              `json:"final_deliverable_id,omitempty"`
	HasFinalDeliverable bool                `json:"has_final_deliverable,omitempty"`
}

var newOrchestrateRuntime = defaultNewOrchestrateRuntime

func RunOrchestrate(args []string) error {
	runtime, err := newOrchestrateRuntime()
	if err != nil {
		return err
	}
	if runtime != nil && runtime.close != nil {
		defer runtime.close()
	}
	return runOrchestrateToWriterWithServices(os.Stdout, runtime.service, runtime.ceo, args)
}

func runOrchestrateToWriter(out io.Writer, svc orchestrateService, args []string) error {
	return runOrchestrateToWriterWithServices(out, svc, nil, args)
}

func runOrchestrateToWriterWithServices(out io.Writer, svc orchestrateService, ceoSvc ceoService, args []string) error {
	opts, err := parseOrchestrateArgs(args)
	if err != nil {
		return err
	}
	if opts.Action == "ceo.submit" && ceoSvc == nil {
		return fmt.Errorf("ceo service is not configured")
	}
	if opts.Action != "ceo.submit" && svc == nil {
		return fmt.Errorf("orchestration service is not configured")
	}
	if out == nil {
		out = io.Discard
	}
	return executeOrchestrateAction(context.Background(), out, svc, ceoSvc, opts)
}

func parseOrchestrateArgs(args []string) (orchestrateCLIOptions, error) {
	if len(args) < 2 {
		return orchestrateCLIOptions{}, fmt.Errorf("usage: ai-flow orchestrate <task|ceo> ...")
	}

	opts := orchestrateCLIOptions{JSON: true}
	switch strings.TrimSpace(args[0]) {
	case "task":
		switch strings.TrimSpace(args[1]) {
		case "create":
			opts.Action = "task.create"
		case "follow-up":
			opts.Action = "task.follow-up"
		case "adopt-deliverable":
			opts.Action = "task.adopt-deliverable"
		case "assign-profile":
			opts.Action = "task.assign-profile"
		case "reassign":
			opts.Action = "task.reassign"
		case "decompose":
			opts.Action = "task.decompose"
		case "escalate-thread":
			opts.Action = "task.escalate-thread"
		default:
			return orchestrateCLIOptions{}, fmt.Errorf("usage: ai-flow orchestrate task <create|follow-up|adopt-deliverable|assign-profile|reassign|decompose|escalate-thread> [flags]")
		}
	case "ceo":
		switch strings.TrimSpace(args[1]) {
		case "submit":
			opts.Action = "ceo.submit"
		default:
			return orchestrateCLIOptions{}, fmt.Errorf("usage: ai-flow orchestrate ceo submit [flags]")
		}
	default:
		return orchestrateCLIOptions{}, fmt.Errorf("usage: ai-flow orchestrate <task|ceo> ...")
	}

	for i := 2; i < len(args); i++ {
		arg := strings.TrimSpace(args[i])
		switch {
		case arg == "--title":
			value, next, err := nextArgValue(args, i, "--title")
			if err != nil {
				return orchestrateCLIOptions{}, err
			}
			opts.Title = value
			i = next
		case strings.HasPrefix(arg, "--title="):
			opts.Title = strings.TrimSpace(strings.TrimPrefix(arg, "--title="))
		case arg == "--body":
			value, next, err := nextArgValue(args, i, "--body")
			if err != nil {
				return orchestrateCLIOptions{}, err
			}
			opts.Body = value
			i = next
		case strings.HasPrefix(arg, "--body="):
			opts.Body = strings.TrimSpace(strings.TrimPrefix(arg, "--body="))
		case arg == "--priority":
			value, next, err := nextArgValue(args, i, "--priority")
			if err != nil {
				return orchestrateCLIOptions{}, err
			}
			opts.Priority = value
			i = next
		case strings.HasPrefix(arg, "--priority="):
			opts.Priority = strings.TrimSpace(strings.TrimPrefix(arg, "--priority="))
		case arg == "--labels":
			value, next, err := nextArgValue(args, i, "--labels")
			if err != nil {
				return orchestrateCLIOptions{}, err
			}
			opts.Labels = parseCSV(value)
			i = next
		case strings.HasPrefix(arg, "--labels="):
			opts.Labels = parseCSV(strings.TrimPrefix(arg, "--labels="))
		case arg == "--dedupe-key":
			value, next, err := nextArgValue(args, i, "--dedupe-key")
			if err != nil {
				return orchestrateCLIOptions{}, err
			}
			opts.DedupeKey = value
			i = next
		case strings.HasPrefix(arg, "--dedupe-key="):
			opts.DedupeKey = strings.TrimSpace(strings.TrimPrefix(arg, "--dedupe-key="))
		case arg == "--source-goal-ref":
			value, next, err := nextArgValue(args, i, "--source-goal-ref")
			if err != nil {
				return orchestrateCLIOptions{}, err
			}
			opts.SourceGoalRef = value
			i = next
		case strings.HasPrefix(arg, "--source-goal-ref="):
			opts.SourceGoalRef = strings.TrimSpace(strings.TrimPrefix(arg, "--source-goal-ref="))
		case arg == "--source-session":
			value, next, err := nextArgValue(args, i, "--source-session")
			if err != nil {
				return orchestrateCLIOptions{}, err
			}
			opts.SourceSession = value
			i = next
		case strings.HasPrefix(arg, "--source-session="):
			opts.SourceSession = strings.TrimSpace(strings.TrimPrefix(arg, "--source-session="))
		case arg == "--project-id":
			value, next, err := nextArgValue(args, i, "--project-id")
			if err != nil {
				return orchestrateCLIOptions{}, err
			}
			projectID, err := parsePositiveInt64(value, "--project-id")
			if err != nil {
				return orchestrateCLIOptions{}, err
			}
			opts.ProjectID = &projectID
			i = next
		case strings.HasPrefix(arg, "--project-id="):
			projectID, err := parsePositiveInt64(strings.TrimPrefix(arg, "--project-id="), "--project-id")
			if err != nil {
				return orchestrateCLIOptions{}, err
			}
			opts.ProjectID = &projectID
		case arg == "--work-item-id":
			value, next, err := nextArgValue(args, i, "--work-item-id")
			if err != nil {
				return orchestrateCLIOptions{}, err
			}
			workItemID, err := parsePositiveInt64(value, "--work-item-id")
			if err != nil {
				return orchestrateCLIOptions{}, err
			}
			opts.WorkItemID = workItemID
			i = next
		case strings.HasPrefix(arg, "--work-item-id="):
			workItemID, err := parsePositiveInt64(strings.TrimPrefix(arg, "--work-item-id="), "--work-item-id")
			if err != nil {
				return orchestrateCLIOptions{}, err
			}
			opts.WorkItemID = workItemID
		case arg == "--deliverable-id":
			value, next, err := nextArgValue(args, i, "--deliverable-id")
			if err != nil {
				return orchestrateCLIOptions{}, err
			}
			deliverableID, err := parsePositiveInt64(value, "--deliverable-id")
			if err != nil {
				return orchestrateCLIOptions{}, err
			}
			opts.DeliverableID = deliverableID
			i = next
		case strings.HasPrefix(arg, "--deliverable-id="):
			deliverableID, err := parsePositiveInt64(strings.TrimPrefix(arg, "--deliverable-id="), "--deliverable-id")
			if err != nil {
				return orchestrateCLIOptions{}, err
			}
			opts.DeliverableID = deliverableID
		case arg == "--objective":
			value, next, err := nextArgValue(args, i, "--objective")
			if err != nil {
				return orchestrateCLIOptions{}, err
			}
			opts.Objective = value
			i = next
		case strings.HasPrefix(arg, "--objective="):
			opts.Objective = strings.TrimSpace(strings.TrimPrefix(arg, "--objective="))
		case arg == "--overwrite-existing":
			opts.OverwriteExisting = true
		case arg == "--profile":
			value, next, err := nextArgValue(args, i, "--profile")
			if err != nil {
				return orchestrateCLIOptions{}, err
			}
			opts.Profile = value
			i = next
		case strings.HasPrefix(arg, "--profile="):
			opts.Profile = strings.TrimSpace(strings.TrimPrefix(arg, "--profile="))
		case arg == "--reason":
			value, next, err := nextArgValue(args, i, "--reason")
			if err != nil {
				return orchestrateCLIOptions{}, err
			}
			opts.Reason = value
			i = next
		case strings.HasPrefix(arg, "--reason="):
			opts.Reason = strings.TrimSpace(strings.TrimPrefix(arg, "--reason="))
		case arg == "--actor-profile":
			value, next, err := nextArgValue(args, i, "--actor-profile")
			if err != nil {
				return orchestrateCLIOptions{}, err
			}
			opts.ActorProfile = value
			i = next
		case strings.HasPrefix(arg, "--actor-profile="):
			opts.ActorProfile = strings.TrimSpace(strings.TrimPrefix(arg, "--actor-profile="))
		case arg == "--thread-title":
			value, next, err := nextArgValue(args, i, "--thread-title")
			if err != nil {
				return orchestrateCLIOptions{}, err
			}
			opts.ThreadTitle = value
			i = next
		case strings.HasPrefix(arg, "--thread-title="):
			opts.ThreadTitle = strings.TrimSpace(strings.TrimPrefix(arg, "--thread-title="))
		case arg == "--invite-profiles":
			value, next, err := nextArgValue(args, i, "--invite-profiles")
			if err != nil {
				return orchestrateCLIOptions{}, err
			}
			opts.InviteProfiles = parseCSV(value)
			i = next
		case strings.HasPrefix(arg, "--invite-profiles="):
			opts.InviteProfiles = parseCSV(strings.TrimPrefix(arg, "--invite-profiles="))
		case arg == "--invite-humans":
			value, next, err := nextArgValue(args, i, "--invite-humans")
			if err != nil {
				return orchestrateCLIOptions{}, err
			}
			opts.InviteHumans = parseCSV(value)
			i = next
		case strings.HasPrefix(arg, "--invite-humans="):
			opts.InviteHumans = parseCSV(strings.TrimPrefix(arg, "--invite-humans="))
		case arg == "--force-new":
			opts.ForceNew = true
		case arg == "--description":
			value, next, err := nextArgValue(args, i, "--description")
			if err != nil {
				return orchestrateCLIOptions{}, err
			}
			opts.Description = value
			i = next
		case strings.HasPrefix(arg, "--description="):
			opts.Description = strings.TrimSpace(strings.TrimPrefix(arg, "--description="))
		case arg == "--context":
			value, next, err := nextArgValue(args, i, "--context")
			if err != nil {
				return orchestrateCLIOptions{}, err
			}
			opts.Context = value
			i = next
		case strings.HasPrefix(arg, "--context="):
			opts.Context = strings.TrimSpace(strings.TrimPrefix(arg, "--context="))
		case arg == "--owner-id":
			value, next, err := nextArgValue(args, i, "--owner-id")
			if err != nil {
				return orchestrateCLIOptions{}, err
			}
			opts.OwnerID = value
			i = next
		case strings.HasPrefix(arg, "--owner-id="):
			opts.OwnerID = strings.TrimSpace(strings.TrimPrefix(arg, "--owner-id="))
		case arg == "--json":
			opts.JSON = true
		default:
			return orchestrateCLIOptions{}, fmt.Errorf("unknown flag: %s", arg)
		}
	}

	return opts, nil
}

func executeOrchestrateAction(ctx context.Context, out io.Writer, svc orchestrateService, ceoSvc ceoService, opts orchestrateCLIOptions) error {
	var result orchestrateResult
	var err error

	switch opts.Action {
	case "task.create":
		result, err = executeCreateTask(ctx, svc, opts)
	case "task.follow-up":
		result, err = executeFollowUpTask(ctx, svc, opts)
	case "task.adopt-deliverable":
		result, err = executeAdoptDeliverable(ctx, svc, opts)
	case "task.assign-profile", "task.reassign":
		result, err = executeReassignTask(ctx, svc, opts)
	case "task.decompose":
		result, err = executeDecomposeTask(ctx, svc, opts)
	case "task.escalate-thread":
		result, err = executeEscalateThread(ctx, svc, opts)
	case "ceo.submit":
		result, err = executeCEOSubmit(ctx, ceoSvc, opts)
	default:
		return fmt.Errorf("unsupported action: %s", opts.Action)
	}
	if err != nil {
		return err
	}

	enc := json.NewEncoder(out)
	enc.SetEscapeHTML(false)
	return enc.Encode(result)
}

func executeCEOSubmit(ctx context.Context, svc ceoService, opts orchestrateCLIOptions) (orchestrateResult, error) {
	resp, err := svc.Submit(ctx, ceoapp.SubmitInput{
		Description: opts.Description,
		Context:     opts.Context,
		OwnerID:     opts.OwnerID,
	})
	if err != nil {
		return orchestrateResult{}, err
	}
	result := orchestrateResult{
		OK:                  true,
		Action:              opts.Action,
		Summary:             resp.Summary,
		Mode:                string(resp.Mode),
		RecommendedNextStep: resp.NextStep,
		ActionCount:         resp.ActionCount,
	}
	if resp.Thread != nil {
		result.ThreadID = resp.Thread.ID
	}
	if resp.WorkItemID > 0 {
		result.WorkItemID = resp.WorkItemID
	}
	return result, nil
}

func executeCreateTask(ctx context.Context, svc orchestrateService, opts orchestrateCLIOptions) (orchestrateResult, error) {
	resp, err := svc.CreateTask(ctx, orchestrateapp.CreateTaskInput{
		Title:               opts.Title,
		Body:                opts.Body,
		ProjectID:           opts.ProjectID,
		Priority:            opts.Priority,
		Labels:              cloneStrings(opts.Labels),
		DedupeKey:           opts.DedupeKey,
		SourceChatSessionID: opts.SourceSession,
		SourceGoalRef:       opts.SourceGoalRef,
	})
	if err != nil {
		return orchestrateResult{}, err
	}
	summary := "created work item"
	if !resp.Created {
		summary = "reused existing open work item"
	}
	return orchestrateResult{
		OK:         true,
		Action:     opts.Action,
		Summary:    summary,
		WorkItemID: resp.WorkItem.ID,
		Created:    resp.Created,
	}, nil
}

func executeFollowUpTask(ctx context.Context, svc orchestrateService, opts orchestrateCLIOptions) (orchestrateResult, error) {
	resp, err := svc.FollowUpTask(ctx, orchestrateapp.FollowUpTaskInput{WorkItemID: opts.WorkItemID})
	if err != nil {
		return orchestrateResult{}, err
	}
	return orchestrateResult{
		OK:                  true,
		Action:              opts.Action,
		Summary:             "fetched work item follow-up",
		WorkItemID:          resp.WorkItemID,
		Status:              resp.Status,
		Blocked:             resp.Blocked,
		ActiveProfile:       resp.ActiveProfileID,
		RecommendedNextStep: resp.RecommendedNextStep,
		LatestRunSummary:    resp.LatestRunSummary,
		LatestSummarySource: resp.LatestSummarySource,
		FinalDeliverableID:  resp.FinalDeliverableID,
		HasFinalDeliverable: resp.HasFinalDeliverable,
	}, nil
}

func executeAdoptDeliverable(ctx context.Context, svc orchestrateService, opts orchestrateCLIOptions) (orchestrateResult, error) {
	resp, err := svc.AdoptDeliverable(ctx, orchestrateapp.AdoptDeliverableInput{
		WorkItemID:    opts.WorkItemID,
		DeliverableID: opts.DeliverableID,
		ActorProfile:  opts.ActorProfile,
		SourceSession: opts.SourceSession,
	})
	if err != nil {
		return orchestrateResult{}, err
	}
	return orchestrateResult{
		OK:                  true,
		Action:              opts.Action,
		Summary:             "adopted final deliverable",
		WorkItemID:          resp.WorkItemID,
		DeliverableID:       resp.DeliverableID,
		Status:              resp.Status,
		FinalDeliverableID:  resp.FinalDeliverableID,
		HasFinalDeliverable: resp.FinalDeliverableID != nil,
	}, nil
}

func executeReassignTask(ctx context.Context, svc orchestrateService, opts orchestrateCLIOptions) (orchestrateResult, error) {
	resp, err := svc.ReassignTask(ctx, orchestrateapp.ReassignTaskInput{
		WorkItemID:    opts.WorkItemID,
		NewProfile:    opts.Profile,
		Reason:        opts.Reason,
		ActorProfile:  opts.ActorProfile,
		SourceSession: opts.SourceSession,
	})
	if err != nil {
		return orchestrateResult{}, err
	}
	summary := "reassigned work item profile"
	if opts.Action == "task.assign-profile" {
		summary = "assigned preferred profile"
	}
	return orchestrateResult{
		OK:         true,
		Action:     opts.Action,
		Summary:    summary,
		WorkItemID: resp.WorkItemID,
		OldProfile: resp.OldProfile,
		NewProfile: resp.NewProfile,
		Profile:    resp.NewProfile,
		Reason:     opts.Reason,
	}, nil
}

func executeDecomposeTask(ctx context.Context, svc orchestrateService, opts orchestrateCLIOptions) (orchestrateResult, error) {
	resp, err := svc.DecomposeTask(ctx, orchestrateapp.DecomposeTaskInput{
		WorkItemID:        opts.WorkItemID,
		Objective:         opts.Objective,
		OverwriteExisting: opts.OverwriteExisting,
	})
	if err != nil {
		return orchestrateResult{}, err
	}
	return orchestrateResult{
		OK:          true,
		Action:      opts.Action,
		Summary:     "materialized execution actions",
		WorkItemID:  resp.WorkItemID,
		ActionCount: resp.ActionCount,
	}, nil
}

func executeEscalateThread(ctx context.Context, svc orchestrateService, opts orchestrateCLIOptions) (orchestrateResult, error) {
	resp, err := svc.EscalateThread(ctx, orchestrateapp.EscalateThreadInput{
		WorkItemID:     opts.WorkItemID,
		Reason:         opts.Reason,
		ThreadTitle:    opts.ThreadTitle,
		ActorProfile:   opts.ActorProfile,
		SourceSession:  opts.SourceSession,
		InviteProfiles: cloneStrings(opts.InviteProfiles),
		InviteHumans:   cloneStrings(opts.InviteHumans),
		ForceNew:       opts.ForceNew,
	})
	if err != nil {
		return orchestrateResult{}, err
	}
	summary := "reused existing active thread"
	if resp.Created {
		summary = "created escalation thread"
	}
	return orchestrateResult{
		OK:         true,
		Action:     opts.Action,
		Summary:    summary,
		WorkItemID: resp.WorkItemID,
		ThreadID:   resp.Thread.ID,
		Created:    resp.Created,
	}, nil
}

func defaultNewOrchestrateRuntime() (*orchestrateRuntime, error) {
	cfg, dataDir, _, err := LoadConfig()
	if err != nil {
		return nil, err
	}

	skillsRoot := filepath.Join(dataDir, "skills")
	if err := skills.EnsureBuiltinSkills(skillsRoot); err != nil {
		return nil, fmt.Errorf("ensure builtin skills: %w", err)
	}

	storePath := ExpandStorePath(cfg.Store.Path, dataDir)
	runtimeDBPath := strings.TrimSuffix(storePath, filepath.Ext(storePath)) + "_runtime.db"
	store, err := sqlite.New(runtimeDBPath)
	if err != nil {
		return nil, fmt.Errorf("open runtime store: %w", err)
	}

	bootstrap.SeedRegistry(context.Background(), store, cfg)

	workItems := workitemapp.New(workitemapp.Config{Store: store, Registry: store})
	threads := threadapp.New(threadapp.Config{Store: store})

	var plannerSvc orchestrateapp.Planner
	if planner := newPlanningService(cfg, store, skillsRoot); planner != nil {
		plannerSvc = planner
	}

	service := orchestrateapp.New(orchestrateapp.Config{
		Store:           store,
		WorkItemCreator: workItems,
		Deliverables:    workItems,
		Planner:         plannerSvc,
		Threads:         threads,
		Registry:        store,
	})
	requirements := requirementapp.New(requirementapp.Config{
		Store:         store,
		Registry:      store,
		ThreadService: threads,
		LLM:           newRequirementCompleter(cfg),
	})

	return &orchestrateRuntime{
		service: service,
		ceo:     ceoapp.New(ceoapp.Config{Requirements: requirements, Tasks: service}),
		close:   store.Close,
	}, nil
}

func newPlanningService(cfg *config.Config, store *sqlite.Store, skillsRoot string) *planning.Service {
	llmCfg, ok := resolveOrchestrateLLMConfig(cfg)
	if !ok {
		return nil
	}
	client, err := llm.New(llmCfg)
	if err != nil {
		return nil
	}
	return planning.NewService(llmplanning.NewCompleter(client), store, planning.WithPlanningSkillsRoot(skillsRoot))
}

func newRequirementCompleter(cfg *config.Config) requirementapp.LLMCompleter {
	llmCfg, ok := resolveOrchestrateLLMConfig(cfg)
	if !ok {
		return nil
	}
	client, err := llm.New(llmCfg)
	if err != nil {
		return nil
	}
	return llmplanning.NewCompleter(client)
}

func resolveOrchestrateLLMConfig(cfg *config.Config) (llm.Config, bool) {
	if cfg == nil {
		return llm.Config{}, false
	}

	defaultID := strings.TrimSpace(cfg.Runtime.LLM.DefaultConfigID)
	if defaultID != "" {
		for _, entry := range cfg.Runtime.LLM.Configs {
			if strings.TrimSpace(entry.ID) != defaultID {
				continue
			}
			return llmConfigFromRuntimeEntry(entry)
		}
		return llm.Config{}, false
	}

	for _, entry := range cfg.Runtime.LLM.Configs {
		if llmCfg, ok := llmConfigFromRuntimeEntry(entry); ok {
			return llmCfg, true
		}
	}
	return llm.Config{}, false
}

func llmConfigFromRuntimeEntry(entry config.RuntimeLLMEntryConfig) (llm.Config, bool) {
	provider := strings.TrimSpace(entry.Type)
	switch provider {
	case "", llm.ProviderOpenAIResponse, llm.ProviderOpenAIChatCompletion, llm.ProviderAnthropic:
	default:
		return llm.Config{}, false
	}
	if strings.TrimSpace(entry.APIKey) == "" || strings.TrimSpace(entry.Model) == "" {
		return llm.Config{}, false
	}
	return llm.Config{
		Provider:             provider,
		BaseURL:              strings.TrimSpace(entry.BaseURL),
		APIKey:               strings.TrimSpace(entry.APIKey),
		Model:                strings.TrimSpace(entry.Model),
		Temperature:          entry.Temperature,
		MaxOutputTokens:      max(0, entry.MaxOutputTokens),
		ReasoningEffort:      strings.TrimSpace(entry.ReasoningEffort),
		ThinkingBudgetTokens: max(0, entry.ThinkingBudgetTokens),
	}, true
}

func nextArgValue(args []string, index int, flag string) (string, int, error) {
	index++
	if index >= len(args) {
		return "", index, fmt.Errorf("missing value for %s", flag)
	}
	return strings.TrimSpace(args[index]), index, nil
}

func parsePositiveInt64(raw string, flag string) (int64, error) {
	value, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("invalid value for %s: %s", flag, raw)
	}
	return value, nil
}

func parseCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" {
			continue
		}
		out = append(out, value)
	}
	return out
}

func cloneStrings(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, len(in))
	copy(out, in)
	return out
}
