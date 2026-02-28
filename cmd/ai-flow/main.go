package main

import (
	"fmt"
	"os"

	agentclaude "github.com/user/ai-workflow/internal/plugins/agent-claude"
	runtimeprocess "github.com/user/ai-workflow/internal/plugins/runtime-process"
	"github.com/user/ai-workflow/internal/tui"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	args := os.Args[1:]
	if len(args) == 0 {
		printUsage()
		return nil
	}

	switch args[0] {
	case "version":
		fmt.Println("ai-flow v0.1.0-dev")
	case "project":
		if len(args) < 2 {
			return fmt.Errorf("usage: ai-flow project <add|list>")
		}
		switch args[1] {
		case "add":
			return cmdProjectAdd(args[2:])
		case "list", "ls":
			return cmdProjectList()
		default:
			return fmt.Errorf("unknown project command: %s", args[1])
		}
	case "pipeline":
		if len(args) < 2 {
			return fmt.Errorf("usage: ai-flow pipeline <create|start|status>")
		}
		switch args[1] {
		case "create":
			return cmdPipelineCreate(args[2:])
		case "start":
			return cmdPipelineStart(args[2:])
		case "status":
			return cmdPipelineStatus(args[2:])
		default:
			return fmt.Errorf("unknown pipeline command: %s", args[1])
		}
	case "tui":
		exec, store, err := bootstrap()
		if err != nil {
			return err
		}
		defer store.Close()
		claude := agentclaude.New("claude")
		runtime := runtimeprocess.New()
		return tui.Run(exec, store, claude, runtime)
	default:
		return fmt.Errorf("unknown command: %s", args[0])
	}
	return nil
}

func printUsage() {
	fmt.Println(`ai-flow - AI Workflow Orchestrator

Usage:
  ai-flow version
  ai-flow project add <id> <repo-path>
  ai-flow project list
  ai-flow pipeline create <project-id> <name> <description> [template]
  ai-flow pipeline start <pipeline-id>
  ai-flow pipeline status <pipeline-id>
  ai-flow tui`)
}
