package web

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestCodexChatAssistantReplyBuildsNewSessionCommand(t *testing.T) {
	runner := &recordingClaudeRunner{
		stdout: strings.Join([]string{
			`{"type":"thread.started","thread_id":"thread-1"}`,
			`{"type":"item.completed","item":{"type":"agent_message","text":"hello from codex"}}`,
		}, "\n"),
	}
	assistant := newCodexChatAssistantForTest("codex", "gpt-5.3-codex", "high", runner)

	got, err := assistant.Reply(context.Background(), ChatAssistantRequest{
		Message: "hello",
		WorkDir: "D:/repo/demo",
	})
	if err != nil {
		t.Fatalf("Reply returned error: %v", err)
	}
	if got.Reply != "hello from codex" {
		t.Fatalf("expected reply %q, got %q", "hello from codex", got.Reply)
	}
	if got.AgentSessionID != "thread-1" {
		t.Fatalf("expected thread id %q, got %q", "thread-1", got.AgentSessionID)
	}

	if len(runner.calls) != 1 {
		t.Fatalf("expected one runner call, got %d", len(runner.calls))
	}
	call := runner.calls[0]
	if call.command != "codex" {
		t.Fatalf("expected codex command, got %s", call.command)
	}
	joined := strings.Join(call.args, " ")
	if !strings.Contains(joined, "-C D:/repo/demo") {
		t.Fatalf("expected -C workdir arg, got %v", call.args)
	}
	if !strings.Contains(joined, "-a never") {
		t.Fatalf("expected global -a never, got %v", call.args)
	}
	if !strings.Contains(joined, "exec --json") {
		t.Fatalf("expected exec prompt, got %v", call.args)
	}
	if !strings.Contains(joined, "--json") {
		t.Fatalf("expected --json arg, got %v", call.args)
	}
	if !strings.Contains(joined, "--disable shell_tool") {
		t.Fatalf("expected --disable shell_tool arg, got %v", call.args)
	}
	if !strings.Contains(joined, " hello ") && !strings.HasSuffix(joined, " hello") {
		t.Fatalf("expected message in args, got %v", call.args)
	}
}

func TestCodexChatAssistantReplyInjectsRoleContext(t *testing.T) {
	runner := &recordingClaudeRunner{
		stdout: strings.Join([]string{
			`{"type":"item.completed","item":{"type":"agent_message","text":"ok"}}`,
		}, "\n"),
	}
	assistant := newCodexChatAssistantForTest("codex", "", "", runner)

	_, err := assistant.Reply(context.Background(), ChatAssistantRequest{
		Message: "hello",
		Role:    "reviewer",
	})
	if err != nil {
		t.Fatalf("Reply returned error: %v", err)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("expected one runner call, got %d", len(runner.calls))
	}
	joined := strings.Join(runner.calls[0].args, " ")
	if !strings.Contains(joined, "[role_id=reviewer]") {
		t.Fatalf("expected role context in args, got %v", runner.calls[0].args)
	}
}

func TestCodexChatAssistantReplyUsesResumeForExistingSession(t *testing.T) {
	runner := &recordingClaudeRunner{
		stdout: `{"type":"item.completed","item":{"type":"agent_message","text":"continued"}}`,
	}
	assistant := newCodexChatAssistantForTest("codex", "", "", runner)

	got, err := assistant.Reply(context.Background(), ChatAssistantRequest{
		Message:        "next question",
		AgentSessionID: "thread-old",
	})
	if err != nil {
		t.Fatalf("Reply returned error: %v", err)
	}
	if got.AgentSessionID != "thread-old" {
		t.Fatalf("expected fallback thread id %q, got %q", "thread-old", got.AgentSessionID)
	}
	args := strings.Join(runner.calls[0].args, " ")
	if !strings.Contains(args, "-a never exec resume") {
		t.Fatalf("expected global -a never exec resume, got %v", runner.calls[0].args)
	}
	if !strings.Contains(args, " thread-old ") {
		t.Fatalf("expected session id in resume args, got %v", runner.calls[0].args)
	}
	if !strings.Contains(args, " next question") {
		t.Fatalf("expected resume call, got %v", runner.calls[0].args)
	}
}

func TestCodexChatAssistantReplyReturnsCommandError(t *testing.T) {
	runner := &recordingClaudeRunner{
		stderr: "network unavailable",
		err:    errors.New("exit status 1"),
	}
	assistant := newCodexChatAssistantForTest("codex", "", "", runner)

	_, err := assistant.Reply(context.Background(), ChatAssistantRequest{
		Message: "hello",
	})
	if err == nil {
		t.Fatal("expected error when codex runner fails")
	}
	if !strings.Contains(err.Error(), "network unavailable") {
		t.Fatalf("expected stderr detail in error, got %v", err)
	}
}
