package notifierdesktop

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/yoke233/ai-workflow/internal/core"
)

var _ core.Notifier = (*DesktopNotifier)(nil)

type commandRunner func(ctx context.Context, name string, args ...string) error

// DesktopNotifier sends desktop notifications via OS native command-line tools.
type DesktopNotifier struct {
	runner commandRunner
	goos   string
	ci     bool
}

func New() *DesktopNotifier {
	return newWithRunner(runCommand, runtime.GOOS, isCIEnvironment())
}

func newWithRunner(runner commandRunner, goos string, ci bool) *DesktopNotifier {
	if runner == nil {
		runner = runCommand
	}
	if strings.TrimSpace(goos) == "" {
		goos = runtime.GOOS
	}
	return &DesktopNotifier{
		runner: runner,
		goos:   goos,
		ci:     ci,
	}
}

func (n *DesktopNotifier) Name() string {
	return "desktop"
}

func (n *DesktopNotifier) Init(context.Context) error {
	return nil
}

func (n *DesktopNotifier) Close() error {
	return nil
}

func (n *DesktopNotifier) Notify(ctx context.Context, msg core.Notification) error {
	if n == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if n.ci {
		return nil
	}

	command, args, ok := n.buildCommand(msg)
	if !ok {
		return nil
	}
	return n.runner(ctx, command, args...)
}

func (n *DesktopNotifier) buildCommand(msg core.Notification) (string, []string, bool) {
	title, body := normalizeNotification(msg)

	switch n.goos {
	case "windows":
		script := fmt.Sprintf(
			"$wshell = New-Object -ComObject Wscript.Shell; $wshell.Popup('%s', 5, '%s', 64) | Out-Null",
			escapePowerShellSingleQuoted(body),
			escapePowerShellSingleQuoted(title),
		)
		return "powershell", []string{"-NoProfile", "-NonInteractive", "-Command", script}, true
	case "darwin":
		script := fmt.Sprintf(
			"display notification \"%s\" with title \"%s\"",
			escapeAppleScriptString(body),
			escapeAppleScriptString(title),
		)
		return "osascript", []string{"-e", script}, true
	default:
		return "", nil, false
	}
}

func normalizeNotification(msg core.Notification) (title string, body string) {
	title = strings.TrimSpace(msg.Title)
	if title == "" {
		title = "AI Workflow"
	}

	parts := make([]string, 0, 2)
	mainBody := strings.TrimSpace(msg.Body)
	if mainBody != "" {
		parts = append(parts, mainBody)
	}
	if action := strings.TrimSpace(msg.ActionURL); action != "" {
		parts = append(parts, action)
	}
	if len(parts) == 0 {
		parts = append(parts, "Notification")
	}
	body = strings.Join(parts, "\n")
	return title, body
}

func runCommand(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s failed: %s: %w", name, strings.TrimSpace(stderr.String()), err)
	}
	return nil
}

func isCIEnvironment() bool {
	value := strings.TrimSpace(os.Getenv("CI"))
	return strings.EqualFold(value, "1") ||
		strings.EqualFold(value, "true") ||
		strings.EqualFold(value, "yes")
}

func escapePowerShellSingleQuoted(input string) string {
	return strings.ReplaceAll(input, "'", "''")
}

func escapeAppleScriptString(input string) string {
	input = strings.ReplaceAll(input, "\\", "\\\\")
	return strings.ReplaceAll(input, "\"", "\\\"")
}
