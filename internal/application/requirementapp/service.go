package requirementapp

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode"

	threadapp "github.com/yoke233/zhanggui/internal/application/threadapp"
	"github.com/yoke233/zhanggui/internal/core"
)

type llmAnalyzePayload struct {
	Summary         string `json:"summary"`
	Type            string `json:"type"`
	MatchedProjects []struct {
		ProjectID      int64  `json:"project_id"`
		Reason         string `json:"reason"`
		Relevance      string `json:"relevance"`
		SuggestedScope string `json:"suggested_scope"`
	} `json:"matched_projects"`
	SuggestedAgents []struct {
		ProfileID string `json:"profile_id"`
		Reason    string `json:"reason"`
	} `json:"suggested_agents"`
	Complexity           string   `json:"complexity"`
	SuggestedMeetingMode string   `json:"suggested_meeting_mode"`
	Risks                []string `json:"risks"`
	SuggestedThread      struct {
		Title            string                `json:"title"`
		ContextRefs      []SuggestedContextRef `json:"context_refs"`
		Agents           []string              `json:"agents"`
		MeetingMode      string                `json:"meeting_mode"`
		MeetingMaxRounds int                   `json:"meeting_max_rounds"`
	} `json:"suggested_thread"`
}

func (s *Service) Analyze(ctx context.Context, input AnalyzeInput) (*AnalyzeResult, error) {
	description := strings.TrimSpace(input.Description)
	if description == "" {
		return nil, fmt.Errorf("description is required")
	}
	projects, err := s.store.ListProjects(ctx, 500, 0)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	profiles, err := s.listProfiles(ctx)
	if err != nil {
		return nil, fmt.Errorf("list profiles: %w", err)
	}

	if s.llm != nil {
		prompt := buildAnalyzePrompt(input, projects, profiles)
		raw, llmErr := s.llm.Complete(ctx, prompt, buildAnalyzeSchema(projects, profiles))
		if llmErr == nil {
			var payload llmAnalyzePayload
			if err := json.Unmarshal(raw, &payload); err == nil {
				result := normalizeLLMAnalysis(payload, projects, profiles, input)
				return &result, nil
			}
		}
	}

	result := buildHeuristicAnalysis(input, projects, profiles)
	return &result, nil
}

func (s *Service) CreateThread(ctx context.Context, input CreateThreadInput) (*CreateThreadResult, error) {
	if s.threadService == nil {
		return nil, fmt.Errorf("thread service is not configured")
	}
	description := strings.TrimSpace(input.Description)
	if description == "" {
		return nil, fmt.Errorf("description is required")
	}

	threadCfg := normalizeThreadConfig(input.ThreadConfig, input.Analysis, description)
	createResult, err := s.threadService.CreateThread(ctx, threadapp.CreateThreadInput{
		Title:   threadCfg.Title,
		OwnerID: strings.TrimSpace(input.OwnerID),
		Metadata: map[string]any{
			"meeting_mode":              threadCfg.MeetingMode,
			"meeting_max_rounds":        threadCfg.MeetingMaxRounds,
			"agent_routing_mode":        "auto",
			"source":                    "requirements.create_thread",
			"skip_default_context_refs": true,
		},
	})
	if err != nil {
		return nil, err
	}

	thread := createResult.Thread
	existingRefs, err := s.store.ListThreadContextRefs(ctx, thread.ID)
	if err != nil {
		return nil, err
	}
	refs := make([]*core.ThreadContextRef, 0, len(threadCfg.ContextRefs)+len(existingRefs))
	seenProjects := make(map[int64]struct{}, len(threadCfg.ContextRefs)+len(existingRefs))
	for _, ref := range existingRefs {
		if ref == nil || ref.ProjectID <= 0 {
			continue
		}
		seenProjects[ref.ProjectID] = struct{}{}
		refs = append(refs, ref)
	}
	for _, ref := range threadCfg.ContextRefs {
		if ref.ProjectID <= 0 {
			continue
		}
		if _, exists := seenProjects[ref.ProjectID]; exists {
			continue
		}
		seenProjects[ref.ProjectID] = struct{}{}
		created, err := s.threadService.CreateThreadContextRef(ctx, threadapp.CreateThreadContextRefInput{
			ThreadID:  thread.ID,
			ProjectID: ref.ProjectID,
			Access:    normalizeAccess(ref.Access),
			GrantedBy: strings.TrimSpace(input.OwnerID),
			Note:      "requirements routing",
		})
		if err != nil {
			return nil, err
		}
		refs = append(refs, created)
	}

	agentIDs, err := s.normalizeAgentIDs(ctx, threadCfg.Agents)
	if err != nil {
		return nil, err
	}

	return &CreateThreadResult{
		Thread:      thread,
		ContextRefs: refs,
		AgentIDs:    agentIDs,
	}, nil
}

func (s *Service) listProfiles(ctx context.Context) ([]*core.AgentProfile, error) {
	if s.registry == nil {
		return nil, nil
	}
	return s.registry.ListProfiles(ctx)
}

func normalizeLLMAnalysis(payload llmAnalyzePayload, projects []*core.Project, profiles []*core.AgentProfile, input AnalyzeInput) AnalyzeResult {
	projectMap := make(map[int64]*core.Project, len(projects))
	for _, project := range projects {
		if project != nil {
			projectMap[project.ID] = project
		}
	}
	profileMap := make(map[string]*core.AgentProfile, len(profiles))
	for _, profile := range profiles {
		if profile != nil {
			profileMap[profile.ID] = profile
		}
	}

	matched := make([]MatchedProject, 0, len(payload.MatchedProjects))
	contextRefs := make([]SuggestedContextRef, 0, len(payload.MatchedProjects))
	for _, item := range payload.MatchedProjects {
		project := projectMap[item.ProjectID]
		if project == nil {
			continue
		}
		matched = append(matched, MatchedProject{
			ProjectID:      item.ProjectID,
			ProjectName:    project.Name,
			Relevance:      normalizeRelevance(item.Relevance),
			Reason:         strings.TrimSpace(item.Reason),
			SuggestedScope: strings.TrimSpace(item.SuggestedScope),
		})
		contextRefs = append(contextRefs, SuggestedContextRef{ProjectID: item.ProjectID, Access: "read"})
	}

	suggestedAgents := make([]SuggestedAgent, 0, len(payload.SuggestedAgents))
	agentIDs := make([]string, 0, len(payload.SuggestedAgents))
	seenAgents := map[string]struct{}{}
	for _, item := range payload.SuggestedAgents {
		profileID := strings.TrimSpace(item.ProfileID)
		if profileMap[profileID] == nil {
			continue
		}
		if _, exists := seenAgents[profileID]; exists {
			continue
		}
		seenAgents[profileID] = struct{}{}
		suggestedAgents = append(suggestedAgents, SuggestedAgent{
			ProfileID: profileID,
			Reason:    strings.TrimSpace(item.Reason),
		})
		agentIDs = append(agentIDs, profileID)
	}

	threadCfg := normalizeThreadConfig(SuggestedThread{
		Title:            strings.TrimSpace(payload.SuggestedThread.Title),
		ContextRefs:      payload.SuggestedThread.ContextRefs,
		Agents:           payload.SuggestedThread.Agents,
		MeetingMode:      payload.SuggestedThread.MeetingMode,
		MeetingMaxRounds: payload.SuggestedThread.MeetingMaxRounds,
	}, &AnalysisResult{
		Summary:              strings.TrimSpace(payload.Summary),
		Type:                 normalizeRequirementType(payload.Type, len(matched)),
		MatchedProjects:      matched,
		SuggestedAgents:      suggestedAgents,
		Complexity:           normalizeComplexity(payload.Complexity),
		SuggestedMeetingMode: normalizeMeetingMode(payload.SuggestedMeetingMode, len(agentIDs)),
		Risks:                normalizeStringList(payload.Risks),
	}, strings.TrimSpace(input.Description))
	if len(threadCfg.ContextRefs) == 0 {
		threadCfg.ContextRefs = contextRefs
	}
	if len(threadCfg.Agents) == 0 {
		threadCfg.Agents = agentIDs
	}

	return AnalyzeResult{
		Analysis: AnalysisResult{
			Summary:              firstNonEmpty(strings.TrimSpace(payload.Summary), deriveSummary(input.Description)),
			Type:                 normalizeRequirementType(payload.Type, len(matched)),
			MatchedProjects:      matched,
			SuggestedAgents:      suggestedAgents,
			Complexity:           normalizeComplexity(payload.Complexity),
			SuggestedMeetingMode: normalizeMeetingMode(payload.SuggestedMeetingMode, len(agentIDs)),
			Risks:                normalizeStringList(payload.Risks),
		},
		SuggestedThread: threadCfg,
	}
}

func buildHeuristicAnalysis(input AnalyzeInput, projects []*core.Project, profiles []*core.AgentProfile) AnalyzeResult {
	query := strings.ToLower(strings.TrimSpace(input.Description + " " + input.Context))
	projectScores := scoreProjects(query, projects)
	sort.SliceStable(projectScores, func(i, j int) bool {
		if projectScores[i].score == projectScores[j].score {
			return projectScores[i].project.ID < projectScores[j].project.ID
		}
		return projectScores[i].score > projectScores[j].score
	})

	limit := 1
	if len(projectScores) > 1 && projectScores[1].score > 0 {
		limit = 2
	}
	matched := make([]MatchedProject, 0, limit)
	contextRefs := make([]SuggestedContextRef, 0, limit)
	agentHints := make([]string, 0, limit)
	for _, item := range projectScores {
		if item.project == nil || item.score <= 0 {
			continue
		}
		matched = append(matched, MatchedProject{
			ProjectID:      item.project.ID,
			ProjectName:    item.project.Name,
			Relevance:      relevanceFromScore(item.score),
			Reason:         item.reason,
			SuggestedScope: strings.TrimSpace(item.project.Metadata[core.ProjectMetaScope]),
		})
		contextRefs = append(contextRefs, SuggestedContextRef{ProjectID: item.project.ID, Access: "read"})
		agentHints = append(agentHints, splitCSV(item.project.Metadata[core.ProjectMetaAgentHints])...)
		if len(matched) >= limit {
			break
		}
	}

	agents := scoreAgents(query, profiles, agentHints)
	suggestedAgents := make([]SuggestedAgent, 0, len(agents))
	agentIDs := make([]string, 0, len(agents))
	for _, item := range agents {
		suggestedAgents = append(suggestedAgents, SuggestedAgent{
			ProfileID: item.profile.ID,
			Reason:    item.reason,
		})
		agentIDs = append(agentIDs, item.profile.ID)
	}

	analysis := AnalysisResult{
		Summary:              deriveSummary(input.Description),
		Type:                 normalizeRequirementType("", len(matched)),
		MatchedProjects:      matched,
		SuggestedAgents:      suggestedAgents,
		Complexity:           deriveComplexity(input.Description, input.Context, len(matched)),
		SuggestedMeetingMode: normalizeMeetingMode("", maxInt(len(agentIDs), len(matched))),
	}
	if len(matched) == 0 {
		analysis.Type = "new_project"
		analysis.Risks = []string{"当前没有高置信度匹配项目，可能需要补充项目 metadata 或新建项目。"}
	} else if len(matched) > 1 {
		analysis.Risks = []string{"这是跨项目需求，建议先统一边界与依赖关系。"}
	}

	return AnalyzeResult{
		Analysis: analysis,
		SuggestedThread: normalizeThreadConfig(SuggestedThread{
			ContextRefs: contextRefs,
			Agents:      agentIDs,
		}, &analysis, strings.TrimSpace(input.Description)),
	}
}

type scoredProject struct {
	project *core.Project
	score   int
	reason  string
}

func scoreProjects(query string, projects []*core.Project) []scoredProject {
	queryTokens := tokenize(query)
	scores := make([]scoredProject, 0, len(projects))
	for _, project := range projects {
		if project == nil {
			continue
		}
		score := 0
		reasons := make([]string, 0, 3)
		fields := []struct {
			label  string
			text   string
			weight int
		}{
			{"name", project.Name, 5},
			{"description", project.Description, 3},
			{"scope", project.Metadata[core.ProjectMetaScope], 4},
			{"keywords", project.Metadata[core.ProjectMetaKeywords], 4},
			{"tech_stack", project.Metadata[core.ProjectMetaTechStack], 2},
		}
		for _, field := range fields {
			fieldText := strings.ToLower(strings.TrimSpace(field.text))
			if fieldText == "" {
				continue
			}
			for _, token := range queryTokens {
				if token == "" {
					continue
				}
				if strings.Contains(fieldText, token) {
					score += field.weight
					reasons = append(reasons, fmt.Sprintf("%s 命中 %q", field.label, token))
				}
			}
		}
		scores = append(scores, scoredProject{
			project: project,
			score:   score,
			reason:  firstNonEmpty(strings.Join(uniqueStrings(reasons), "；"), "项目描述与需求存在潜在关联"),
		})
	}
	return scores
}

type scoredAgent struct {
	profile *core.AgentProfile
	score   int
	reason  string
}

func scoreAgents(query string, profiles []*core.AgentProfile, hinted []string) []scoredAgent {
	queryTokens := tokenize(query)
	hintSet := make(map[string]struct{}, len(hinted))
	for _, item := range hinted {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			hintSet[trimmed] = struct{}{}
		}
	}
	scored := make([]scoredAgent, 0, len(profiles))
	for _, profile := range profiles {
		if profile == nil {
			continue
		}
		score := 0
		reasons := make([]string, 0, 3)
		if _, ok := hintSet[profile.ID]; ok {
			score += 5
			reasons = append(reasons, "项目 metadata 的 agent_hints 推荐")
		}
		for _, token := range queryTokens {
			if strings.Contains(strings.ToLower(profile.ID), token) || strings.Contains(strings.ToLower(profile.Name), token) {
				score += 3
				reasons = append(reasons, fmt.Sprintf("profile 标识命中 %q", token))
			}
			for _, cap := range profile.Capabilities {
				if strings.Contains(strings.ToLower(cap), token) {
					score += 4
					reasons = append(reasons, fmt.Sprintf("capability 命中 %q", token))
				}
			}
		}
		if score == 0 && profile.Role == core.RoleLead {
			score = 1
			reasons = append(reasons, "保留 lead 参与收敛讨论")
		}
		if score <= 0 {
			continue
		}
		scored = append(scored, scoredAgent{
			profile: profile,
			score:   score,
			reason:  strings.Join(uniqueStrings(reasons), "；"),
		})
	}
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score == scored[j].score {
			return scored[i].profile.ID < scored[j].profile.ID
		}
		return scored[i].score > scored[j].score
	})
	if len(scored) > 3 {
		scored = scored[:3]
	}
	return scored
}

func normalizeThreadConfig(cfg SuggestedThread, analysis *AnalysisResult, description string) SuggestedThread {
	out := SuggestedThread{
		Title:            strings.TrimSpace(cfg.Title),
		ContextRefs:      append([]SuggestedContextRef(nil), cfg.ContextRefs...),
		Agents:           append([]string(nil), cfg.Agents...),
		MeetingMode:      normalizeMeetingMode(cfg.MeetingMode, len(cfg.Agents)),
		MeetingMaxRounds: cfg.MeetingMaxRounds,
	}
	if analysis != nil {
		if out.Title == "" {
			out.Title = buildRequirementThreadTitle(firstNonEmpty(analysis.Summary, description))
		}
		if len(out.ContextRefs) == 0 {
			for _, item := range analysis.MatchedProjects {
				out.ContextRefs = append(out.ContextRefs, SuggestedContextRef{
					ProjectID: item.ProjectID,
					Access:    "read",
				})
			}
		}
		if len(out.Agents) == 0 {
			for _, item := range analysis.SuggestedAgents {
				out.Agents = append(out.Agents, item.ProfileID)
			}
		}
		if strings.TrimSpace(cfg.MeetingMode) == "" {
			out.MeetingMode = normalizeMeetingMode(analysis.SuggestedMeetingMode, maxInt(len(out.Agents), len(out.ContextRefs)))
		}
	}
	if out.Title == "" {
		out.Title = buildRequirementThreadTitle(description)
	}
	if out.MeetingMaxRounds <= 0 {
		switch out.MeetingMode {
		case "group_chat":
			out.MeetingMaxRounds = 6
		case "concurrent":
			out.MeetingMaxRounds = 4
		default:
			out.MeetingMaxRounds = 3
		}
	}
	if out.MeetingMaxRounds > 12 {
		out.MeetingMaxRounds = 12
	}
	out.ContextRefs = normalizeContextRefs(out.ContextRefs)
	out.Agents = normalizeStringList(out.Agents)
	return out
}

func normalizeContextRefs(refs []SuggestedContextRef) []SuggestedContextRef {
	out := make([]SuggestedContextRef, 0, len(refs))
	seen := make(map[int64]struct{}, len(refs))
	for _, ref := range refs {
		if ref.ProjectID <= 0 {
			continue
		}
		if _, exists := seen[ref.ProjectID]; exists {
			continue
		}
		seen[ref.ProjectID] = struct{}{}
		out = append(out, SuggestedContextRef{
			ProjectID: ref.ProjectID,
			Access:    normalizeAccess(ref.Access),
		})
	}
	return out
}

func (s *Service) normalizeAgentIDs(ctx context.Context, agentIDs []string) ([]string, error) {
	normalized := normalizeStringList(agentIDs)
	if s.registry == nil {
		return normalized, nil
	}
	for _, profileID := range normalized {
		if _, err := s.registry.ResolveByID(ctx, profileID); err != nil {
			return nil, fmt.Errorf("resolve profile %q: %w", profileID, err)
		}
	}
	return normalized, nil
}

func normalizeRequirementType(value string, matchedProjects int) string {
	switch strings.TrimSpace(value) {
	case "single_project", "cross_project", "new_project":
		return strings.TrimSpace(value)
	}
	switch {
	case matchedProjects > 1:
		return "cross_project"
	case matchedProjects == 1:
		return "single_project"
	default:
		return "new_project"
	}
}

func normalizeComplexity(value string) string {
	switch strings.TrimSpace(value) {
	case "low", "medium", "high":
		return strings.TrimSpace(value)
	default:
		return "medium"
	}
}

func normalizeMeetingMode(value string, participants int) string {
	switch strings.TrimSpace(value) {
	case "direct", "concurrent", "group_chat":
		return strings.TrimSpace(value)
	}
	switch {
	case participants >= 3:
		return "group_chat"
	case participants == 2:
		return "concurrent"
	default:
		return "direct"
	}
}

func normalizeRelevance(value string) string {
	switch strings.TrimSpace(value) {
	case "high", "medium", "low":
		return strings.TrimSpace(value)
	default:
		return "medium"
	}
}

func normalizeAccess(value string) string {
	switch strings.TrimSpace(value) {
	case "check", "write":
		return strings.TrimSpace(value)
	default:
		return "read"
	}
}

func normalizeStringList(items []string) []string {
	out := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func buildRequirementThreadTitle(value string) string {
	title := strings.TrimSpace(value)
	if title == "" {
		return "讨论：新需求"
	}
	title = strings.ReplaceAll(title, "\n", " ")
	if len([]rune(title)) > 48 {
		title = string([]rune(title)[:48])
	}
	return "讨论：" + title
}

func deriveSummary(value string) string {
	text := strings.TrimSpace(value)
	if text == "" {
		return "新需求"
	}
	for _, sep := range []string{"。", ".", "！", "!", "？", "?"} {
		if idx := strings.Index(text, sep); idx > 0 {
			return strings.TrimSpace(text[:idx])
		}
	}
	if len([]rune(text)) > 48 {
		return string([]rune(text)[:48])
	}
	return text
}

func deriveComplexity(description, context string, matchedProjects int) string {
	text := strings.TrimSpace(description + " " + context)
	switch {
	case matchedProjects > 1:
		return "high"
	case len([]rune(text)) > 120:
		return "high"
	case len([]rune(text)) > 50:
		return "medium"
	default:
		return "low"
	}
}

func tokenize(value string) []string {
	var b strings.Builder
	for _, r := range strings.ToLower(value) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			continue
		}
		b.WriteRune(' ')
	}
	parts := strings.Fields(b.String())
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if len(part) < 2 {
			continue
		}
		out = append(out, part)
	}
	return uniqueStrings(out)
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func uniqueStrings(items []string) []string {
	out := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		if _, exists := seen[item]; exists {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func relevanceFromScore(score int) string {
	switch {
	case score >= 9:
		return "high"
	case score >= 4:
		return "medium"
	default:
		return "low"
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
