package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"time"

	flowapp "github.com/yoke233/ai-workflow/internal/application/flow"
	"github.com/yoke233/ai-workflow/internal/core"
	"github.com/yoke233/ai-workflow/internal/adapters/workspace/clone"
)

type bootstrapPRFlowRequest struct {
	BaseBranch *string `json:"base_branch,omitempty"`
	Title      *string `json:"title,omitempty"`
	Body       *string `json:"body,omitempty"`
}

type scmBindingInfo struct {
	Provider      string
	RepoPath      string
	DefaultBranch string
	RemoteHost    string
	RemoteOwner   string
	RemoteRepo    string
}

type bootstrapPRFlowResponse struct {
	FlowID       int64 `json:"flow_id"`
	ImplementID  int64 `json:"implement_step_id"`
	CommitPushID int64 `json:"commit_push_step_id"`
	OpenPRID     int64 `json:"open_pr_step_id"`
	GateID       int64 `json:"gate_step_id"`
}

// bootstrapPRFlow creates a standard PR automation flow:
// implement(exec) → commit_push(exec,builtin) → open_pr(exec,builtin) → review_merge_gate(gate).
//
// Requirements:
// - Flow must belong to a project
// - Project must have a supported SCM git resource binding (GitHub / Codeup)
func (h *Handler) bootstrapPRFlow(w http.ResponseWriter, r *http.Request) {
	flowID, ok := urlParamInt64(r, "flowID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid flow ID", "BAD_ID")
		return
	}

	flow, err := h.store.GetFlow(r.Context(), flowID)
	if err == core.ErrNotFound {
		writeError(w, http.StatusNotFound, "flow not found", "NOT_FOUND")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "STORE_ERROR")
		return
	}
	if flow.ProjectID == nil {
		writeError(w, http.StatusBadRequest, "flow must belong to a project", "MISSING_PROJECT")
		return
	}

	projectID := *flow.ProjectID
	bindings, err := h.store.ListResourceBindings(r.Context(), projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "STORE_ERROR")
		return
	}
	bindingInfo, ok := resolveSCMRepoFromBindings(r.Context(), bindings)
	if !ok {
		writeError(w, http.StatusBadRequest, "project does not have a supported SCM git binding", "MISSING_SCM_BINDING")
		return
	}
	_ = bindingInfo.RepoPath // used by builtin steps via workspace provider; keep here for validation side-effects.

	var req bootstrapPRFlowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != context.Canceled {
		// Allow empty body.
		if strings.TrimSpace(err.Error()) != "EOF" {
			writeError(w, http.StatusBadRequest, "invalid JSON body", "BAD_REQUEST")
			return
		}
	}

	baseBranch := bindingInfo.DefaultBranch
	if req.BaseBranch != nil && strings.TrimSpace(*req.BaseBranch) != "" {
		baseBranch = strings.TrimSpace(*req.BaseBranch)
	}

	title := fmt.Sprintf("ai-flow: flow %d", flowID)
	body := fmt.Sprintf("Automated change request for %s/%s.", bindingInfo.RemoteOwner, bindingInfo.RemoteRepo)
	if req.Title != nil && strings.TrimSpace(*req.Title) != "" {
		title = strings.TrimSpace(*req.Title)
	}
	if req.Body != nil && strings.TrimSpace(*req.Body) != "" {
		body = strings.TrimSpace(*req.Body)
	}

	steps, err := h.store.ListStepsByFlow(r.Context(), flowID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "STORE_ERROR")
		return
	}
	if len(steps) > 0 {
		writeError(w, http.StatusConflict, "flow already has steps", "FLOW_HAS_STEPS")
		return
	}

	providerPrompts := h.currentPRFlowPrompts().Provider(bindingInfo.Provider)
	implementObjective := providerPrompts.ImplementObjective
	gateObjective := providerPrompts.GateObjective
	commitMessage := defaultPRCommitMessage(flowID)

	implement := &core.Step{
		FlowID:     flowID,
		Name:       "implement",
		Type:       core.StepExec,
		Status:     core.StepPending,
		AgentRole:  "worker",
		Timeout:    15 * time.Minute,
		MaxRetries: 3,
		Config: map[string]any{
			"objective": implementObjective,
		},
	}
	implementID, err := h.store.CreateStep(r.Context(), implement)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "STORE_ERROR")
		return
	}

	commitPush := &core.Step{
		FlowID:     flowID,
		Name:       "commit_push",
		Type:       core.StepExec,
		Status:     core.StepPending,
		DependsOn:  []int64{implementID},
		AgentRole:  "worker",
		Timeout:    5 * time.Minute,
		MaxRetries: 0,
		Config: map[string]any{
			"builtin":        "git_commit_push",
			"commit_message": commitMessage,
		},
	}
	commitPushID, err := h.store.CreateStep(r.Context(), commitPush)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "STORE_ERROR")
		return
	}

	openPR := &core.Step{
		FlowID:     flowID,
		Name:       "open_pr",
		Type:       core.StepExec,
		Status:     core.StepPending,
		DependsOn:  []int64{commitPushID},
		AgentRole:  "worker",
		Timeout:    5 * time.Minute,
		MaxRetries: 0,
		Config: map[string]any{
			"builtin": "scm_open_pr",
			"base":    baseBranch,
			"title":   title,
			"body":    body,
		},
	}
	openPRID, err := h.store.CreateStep(r.Context(), openPR)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "STORE_ERROR")
		return
	}

	gate := &core.Step{
		FlowID:     flowID,
		Name:       "review_merge_gate",
		Type:       core.StepGate,
		Status:     core.StepPending,
		DependsOn:  []int64{openPRID},
		AgentRole:  "gate",
		Timeout:    10 * time.Minute,
		MaxRetries: 0,
		RequiredCapabilities: []string{
			"pr.review",
		},
		Config: map[string]any{
			"merge_on_pass":          true,
			"merge_method":           mergeMethodFromBindings(bindings),
			"reset_upstream_closure": true,
			"objective":              gateObjective,
		},
	}
	gateID, err := h.store.CreateStep(r.Context(), gate)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "STORE_ERROR")
		return
	}

	writeJSON(w, http.StatusCreated, bootstrapPRFlowResponse{
		FlowID:       flowID,
		ImplementID:  implementID,
		CommitPushID: commitPushID,
		OpenPRID:     openPRID,
		GateID:       gateID,
	})
}

func (h *Handler) currentPRFlowPrompts() flowapp.PRFlowPrompts {
	if h != nil && h.prPrompts != nil {
		return flowapp.MergePRFlowPrompts(h.prPrompts())
	}
	return flowapp.DefaultPRFlowPrompts()
}

func defaultPRCommitMessage(flowID int64) string {
	return fmt.Sprintf("chore(pr-flow): apply flow %d updates", flowID)
}

func resolveSCMRepoFromBindings(ctx context.Context, bindings []*core.ResourceBinding) (scmBindingInfo, bool) {
	for _, b := range bindings {
		if b == nil || strings.TrimSpace(b.Kind) != "git" {
			continue
		}
		repoPath := strings.TrimSpace(b.URI)
		if repoPath == "" {
			continue
		}
		defaultBranch := bindingDefaultBranch(b)
		originURL, err := gitOriginURL(ctx, repoPath)
		if err != nil {
			continue
		}
		remote, err := workspaceclone.ParseRemoteURL(originURL)
		if err != nil {
			continue
		}
		provider := bindingProvider(b, remote.Host)
		if provider == "" {
			continue
		}
		return scmBindingInfo{
			Provider:      provider,
			RepoPath:      repoPath,
			DefaultBranch: defaultBranch,
			RemoteHost:    strings.TrimSpace(remote.Host),
			RemoteOwner:   strings.TrimSpace(remote.Owner),
			RemoteRepo:    strings.TrimSpace(remote.Repo),
		}, true
	}
	return scmBindingInfo{}, false
}

func bindingProvider(b *core.ResourceBinding, host string) string {
	if b != nil && b.Config != nil {
		if v, ok := b.Config["provider"].(string); ok && strings.TrimSpace(v) != "" {
			return strings.ToLower(strings.TrimSpace(v))
		}
	}
	host = strings.ToLower(strings.TrimSpace(host))
	switch {
	case host == "github.com":
		return "github"
	case strings.Contains(host, "rdc.aliyuncs.com"), strings.Contains(host, "codeup.aliyun.com"):
		return "codeup"
	default:
		return ""
	}
}

func bindingDefaultBranch(b *core.ResourceBinding) string {
	if b != nil && b.Config != nil {
		for _, key := range []string{"base_branch", "default_branch"} {
			if v, ok := b.Config[key].(string); ok && strings.TrimSpace(v) != "" {
				return strings.TrimSpace(v)
			}
		}
	}
	return "main"
}

func mergeMethodFromBindings(bindings []*core.ResourceBinding) string {
	for _, b := range bindings {
		if b == nil || strings.TrimSpace(b.Kind) != "git" || b.Config == nil {
			continue
		}
		if v, ok := b.Config["merge_method"].(string); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return "squash"
}

func gitOriginURL(ctx context.Context, repoPath string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	cmd := exec.CommandContext(ctx, "git", "-C", repoPath, "remote", "get-url", "origin")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git origin url: %s", strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(string(out)), nil
}
