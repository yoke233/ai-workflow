package requirementapp

import (
	"context"
	"encoding/json"

	planningapp "github.com/yoke233/zhanggui/internal/application/planning"
	threadapp "github.com/yoke233/zhanggui/internal/application/threadapp"
	"github.com/yoke233/zhanggui/internal/core"
)

type Store interface {
	core.ProjectStore
	core.ThreadStore
}

type ThreadService interface {
	CreateThread(ctx context.Context, input threadapp.CreateThreadInput) (*threadapp.CreateThreadResult, error)
	CreateThreadContextRef(ctx context.Context, input threadapp.CreateThreadContextRefInput) (*core.ThreadContextRef, error)
}

type LLMCompleter interface {
	Complete(ctx context.Context, prompt string, tools []planningapp.ToolDef) (json.RawMessage, error)
}

type Config struct {
	Store         Store
	Registry      core.AgentRegistry
	ThreadService ThreadService
	LLM           LLMCompleter
}

type Service struct {
	store         Store
	registry      core.AgentRegistry
	threadService ThreadService
	llm           LLMCompleter
}

type AnalyzeInput struct {
	Description string `json:"description"`
	Context     string `json:"context,omitempty"`
}

type MatchedProject struct {
	ProjectID      int64  `json:"project_id"`
	ProjectName    string `json:"project_name"`
	Relevance      string `json:"relevance,omitempty"`
	Reason         string `json:"reason,omitempty"`
	SuggestedScope string `json:"suggested_scope,omitempty"`
}

type SuggestedAgent struct {
	ProfileID string `json:"profile_id"`
	Reason    string `json:"reason,omitempty"`
}

type AnalysisResult struct {
	Summary              string           `json:"summary"`
	Type                 string           `json:"type"`
	MatchedProjects      []MatchedProject `json:"matched_projects,omitempty"`
	SuggestedAgents      []SuggestedAgent `json:"suggested_agents,omitempty"`
	Complexity           string           `json:"complexity,omitempty"`
	SuggestedMeetingMode string           `json:"suggested_meeting_mode,omitempty"`
	Risks                []string         `json:"risks,omitempty"`
}

type SuggestedContextRef struct {
	ProjectID int64  `json:"project_id"`
	Access    string `json:"access"`
}

type SuggestedThread struct {
	Title            string                `json:"title"`
	ContextRefs      []SuggestedContextRef `json:"context_refs,omitempty"`
	Agents           []string              `json:"agents,omitempty"`
	MeetingMode      string                `json:"meeting_mode,omitempty"`
	MeetingMaxRounds int                   `json:"meeting_max_rounds,omitempty"`
}

type AnalyzeResult struct {
	Analysis        AnalysisResult  `json:"analysis"`
	SuggestedThread SuggestedThread `json:"suggested_thread"`
}

type CreateThreadInput struct {
	Description  string          `json:"description"`
	Context      string          `json:"context,omitempty"`
	OwnerID      string          `json:"owner_id,omitempty"`
	Analysis     *AnalysisResult `json:"analysis,omitempty"`
	ThreadConfig SuggestedThread `json:"thread_config"`
}

type CreateThreadResult struct {
	Thread      *core.Thread             `json:"thread"`
	ContextRefs []*core.ThreadContextRef `json:"context_refs,omitempty"`
	AgentIDs    []string                 `json:"agents,omitempty"`
}

func New(cfg Config) *Service {
	return &Service{
		store:         cfg.Store,
		registry:      cfg.Registry,
		threadService: cfg.ThreadService,
		llm:           cfg.LLM,
	}
}
