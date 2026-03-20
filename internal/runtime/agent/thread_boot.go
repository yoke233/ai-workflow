package agentruntime

import (
	"fmt"
	"sort"
	"strings"

	"github.com/yoke233/zhanggui/internal/core"
)

// ThreadBootInput holds all context needed to assemble a boot prompt for an
// agent joining a Thread.
type ThreadBootInput struct {
	Thread         *core.Thread
	RecentMessages []*core.ThreadMessage
	Participants   []*core.ThreadMember
	WorkItems      []*core.WorkItem
	AgentProfile   *core.AgentProfile
	PriorSummary   string // progress_summary from a previous session (if resuming)
	Workspace      *core.ThreadWorkspaceContext
	SharedTemplate string
}

// BuildBootPrompt assembles a Markdown system prompt that orients an agent
// joining a Thread.  The prompt is intentionally concise to preserve prompt
// cache efficiency.
func BuildBootPrompt(in ThreadBootInput) string {
	var b strings.Builder

	if strings.TrimSpace(in.SharedTemplate) != "" {
		b.WriteString(strings.TrimSpace(in.SharedTemplate))
		b.WriteString("\n\n")
	}

	// Profile-specific template augments the shared thread collaboration rules.
	if in.AgentProfile != nil && strings.TrimSpace(in.AgentProfile.Session.ThreadBootTemplate) != "" {
		b.WriteString(strings.TrimSpace(in.AgentProfile.Session.ThreadBootTemplate))
		b.WriteString("\n\n")
	}

	// Thread context.
	b.WriteString("## Thread Context\n")
	if in.Thread != nil {
		fmt.Fprintf(&b, "**Title**: %s\n", in.Thread.Title)
		fmt.Fprintf(&b, "**Status**: %s\n", in.Thread.Status)
		if focusProjectID, ok := core.ReadThreadFocusProjectID(in.Thread); ok {
			fmt.Fprintf(&b, "**Focus Project ID**: %d\n", focusProjectID)
		}
	}
	if in.AgentProfile != nil {
		fmt.Fprintf(&b, "**Your Role**: %s (%s)\n", in.AgentProfile.Role, in.AgentProfile.ID)
	}
	b.WriteString("\n")

	// Recent conversation.
	if len(in.RecentMessages) > 0 {
		fmt.Fprintf(&b, "## Recent Conversation (last %d messages)\n", len(in.RecentMessages))
		for _, msg := range in.RecentMessages {
			sender := msg.SenderID
			if sender == "" {
				sender = "anonymous"
			}
			fmt.Fprintf(&b, "[%s] (%s): %s\n", sender, msg.Role, msg.Content)
		}
		b.WriteString("\n")
	}

	// Participants.
	if len(in.Participants) > 0 {
		b.WriteString("## Participants\n")
		for _, p := range in.Participants {
			marker := ""
			if in.AgentProfile != nil && p.UserID == in.AgentProfile.ID {
				marker = " ← you"
			}
			fmt.Fprintf(&b, "- %s (%s)%s\n", p.UserID, p.Role, marker)
		}
		b.WriteString("\n")
	}

	// Linked work items.
	if len(in.WorkItems) > 0 {
		b.WriteString("## Linked Work Items\n")
		for _, wi := range in.WorkItems {
			fmt.Fprintf(&b, "- #%d: %s [status: %s]\n", wi.ID, wi.Title, wi.Status)
		}
		b.WriteString("\n")
	}

	if in.Workspace != nil {
		b.WriteString("## Workspace Context\n")
		fmt.Fprintf(&b, "- Workspace: %s\n", strings.TrimSpace(in.Workspace.Workspace))
		if len(in.Workspace.Mounts) > 0 {
			keys := make([]string, 0, len(in.Workspace.Mounts))
			for key := range in.Workspace.Mounts {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				mount := in.Workspace.Mounts[key]
				fmt.Fprintf(&b, "- Mount %s => %s [%s]\n", key, mount.Path, mount.Access)
			}
		}
		if len(in.Workspace.Attachments) > 0 {
			b.WriteString("- Attachments:\n")
			for _, att := range in.Workspace.Attachments {
				label := att.FileName
				if att.IsDirectory {
					label += " (directory)"
				}
				if att.Note != "" {
					fmt.Fprintf(&b, "  - %s => %s — %s\n", label, att.FilePath, att.Note)
				} else {
					fmt.Fprintf(&b, "  - %s => %s\n", label, att.FilePath)
				}
			}
		}
		b.WriteString("\n")
	}

	// Prior context (resuming from a paused session).
	if strings.TrimSpace(in.PriorSummary) != "" {
		b.WriteString("## Prior Context (resuming)\n")
		b.WriteString(strings.TrimSpace(in.PriorSummary))
		b.WriteString("\n\n")
	}

	// Instructions.
	b.WriteString("## Instructions\n")
	b.WriteString("You are joining this thread. Review the context above, keep track of dependencies or hand-offs mentioned in the thread, and act directly whenever the runtime routes work to you.\n")

	return b.String()
}
