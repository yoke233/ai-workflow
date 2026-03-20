package api

import (
	"strings"
	"testing"

	"github.com/yoke233/zhanggui/internal/core"
)

func TestBuildDirectThreadDispatchPrompt_EncouragesRelayCollaboration(t *testing.T) {
	out := buildDirectThreadDispatchPrompt(&core.ThreadMessage{
		Content: "@worker-a 请先看接口异常，再决定是否需要后端继续跟进",
	}, "worker-a", "worker-a")

	for _, want := range []string{
		"当前接到的一棒",
		"明确说明你在等谁/等什么",
		"说明是否需要谁继续接力",
		"不要试图一条消息包办所有人的工作",
		"去掉 @mention 后你需要处理的内容",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("prompt should contain %q.\nPrompt:\n%s", want, out)
		}
	}
}

func TestBuildConcurrentMeetingPrompt_EncouragesRelayCollaboration(t *testing.T) {
	out := buildConcurrentMeetingPrompt(&core.ThreadMessage{
		Content: "请并行分析这个问题",
	}, []string{"worker-a", "worker-b"})

	for _, want := range []string{
		"会议模式：concurrent",
		"你这一棒的结论、风险和下一步建议",
		"不要替其他参与者下未验证的结论",
		"明确写出你在等谁/等什么",
		"建议某位参与者继续接力",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("prompt should contain %q.\nPrompt:\n%s", want, out)
		}
	}
}

func TestBuildGroupChatMeetingPrompt_EncouragesRelayCollaboration(t *testing.T) {
	out := buildGroupChatMeetingPrompt(&core.ThreadMessage{
		Content: "请轮流讨论这个问题",
	}, []string{"worker-a", "worker-b"}, []threadMeetingTurn{
		{ProfileID: "worker-a", Content: "我先排查前端状态。", Round: 1},
	}, 2, 4, "worker-b")

	for _, want := range []string{
		"会议模式：group_chat",
		"优先补充新信息、收敛分歧或明确接力关系",
		"不要试图一条消息包办全部讨论",
		"说明你在等谁",
		"最适合由某位参与者接力",
		"可审批提案",
		"WorkItem 草案",
		"本次会议已有发言",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("prompt should contain %q.\nPrompt:\n%s", want, out)
		}
	}
}
