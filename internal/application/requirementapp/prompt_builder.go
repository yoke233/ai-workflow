package requirementapp

import (
	"fmt"
	"strings"

	planningapp "github.com/yoke233/zhanggui/internal/application/planning"
	"github.com/yoke233/zhanggui/internal/core"
)

func buildAnalyzePrompt(input AnalyzeInput, projects []*core.Project, profiles []*core.AgentProfile) string {
	var b strings.Builder
	b.WriteString("你是需求分析与讨论路由助手。你的目标是帮助用户把一句自然语言需求，整理成适合创建 thread 讨论的建议。\n")
	b.WriteString("请给出清晰、可执行的建议，但不要假装确定你不知道的信息。更偏向提供有用的判断，而不是机械套规则。\n\n")

	if len(projects) > 0 {
		b.WriteString("当前项目：\n")
		for _, project := range projects {
			if project == nil {
				continue
			}
			fmt.Fprintf(&b, "- [%d] %s (%s)\n", project.ID, project.Name, project.Kind)
			if desc := strings.TrimSpace(project.Description); desc != "" {
				fmt.Fprintf(&b, "  描述: %s\n", desc)
			}
			if meta := strings.TrimSpace(project.Metadata[core.ProjectMetaScope]); meta != "" {
				fmt.Fprintf(&b, "  scope: %s\n", meta)
			}
			if meta := strings.TrimSpace(project.Metadata[core.ProjectMetaTechStack]); meta != "" {
				fmt.Fprintf(&b, "  tech_stack: %s\n", meta)
			}
			if meta := strings.TrimSpace(project.Metadata[core.ProjectMetaKeywords]); meta != "" {
				fmt.Fprintf(&b, "  keywords: %s\n", meta)
			}
			if meta := strings.TrimSpace(project.Metadata[core.ProjectMetaDependsOn]); meta != "" {
				fmt.Fprintf(&b, "  depends_on: %s\n", meta)
			}
			if meta := strings.TrimSpace(project.Metadata[core.ProjectMetaAgentHints]); meta != "" {
				fmt.Fprintf(&b, "  agent_hints: %s\n", meta)
			}
		}
		b.WriteString("\n")
	}

	if len(profiles) > 0 {
		b.WriteString("可用 Agent Profiles：\n")
		for _, profile := range profiles {
			if profile == nil {
				continue
			}
			caps := "none"
			if len(profile.Capabilities) > 0 {
				caps = strings.Join(profile.Capabilities, ", ")
			}
			fmt.Fprintf(&b, "- %s (role=%s, capabilities=[%s])\n", profile.ID, profile.Role, caps)
		}
		b.WriteString("\n")
	}

	b.WriteString("用户需求描述：\n---\n")
	b.WriteString(strings.TrimSpace(input.Description))
	b.WriteString("\n---\n")
	if ctx := strings.TrimSpace(input.Context); ctx != "" {
		b.WriteString("\n补充上下文：\n---\n")
		b.WriteString(ctx)
		b.WriteString("\n---\n")
	}
	b.WriteString("\n请分析：\n")
	b.WriteString("1. 用一句话总结需求。\n")
	b.WriteString("2. 判断这是 single_project / cross_project / new_project。\n")
	b.WriteString("3. 推荐最相关的项目，并说明理由与建议范围。\n")
	b.WriteString("4. 推荐适合参与讨论的 agent。\n")
	b.WriteString("5. 估计复杂度 low / medium / high。\n")
	b.WriteString("6. 推荐 discussion mode：direct / concurrent / group_chat。\n")
	b.WriteString("7. 提醒关键风险。\n")
	b.WriteString("8. 给出建议创建的 thread 标题、关联项目、参与 agents 与 meeting_max_rounds。\n")
	b.WriteString("如果无法确定，宁可少选并在 reason 中说明不确定性。\n")
	return b.String()
}

func buildAnalyzeSchema(projects []*core.Project, profiles []*core.AgentProfile) []planningapp.ToolDef {
	projectIDs := make([]int64, 0, len(projects))
	for _, project := range projects {
		if project != nil && project.ID > 0 {
			projectIDs = append(projectIDs, project.ID)
		}
	}
	agentIDs := make([]string, 0, len(profiles))
	for _, profile := range profiles {
		if profile != nil && strings.TrimSpace(profile.ID) != "" {
			agentIDs = append(agentIDs, profile.ID)
		}
	}

	projectIDSchema := map[string]any{"type": "integer"}
	if len(projectIDs) > 0 {
		projectIDSchema["enum"] = projectIDs
	}
	agentIDSchema := map[string]any{"type": "string"}
	if len(agentIDs) > 0 {
		agentIDSchema["enum"] = agentIDs
	}

	return []planningapp.ToolDef{{
		Name:        "analyze_requirement",
		Description: "Analyze a natural language requirement and propose thread routing details.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"summary": map[string]any{"type": "string"},
				"type": map[string]any{
					"type": "string",
					"enum": []string{"single_project", "cross_project", "new_project"},
				},
				"matched_projects": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"project_id":      projectIDSchema,
							"reason":          map[string]any{"type": "string"},
							"relevance":       map[string]any{"type": "string", "enum": []string{"high", "medium", "low"}},
							"suggested_scope": map[string]any{"type": "string"},
						},
						"required": []string{"project_id"},
					},
				},
				"suggested_agents": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"profile_id": agentIDSchema,
							"reason":     map[string]any{"type": "string"},
						},
						"required": []string{"profile_id"},
					},
				},
				"complexity": map[string]any{
					"type": "string",
					"enum": []string{"low", "medium", "high"},
				},
				"suggested_meeting_mode": map[string]any{
					"type": "string",
					"enum": []string{"direct", "concurrent", "group_chat"},
				},
				"risks": map[string]any{
					"type":  "array",
					"items": map[string]any{"type": "string"},
				},
				"suggested_thread": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"title": map[string]any{"type": "string"},
						"context_refs": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"project_id": projectIDSchema,
									"access": map[string]any{
										"type": "string",
										"enum": []string{"read", "check", "write"},
									},
								},
								"required": []string{"project_id"},
							},
						},
						"agents": map[string]any{
							"type":  "array",
							"items": agentIDSchema,
						},
						"meeting_mode": map[string]any{
							"type": "string",
							"enum": []string{"direct", "concurrent", "group_chat"},
						},
						"meeting_max_rounds": map[string]any{
							"type":    "integer",
							"minimum": 1,
							"maximum": 12,
						},
					},
				},
			},
			"required": []string{"summary", "type"},
		},
	}}
}
