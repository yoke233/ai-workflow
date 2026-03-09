package teamleader

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/yoke233/ai-workflow/internal/core"
)

const decomposeSystemPrompt = `你是技术项目主管。用户给了一个需求，请分解成多个独立可执行的任务。

规则：
1. 每个任务应该是一个独立的代码变更，可以独立开发和测试
2. 明确任务之间的依赖关系（哪个任务必须先完成）
3. 尽量让无依赖的任务可以并行执行
4. 每个任务给出清晰的标题和描述，描述中包含验收标准
5. 输出纯 JSON，不要其他文字

输出格式：
{
  "summary": "方案概述（一句话）",
  "issues": [
    {
      "temp_id": "A",
      "title": "任务标题",
      "body": "任务描述，包含验收标准",
      "depends_on": [],
      "labels": ["backend", "frontend", "test"]
    }
  ]
}`

type DecomposePlanner struct {
	chatFn func(ctx context.Context, systemPrompt, userMessage string) (string, error)
}

func NewDecomposePlanner(chatFn func(ctx context.Context, systemPrompt, userMessage string) (string, error)) *DecomposePlanner {
	return &DecomposePlanner{chatFn: chatFn}
}

func (p *DecomposePlanner) Plan(ctx context.Context, projectID, prompt string) (*core.DecomposeProposal, error) {
	if p == nil || p.chatFn == nil {
		return nil, fmt.Errorf("decompose planner is not configured")
	}
	reply, err := p.chatFn(ctx, decomposeSystemPrompt, prompt)
	if err != nil {
		return nil, err
	}
	return parseDecomposeResponse(projectID, prompt, reply)
}

func parseDecomposeResponse(projectID, prompt, raw string) (*core.DecomposeProposal, error) {
	payload := strings.TrimSpace(raw)
	if payload == "" {
		return nil, fmt.Errorf("empty decompose response")
	}
	jsonPayload := extractJSONPayload(payload)
	if jsonPayload == "" {
		return nil, fmt.Errorf("decompose response does not contain json object")
	}

	var decoded struct {
		Summary string              `json:"summary"`
		Items   []core.ProposalItem `json:"issues"`
	}
	if err := json.Unmarshal([]byte(jsonPayload), &decoded); err != nil {
		return nil, fmt.Errorf("decode decompose response: %w", err)
	}
	proposal := &core.DecomposeProposal{
		ID:        core.NewProposalID(),
		ProjectID: strings.TrimSpace(projectID),
		Prompt:    strings.TrimSpace(prompt),
		Summary:   strings.TrimSpace(decoded.Summary),
		Items:     decoded.Items,
		CreatedAt: time.Now().UTC(),
	}
	if err := proposal.Validate(); err != nil {
		return nil, err
	}
	return proposal, nil
}

var jsonCodeBlockPattern = regexp.MustCompile("(?s)```(?:json)?\\s*(\\{.*?\\})\\s*```")

func extractJSONPayload(raw string) string {
	if matches := jsonCodeBlockPattern.FindStringSubmatch(raw); len(matches) == 2 {
		return strings.TrimSpace(matches[1])
	}
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start >= 0 && end > start {
		return strings.TrimSpace(raw[start : end+1])
	}
	return ""
}
