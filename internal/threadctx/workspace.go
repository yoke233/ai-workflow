package threadctx

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/yoke233/ai-workflow/internal/core"
)

type Store interface {
	GetThread(ctx context.Context, id int64) (*core.Thread, error)
	GetProject(ctx context.Context, id int64) (*core.Project, error)
	ListThreadMembers(ctx context.Context, threadID int64) ([]*core.ThreadMember, error)
	ListThreadContextRefs(ctx context.Context, threadID int64) ([]*core.ThreadContextRef, error)
	ListResourceBindings(ctx context.Context, projectID int64) ([]*core.ResourceBinding, error)
}

type PathsInfo struct {
	ThreadDir    string
	WorkspaceDir string
	MountsDir    string
	ArchiveDir   string
	ContextFile  string
}

type ResolvedMount struct {
	Slug          string
	Project       *core.Project
	TargetPath    string
	Access        core.ContextAccess
	CheckCommands []string
}

var slugSanitizer = regexp.MustCompile(`[^a-z0-9-]`)

func Paths(dataDir string, threadID int64) PathsInfo {
	threadDir := filepath.Join(strings.TrimSpace(dataDir), "threads", strconv.FormatInt(threadID, 10))
	workspaceDir := filepath.Join(threadDir, "workspace")
	return PathsInfo{
		ThreadDir:    threadDir,
		WorkspaceDir: workspaceDir,
		MountsDir:    filepath.Join(threadDir, "mounts"),
		ArchiveDir:   filepath.Join(threadDir, "archive"),
		ContextFile:  filepath.Join(workspaceDir, ".context.json"),
	}
}

func EnsureLayout(dataDir string, threadID int64) (PathsInfo, error) {
	if strings.TrimSpace(dataDir) == "" {
		return PathsInfo{}, nil
	}
	paths := Paths(dataDir, threadID)
	for _, dir := range []string{paths.ThreadDir, paths.WorkspaceDir, paths.MountsDir, paths.ArchiveDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return PathsInfo{}, fmt.Errorf("create thread workspace dir %q: %w", dir, err)
		}
	}
	return paths, nil
}

func LoadContextFile(dataDir string, threadID int64) (*core.ThreadWorkspaceContext, error) {
	paths := Paths(dataDir, threadID)
	raw, err := os.ReadFile(paths.ContextFile)
	if err != nil {
		return nil, err
	}
	var payload core.ThreadWorkspaceContext
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("decode thread context: %w", err)
	}
	return &payload, nil
}

func SyncContextFile(ctx context.Context, store Store, dataDir string, threadID int64) (*core.ThreadWorkspaceContext, error) {
	if strings.TrimSpace(dataDir) == "" {
		return nil, nil
	}
	if store == nil {
		return nil, fmt.Errorf("thread context store is nil")
	}

	paths, err := EnsureLayout(dataDir, threadID)
	if err != nil {
		return nil, err
	}

	payload, err := BuildWorkspaceContext(ctx, store, dataDir, threadID)
	if err != nil {
		return nil, err
	}

	b, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode thread context: %w", err)
	}
	if err := os.WriteFile(paths.ContextFile, append(b, '\n'), 0o644); err != nil {
		return nil, fmt.Errorf("write thread context file: %w", err)
	}
	return payload, nil
}

func BuildWorkspaceContext(ctx context.Context, store Store, dataDir string, threadID int64) (*core.ThreadWorkspaceContext, error) {
	if store == nil {
		return nil, fmt.Errorf("thread context store is nil")
	}

	if _, err := store.GetThread(ctx, threadID); err != nil {
		return nil, err
	}

	refs, err := store.ListThreadContextRefs(ctx, threadID)
	if err != nil {
		return nil, err
	}
	members, err := store.ListThreadMembers(ctx, threadID)
	if err != nil {
		return nil, err
	}

	payload := &core.ThreadWorkspaceContext{
		ThreadID:  threadID,
		Workspace: ".",
		Mounts:    map[string]core.ThreadWorkspaceMount{},
		Archive:   "../archive",
		UpdatedAt: nowUTC(),
	}

	memberSet := make(map[string]struct{})
	for _, member := range members {
		if member == nil {
			continue
		}
		id := strings.TrimSpace(member.UserID)
		if id == "" {
			continue
		}
		memberSet[id] = struct{}{}
	}
	payload.Members = make([]string, 0, len(memberSet))
	for id := range memberSet {
		payload.Members = append(payload.Members, id)
	}
	sort.Strings(payload.Members)

	for _, ref := range refs {
		mount, err := ResolveMount(ctx, store, ref)
		if err != nil || mount == nil {
			continue
		}
		payload.Mounts[mount.Slug] = core.ThreadWorkspaceMount{
			Path:          filepath.ToSlash(filepath.Join("..", "mounts", mount.Slug)),
			ProjectID:     mount.Project.ID,
			Access:        mount.Access,
			CheckCommands: append([]string(nil), mount.CheckCommands...),
		}
	}

	if len(payload.Mounts) == 0 {
		payload.Mounts = nil
	}
	return payload, nil
}

func ResolveMount(ctx context.Context, store Store, ref *core.ThreadContextRef) (*ResolvedMount, error) {
	if store == nil {
		return nil, fmt.Errorf("thread context store is nil")
	}
	if ref == nil {
		return nil, fmt.Errorf("thread context ref is nil")
	}
	project, err := store.GetProject(ctx, ref.ProjectID)
	if err != nil {
		return nil, err
	}
	bindings, err := store.ListResourceBindings(ctx, ref.ProjectID)
	if err != nil {
		return nil, err
	}

	targetPath, checkCommands := resolveBindingTarget(bindings)
	if targetPath == "" {
		return nil, fmt.Errorf("project %d has no resolvable workspace binding", ref.ProjectID)
	}
	return &ResolvedMount{
		Slug:          projectSlug(project),
		Project:       project,
		TargetPath:    targetPath,
		Access:        ref.Access,
		CheckCommands: checkCommands,
	}, nil
}

func resolveBindingTarget(bindings []*core.ResourceBinding) (string, []string) {
	for _, binding := range bindings {
		if binding == nil {
			continue
		}
		switch binding.Kind {
		case core.ResourceKindLocalFS:
			if path := strings.TrimSpace(binding.URI); path != "" {
				return path, readCheckCommands(binding.Config)
			}
		case core.ResourceKindGit:
			if path := resolveGitBindingPath(binding); path != "" {
				return path, readCheckCommands(binding.Config)
			}
		}
	}
	return "", nil
}

func resolveGitBindingPath(binding *core.ResourceBinding) string {
	if binding == nil {
		return ""
	}
	if uri := strings.TrimSpace(binding.URI); uri != "" && !looksLikeRemoteGitURI(uri) {
		return uri
	}
	if binding.Config == nil {
		return ""
	}
	if cloneDir, ok := binding.Config["clone_dir"].(string); ok {
		return strings.TrimSpace(cloneDir)
	}
	return ""
}

func looksLikeRemoteGitURI(uri string) bool {
	if strings.Contains(uri, "://") {
		return true
	}
	return strings.HasPrefix(uri, "git@") && strings.Contains(uri, ":")
}

func readCheckCommands(cfg map[string]any) []string {
	if len(cfg) == 0 {
		return nil
	}
	raw, ok := cfg["check_commands"]
	if !ok {
		return nil
	}
	switch value := raw.(type) {
	case []string:
		out := make([]string, 0, len(value))
		for _, item := range value {
			item = strings.TrimSpace(item)
			if item != "" {
				out = append(out, item)
			}
		}
		return out
	case []any:
		out := make([]string, 0, len(value))
		for _, item := range value {
			if s, ok := item.(string); ok {
				s = strings.TrimSpace(s)
				if s != "" {
					out = append(out, s)
				}
			}
		}
		return out
	default:
		return nil
	}
}

func projectSlug(project *core.Project) string {
	if project == nil {
		return "project"
	}
	base := strings.ToLower(strings.TrimSpace(project.Name))
	if base == "" {
		base = "project-" + strconv.FormatInt(project.ID, 10)
	}
	base = strings.ReplaceAll(base, "_", "-")
	base = strings.ReplaceAll(base, " ", "-")
	base = slugSanitizer.ReplaceAllString(base, "-")
	for strings.Contains(base, "--") {
		base = strings.ReplaceAll(base, "--", "-")
	}
	base = strings.Trim(base, "-")
	if base == "" {
		base = "project-" + strconv.FormatInt(project.ID, 10)
	}
	return base
}

func nowUTC() time.Time {
	return time.Now().UTC()
}
