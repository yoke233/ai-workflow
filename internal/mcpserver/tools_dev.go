//go:build dev

package mcpserver

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// gate is the shared preflight gate instance for dev tools.
var gate = NewPreflightGate()

func registerDevTools(server *mcp.Server, opts Options) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "self_build_frontend",
		Description: "Build the frontend SPA (npm run build) into web/dist/. Run this before self_build with embed_frontend=true, or independently to update static assets.",
	}, selfBuildFrontendHandler(opts))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "self_build",
		Description: "Build the ai-flow Go binary. Set embed_frontend=true to bake web/dist/ into the binary (requires self_build_frontend first). Always includes -tags dev.",
	}, selfBuildHandler(opts))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "self_preflight",
		Description: "Run full quality gate (go vet, go build, go test, frontend typecheck+build) before restart. Must pass before self_restart is allowed.",
	}, selfPreflightHandler(opts))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "self_restart",
		Description: "Restart the ai-flow server. REQUIRES a successful self_preflight. Use force=true only for emergencies.",
	}, selfRestartHandler(opts))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "self_preflight_status",
		Description: "Check the current preflight gate status without running checks.",
	}, selfPreflightStatusHandler(opts))
}

// --- self_build_frontend ---

type SelfBuildFrontendInput struct{}

type SelfBuildFrontendOutput struct {
	Success bool   `json:"success"`
	Output  string `json:"output"`
}

func selfBuildFrontendHandler(opts Options) func(context.Context, *mcp.CallToolRequest, SelfBuildFrontendInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, in SelfBuildFrontendInput) (*mcp.CallToolResult, any, error) {
		if opts.SourceRoot == "" {
			return errorResult("source_root not configured")
		}

		cmd := exec.CommandContext(ctx, "npm", "--prefix", "web", "run", "build")
		cmd.Dir = opts.SourceRoot
		var buf bytes.Buffer
		cmd.Stdout = &buf
		cmd.Stderr = &buf

		if err := cmd.Run(); err != nil {
			return jsonResult(SelfBuildFrontendOutput{
				Success: false,
				Output:  buf.String(),
			})
		}

		return jsonResult(SelfBuildFrontendOutput{
			Success: true,
			Output:  buf.String(),
		})
	}
}

// --- self_build ---

type SelfBuildInput struct {
	EmbedFrontend bool `json:"embed_frontend,omitempty" jsonschema:"Embed web/dist/ into binary (adds -tags webdist). Run self_build_frontend first."`
}

type SelfBuildOutput struct {
	Success    bool   `json:"success"`
	Output     string `json:"output"`
	Tags       string `json:"tags"`
	BinarySize int64  `json:"binary_size"`
}

func selfBuildHandler(opts Options) func(context.Context, *mcp.CallToolRequest, SelfBuildInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, in SelfBuildInput) (*mcp.CallToolResult, any, error) {
		if opts.SourceRoot == "" {
			return errorResult("source_root not configured")
		}
		binaryPath, err := os.Executable()
		if err != nil {
			return nil, nil, fmt.Errorf("resolve executable: %w", err)
		}

		tags := "dev"
		if in.EmbedFrontend {
			tags = "dev,webdist"
		}

		args := []string{"build", "-tags", tags, "-o", binaryPath, "./cmd/ai-flow"}

		cmd := exec.CommandContext(ctx, "go", args...)
		cmd.Dir = opts.SourceRoot
		var buf bytes.Buffer
		cmd.Stdout = &buf
		cmd.Stderr = &buf

		buildErr := cmd.Run()
		output := buf.String()

		if buildErr != nil {
			return jsonResult(SelfBuildOutput{
				Success: false,
				Output:  output,
				Tags:    tags,
			})
		}

		var size int64
		if info, statErr := os.Stat(binaryPath); statErr == nil {
			size = info.Size()
		}

		return jsonResult(SelfBuildOutput{
			Success:    true,
			Output:     output,
			Tags:       tags,
			BinarySize: size,
		})
	}
}

// --- self_preflight ---

type SelfPreflightInput struct {
	SkipFrontend bool `json:"skip_frontend,omitempty" jsonschema:"Skip frontend typecheck and build steps"`
}

type SelfPreflightOutput struct {
	Success   bool         `json:"success"`
	CommitSHA string       `json:"commit_sha"`
	Duration  string       `json:"duration"`
	Steps     []StepResult `json:"steps"`
	Message   string       `json:"message"`
}

func selfPreflightHandler(opts Options) func(context.Context, *mcp.CallToolRequest, SelfPreflightInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, in SelfPreflightInput) (*mcp.CallToolResult, any, error) {
		if opts.SourceRoot == "" {
			return errorResult("source_root not configured")
		}

		// Broadcast preflight start.
		postSystemEvent(opts.ServerAddr, "preflight_start", map[string]any{
			"message": "Preflight quality gate started",
		})

		progress := func(step StepResult, idx, total int) {
			status := "PASS"
			if !step.Success {
				status = "FAIL"
			}
			postSystemEvent(opts.ServerAddr, "preflight_step", map[string]any{
				"step":     idx + 1,
				"total":    total,
				"name":     step.Name,
				"status":   status,
				"duration": step.Duration,
				"message":  fmt.Sprintf("[%d/%d] %s: %s (%s)", idx+1, total, step.Name, status, step.Duration),
			})
		}

		result, err := gate.Run(ctx, opts.SourceRoot, in.SkipFrontend, progress)
		if err != nil {
			return errorResult(fmt.Sprintf("preflight error: %v", err))
		}

		msg := "PASS — self_restart is now allowed"
		event := "preflight_pass"
		if !result.Success {
			msg = "FAIL — self_restart is blocked until all checks pass"
			event = "preflight_fail"
		}

		postSystemEvent(opts.ServerAddr, event, map[string]any{
			"success":    result.Success,
			"commit_sha": result.CommitSHA,
			"duration":   result.Duration.String(),
			"message":    msg,
		})

		return jsonResult(SelfPreflightOutput{
			Success:   result.Success,
			CommitSHA: result.CommitSHA,
			Duration:  result.Duration.String(),
			Steps:     result.Steps,
			Message:   msg,
		})
	}
}

// --- self_preflight_status ---

type SelfPreflightStatusInput struct{}

func selfPreflightStatusHandler(opts Options) func(context.Context, *mcp.CallToolRequest, SelfPreflightStatusInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, in SelfPreflightStatusInput) (*mcp.CallToolResult, any, error) {
		last := gate.LastResult()
		if last == nil {
			return jsonResult(map[string]any{
				"status":  "no_preflight",
				"message": "no preflight has been run yet",
			})
		}

		// Check against current HEAD.
		currentSHA := ""
		if opts.SourceRoot != "" {
			sha, err := gitHeadSHA(ctx, opts.SourceRoot)
			if err == nil {
				currentSHA = sha
			}
		}

		canRestart, reason := gate.CanRestart(currentSHA)
		return jsonResult(map[string]any{
			"last_success":   last.Success,
			"last_commit":    last.CommitSHA,
			"last_timestamp": last.Timestamp.Format(time.RFC3339),
			"current_commit": currentSHA,
			"can_restart":    canRestart,
			"reason":         reason,
		})
	}
}

// --- self_restart (gated) ---

type SelfRestartInput struct {
	GracefulTimeoutSec int  `json:"graceful_timeout_sec,omitempty" jsonschema:"Graceful shutdown timeout in seconds"`
	Force              bool `json:"force,omitempty" jsonschema:"Bypass preflight gate (emergency only)"`
}

func selfRestartHandler(opts Options) func(context.Context, *mcp.CallToolRequest, SelfRestartInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, in SelfRestartInput) (*mcp.CallToolResult, any, error) {
		if opts.ServerAddr == "" {
			return errorResult("server_addr not configured")
		}

		// Preflight gate check.
		if !in.Force {
			currentSHA := ""
			if opts.SourceRoot != "" {
				sha, err := gitHeadSHA(ctx, opts.SourceRoot)
				if err == nil {
					currentSHA = sha
				}
			}
			ok, reason := gate.CanRestart(currentSHA)
			if !ok {
				return errorResult(fmt.Sprintf("restart BLOCKED: %s", reason))
			}
		}

		timeout := 10 * time.Second
		if in.GracefulTimeoutSec > 0 {
			timeout = time.Duration(in.GracefulTimeoutSec) * time.Second
		}

		url := opts.ServerAddr + "/api/v1/admin/ops/restart"
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
		if err != nil {
			return nil, nil, fmt.Errorf("create restart request: %w", err)
		}

		client := &http.Client{Timeout: timeout}
		resp, err := client.Do(httpReq)
		if err != nil {
			return jsonResult(map[string]any{
				"status":  "request_sent",
				"message": "restart signal sent, server may be restarting",
				"error":   err.Error(),
			})
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
			return errorResult(fmt.Sprintf("restart endpoint returned %d", resp.StatusCode))
		}

		return jsonResult(map[string]any{
			"status":  "restarting",
			"message": "server restart initiated (preflight passed)",
		})
	}
}
