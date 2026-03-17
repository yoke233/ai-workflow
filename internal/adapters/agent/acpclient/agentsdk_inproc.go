package acpclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"strings"

	agentsdkacp "github.com/cexll/agentsdk-go/pkg/acp"
	agentsdkapi "github.com/cexll/agentsdk-go/pkg/api"
	agentsdkmodel "github.com/cexll/agentsdk-go/pkg/model"
	acpproto "github.com/coder/acp-go-sdk"
	"github.com/yoke233/ai-workflow/internal/core"
	"github.com/yoke233/ai-workflow/internal/platform/profilellm"
)

const inProcAdapterAgentSDK = "agentsdk-go"

const (
	agentSDKDebugStreamEnv       = "AI_WORKFLOW_AGENTSDK_DEBUG_STREAM"
	agentSDKDebugStreamLegacyEnv = "AGENTSDK_DEBUG_STREAM"
)

var buildAgentSDKOptionsFromLaunch = defaultAgentSDKOptionsFromLaunch

func UsesInProcAdapterProfile(profile *core.AgentProfile) bool {
	if profile == nil {
		return false
	}
	return detectInternalAdapterKind(profile.DriverID, profile.Driver.LaunchCommand, profile.Driver.LaunchArgs) == inProcAdapterAgentSDK
}

func UsesInProcAdapterLaunch(cfg LaunchConfig) bool {
	return detectInternalAdapterKind("", cfg.Command, cfg.Args) == inProcAdapterAgentSDK
}

func newInProcAgentSDKClient(cfg LaunchConfig, h acpproto.Client, opts ...Option) (*Client, error) {
	adapterOptions, err := buildAgentSDKOptionsFromLaunch(cfg)
	if err != nil {
		return nil, err
	}

	agentPipe, clientPipe := net.Pipe()
	adapter := agentsdkacp.NewAdapter(adapterOptions)
	agentConn := acpproto.NewAgentSideConnection(adapter, agentPipe, agentPipe)
	adapter.SetConnection(agentConn)

	client, err := NewWithIO(cfg, h, clientPipe, clientPipe, opts...)
	if err != nil {
		_ = clientPipe.Close()
		_ = agentPipe.Close()
		return nil, err
	}

	previousCloseHook := client.closeHook
	client.closeHook = func(ctx context.Context) error {
		var outErr error
		if previousCloseHook != nil {
			if err := previousCloseHook(ctx); err != nil {
				outErr = err
			}
		}
		if err := agentPipe.Close(); err != nil && !errors.Is(err, net.ErrClosed) && outErr == nil {
			outErr = err
		}
		return outErr
	}
	return client, nil
}

func defaultAgentSDKOptionsFromLaunch(cfg LaunchConfig) (agentsdkapi.Options, error) {
	provider, err := resolveAgentSDKProviderConfig(cfg.Env)
	if err != nil {
		return agentsdkapi.Options{}, err
	}

	entryPoint := agentsdkapi.EntryPointPlatform
	projectRoot := strings.TrimSpace(cfg.WorkDir)
	if projectRoot == "" {
		projectRoot = "."
	}

	return agentsdkapi.Options{
		EntryPoint:  entryPoint,
		Mode:        agentsdkapi.ModeContext{EntryPoint: entryPoint},
		ProjectRoot: projectRoot,
		ModelFactory: agentsdkapi.ModelFactoryFunc(func(context.Context) (agentsdkmodel.Model, error) {
			model, err := buildAgentSDKModel(provider)
			if err != nil {
				return nil, err
			}
			if isAgentSDKStreamDebugEnabled(cfg.Env) {
				slog.Info("acpclient: agentsdk-go stream debug enabled",
					"provider", provider.ProviderType,
					"model", provider.Model,
					"base_url", provider.BaseURL)
				model = newAgentSDKDebugModel(model, provider)
			}
			return model, nil
		}),
	}, nil
}

type agentSDKProviderConfig struct {
	ProviderType string
	APIKey       string
	BaseURL      string
	Model        string
	Temperature  *float64
	MaxTokens    int
}

func resolveAgentSDKProviderConfig(env map[string]string) (agentSDKProviderConfig, error) {
	providerType := profilellm.NormalizeProviderType(firstNonEmpty(
		env["AGENTSDK_PROVIDER"],
		env["AI_WORKFLOW_LLM_PROVIDER"],
	))
	if providerType == "" {
		switch {
		case strings.TrimSpace(firstNonEmpty(env["ANTHROPIC_API_KEY"], env["ANTHROPIC_AUTH_TOKEN"])) != "":
			providerType = profilellm.ProviderAnthropic
		case strings.TrimSpace(env["OPENAI_API_KEY"]) != "":
			providerType = profilellm.ProviderOpenAIResponse
		}
	}
	if providerType == "" {
		return agentSDKProviderConfig{}, fmt.Errorf("acpclient: agentsdk-go in-proc launch requires AGENTSDK_PROVIDER")
	}

	cfg := agentSDKProviderConfig{
		ProviderType: providerType,
	}
	switch providerType {
	case profilellm.ProviderAnthropic:
		cfg.APIKey = firstNonEmpty(env["AGENTSDK_API_KEY"], env["ANTHROPIC_API_KEY"], env["ANTHROPIC_AUTH_TOKEN"])
		cfg.BaseURL = firstNonEmpty(env["AGENTSDK_BASE_URL"], env["ANTHROPIC_BASE_URL"])
		cfg.Model = firstNonEmpty(env["AGENTSDK_MODEL"], env["ANTHROPIC_MODEL"])
	case profilellm.ProviderOpenAIChatCompletion, profilellm.ProviderOpenAIResponse:
		cfg.APIKey = firstNonEmpty(env["AGENTSDK_API_KEY"], env["OPENAI_API_KEY"])
		cfg.BaseURL = firstNonEmpty(env["AGENTSDK_BASE_URL"], env["OPENAI_BASE_URL"])
		cfg.Model = firstNonEmpty(env["AGENTSDK_MODEL"], env["OPENAI_MODEL"])
	default:
		return agentSDKProviderConfig{}, fmt.Errorf("acpclient: unsupported agentsdk-go provider %q", providerType)
	}

	if temp, ok, err := parseOptionalFloat(firstNonEmpty(env["AGENTSDK_TEMPERATURE"], env["AI_WORKFLOW_LLM_TEMPERATURE"])); err != nil {
		return agentSDKProviderConfig{}, fmt.Errorf("acpclient: parse AGENTSDK_TEMPERATURE: %w", err)
	} else if ok {
		cfg.Temperature = &temp
	}

	if maxTokens, ok, err := parseOptionalInt(firstNonEmpty(env["AGENTSDK_MAX_OUTPUT_TOKENS"], env["AI_WORKFLOW_LLM_MAX_OUTPUT_TOKENS"])); err != nil {
		return agentSDKProviderConfig{}, fmt.Errorf("acpclient: parse AGENTSDK_MAX_OUTPUT_TOKENS: %w", err)
	} else if ok {
		cfg.MaxTokens = maxTokens
	}

	return cfg, nil
}

func buildAgentSDKModel(cfg agentSDKProviderConfig) (agentsdkmodel.Model, error) {
	switch cfg.ProviderType {
	case profilellm.ProviderAnthropic:
		return agentsdkmodel.NewAnthropic(agentsdkmodel.AnthropicConfig{
			APIKey:      cfg.APIKey,
			BaseURL:     cfg.BaseURL,
			Model:       cfg.Model,
			MaxTokens:   cfg.MaxTokens,
			Temperature: cfg.Temperature,
		})
	case profilellm.ProviderOpenAIChatCompletion:
		return agentsdkmodel.NewOpenAI(agentsdkmodel.OpenAIConfig{
			APIKey:      cfg.APIKey,
			BaseURL:     cfg.BaseURL,
			Model:       cfg.Model,
			MaxTokens:   cfg.MaxTokens,
			Temperature: cfg.Temperature,
		})
	case profilellm.ProviderOpenAIResponse:
		return agentsdkmodel.NewOpenAIResponses(agentsdkmodel.OpenAIConfig{
			APIKey:      cfg.APIKey,
			BaseURL:     cfg.BaseURL,
			Model:       cfg.Model,
			MaxTokens:   cfg.MaxTokens,
			Temperature: cfg.Temperature,
		})
	default:
		return nil, fmt.Errorf("acpclient: unsupported agentsdk-go provider %q", cfg.ProviderType)
	}
}

func detectInternalAdapterKind(driverID, launchCommand string, launchArgs []string) string {
	haystackParts := make([]string, 0, len(launchArgs)+2)
	if id := strings.ToLower(strings.TrimSpace(driverID)); id != "" {
		haystackParts = append(haystackParts, id)
	}
	if command := strings.ToLower(strings.TrimSpace(launchCommand)); command != "" {
		haystackParts = append(haystackParts, command)
	}
	for _, arg := range launchArgs {
		if trimmed := strings.ToLower(strings.TrimSpace(arg)); trimmed != "" {
			haystackParts = append(haystackParts, trimmed)
		}
	}
	haystack := strings.Join(haystackParts, " ")

	switch {
	case strings.Contains(haystack, "agentsdk-go"), strings.Contains(haystack, "agentsdk"):
		return inProcAdapterAgentSDK
	default:
		return ""
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func parseOptionalFloat(raw string) (float64, bool, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, false, nil
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, false, err
	}
	return value, true, nil
}

func parseOptionalInt(raw string) (int, bool, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, false, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, false, err
	}
	return value, true, nil
}

func isAgentSDKStreamDebugEnabled(env map[string]string) bool {
	return isTruthyString(firstNonEmpty(
		env[agentSDKDebugStreamEnv],
		env[agentSDKDebugStreamLegacyEnv],
	))
}

func isTruthyString(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

type agentSDKDebugModel struct {
	base     agentsdkmodel.Model
	provider agentSDKProviderConfig
	logger   *slog.Logger
}

func newAgentSDKDebugModel(base agentsdkmodel.Model, provider agentSDKProviderConfig) agentsdkmodel.Model {
	return &agentSDKDebugModel{
		base:     base,
		provider: provider,
		logger:   slog.Default(),
	}
}

func (m *agentSDKDebugModel) Complete(ctx context.Context, req agentsdkmodel.Request) (*agentsdkmodel.Response, error) {
	logger := m.loggerOrDefault()
	logger.Info("acpclient: agentsdk-go raw complete start",
		"session_id", req.SessionID,
		"messages", len(req.Messages),
		"tools", len(req.Tools))

	resp, err := m.base.Complete(ctx, req)
	if err != nil {
		logger.Error("acpclient: agentsdk-go raw complete error", "error", err)
		return nil, err
	}

	logger.Info("acpclient: agentsdk-go raw complete response",
		"response", marshalAgentSDKDebugJSON(resp))
	return resp, nil
}

func (m *agentSDKDebugModel) CompleteStream(ctx context.Context, req agentsdkmodel.Request, cb agentsdkmodel.StreamHandler) error {
	logger := m.loggerOrDefault()
	logger.Info("acpclient: agentsdk-go raw stream start",
		"session_id", req.SessionID,
		"messages", len(req.Messages),
		"tools", len(req.Tools))

	err := m.base.CompleteStream(ctx, req, func(sr agentsdkmodel.StreamResult) error {
		logger.Info("acpclient: agentsdk-go raw stream event",
			"event", marshalAgentSDKDebugJSON(sr))
		if cb == nil {
			return nil
		}
		return cb(sr)
	})
	if err != nil {
		logger.Error("acpclient: agentsdk-go raw stream error", "error", err)
		return err
	}

	logger.Info("acpclient: agentsdk-go raw stream end",
		"session_id", req.SessionID)
	return nil
}

func (m *agentSDKDebugModel) loggerOrDefault() *slog.Logger {
	attrs := []any{
		"provider", m.provider.ProviderType,
		"model", m.provider.Model,
		"base_url", m.provider.BaseURL,
	}
	if m.logger != nil {
		return m.logger.With(attrs...)
	}
	return slog.Default().With(attrs...)
}

func marshalAgentSDKDebugJSON(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("marshal_error:%v", err)
	}
	return string(data)
}
