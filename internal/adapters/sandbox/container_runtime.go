package sandbox

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/yoke233/ai-workflow/internal/adapters/agent/acpclient"
)

const (
	containerWorkDir = "/workspace"
	containerHomeDir = "/sandbox/home"
	containerTempDir = "/sandbox/tmp"
)

type containerVolume struct {
	hostPath      string
	containerPath string
}

type containerLaunchSpec struct {
	command        string
	image          string
	runArgs        []string
	cpus           string
	memory         string
	memorySwap     string
	network        string
	pidsLimit      string
	readOnlyRootFS bool
	tmpfs          []string
}

func prepareContainerLaunch(ctx context.Context, base Sandbox, in PrepareInput, spec containerLaunchSpec, runner containerArgsBuilder) (acpclient.LaunchConfig, error) {
	if runner == nil {
		return acpclient.LaunchConfig{}, fmt.Errorf("container sandbox: args builder is required")
	}
	if base == nil {
		base = NoopSandbox{}
	}

	launch, err := base.Prepare(ctx, in)
	if err != nil {
		return launch, err
	}
	program := strings.TrimSpace(launch.Command)
	if program == "" {
		return launch, fmt.Errorf("container sandbox: target program is required")
	}
	command := strings.TrimSpace(spec.command)
	if command == "" {
		return launch, fmt.Errorf("container sandbox: runner command is required")
	}
	image := strings.TrimSpace(spec.image)
	if image == "" {
		return launch, fmt.Errorf("container sandbox: image is required")
	}

	rewritten, mounts, err := rewriteLaunchForContainer(launch)
	if err != nil {
		return launch, err
	}
	if err := materializeExternalLinks(materializeMountRoots(mounts)); err != nil {
		return launch, err
	}

	launch.Command = command
	launch.Args = runner(spec, rewritten, mounts)
	launch.WorkDir = ""
	launch.Env = nil
	return launch, nil
}

type containerArgsBuilder func(spec containerLaunchSpec, launch acpclient.LaunchConfig, mounts []containerVolume) []string

func rewriteLaunchForContainer(launch acpclient.LaunchConfig) (acpclient.LaunchConfig, []containerVolume, error) {
	out := launch
	out.Env = cloneLaunchEnv(launch.Env)

	mounts := make([]containerVolume, 0, 3)
	seen := map[string]string{}

	addMount := func(hostPath string, preferred string) string {
		hostPath = strings.TrimSpace(hostPath)
		if hostPath == "" {
			return ""
		}
		if existing, ok := seen[hostPath]; ok {
			return existing
		}
		seen[hostPath] = preferred
		mounts = append(mounts, containerVolume{hostPath: hostPath, containerPath: preferred})
		return preferred
	}

	if workDir := strings.TrimSpace(launch.WorkDir); workDir != "" {
		out.WorkDir = addMount(workDir, containerWorkDir)
	}

	if homeKey, homeDir := detectLaunchHome(out.Env); homeDir != "" {
		out.Env[homeKey] = addMount(homeDir, containerHomeDir)
	}
	if tempDir := detectLaunchTemp(out.Env); tempDir != "" {
		containerPath := addMount(tempDir, containerTempDir)
		for _, key := range []string{"TMPDIR", "TMP", "TEMP"} {
			if _, ok := out.Env[key]; ok {
				out.Env[key] = containerPath
			}
		}
	}

	return out, mounts, nil
}

func detectLaunchHome(env map[string]string) (string, string) {
	for _, key := range []string{"CODEX_HOME", "CLAUDE_CONFIG_DIR", "HOME"} {
		if value := strings.TrimSpace(env[key]); value != "" {
			return key, value
		}
	}
	return "", ""
}

func detectLaunchTemp(env map[string]string) string {
	for _, key := range []string{"TMPDIR", "TMP", "TEMP"} {
		if value := strings.TrimSpace(env[key]); value != "" {
			return value
		}
	}
	return ""
}

func cloneLaunchEnv(in map[string]string) map[string]string {
	if len(in) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func materializeMountRoots(mounts []containerVolume) []string {
	roots := make([]string, 0, 2)
	seen := map[string]struct{}{}
	for _, mount := range mounts {
		if mount.containerPath != containerHomeDir && mount.containerPath != containerTempDir {
			continue
		}
		hostPath := strings.TrimSpace(mount.hostPath)
		if hostPath == "" {
			continue
		}
		if _, ok := seen[hostPath]; ok {
			continue
		}
		seen[hostPath] = struct{}{}
		roots = append(roots, hostPath)
	}
	return roots
}

func materializeExternalLinks(roots []string) error {
	if len(roots) == 0 {
		return nil
	}
	cleanRoots := make([]string, 0, len(roots))
	for _, root := range roots {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		cleanRoots = append(cleanRoots, filepath.Clean(root))
	}
	for _, root := range cleanRoots {
		if err := materializePath(root, cleanRoots); err != nil {
			return err
		}
	}
	return nil
}

func materializePath(path string, roots []string) error {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("materialize sandbox path %s: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := filepath.EvalSymlinks(path)
		if err != nil {
			return fmt.Errorf("resolve sandbox link %s: %w", path, err)
		}
		if isWithinAnyRoot(target, roots) {
			return nil
		}
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("remove sandbox link %s: %w", path, err)
		}
		if err := copyPath(target, path); err != nil {
			return fmt.Errorf("materialize sandbox link %s from %s: %w", path, target, err)
		}
		return materializePath(path, roots)
	}
	if !info.IsDir() {
		return nil
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return fmt.Errorf("read sandbox dir %s: %w", path, err)
	}
	for _, entry := range entries {
		if err := materializePath(filepath.Join(path, entry.Name()), roots); err != nil {
			return err
		}
	}
	return nil
}

func isWithinAnyRoot(path string, roots []string) bool {
	for _, root := range roots {
		if isWithinRoot(path, root) {
			return true
		}
	}
	return false
}

func isWithinRoot(path string, root string) bool {
	path = filepath.Clean(path)
	root = filepath.Clean(root)
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

func copyPath(src string, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return copyDir(src, dst, info.Mode())
	}
	return copyFileWithMode(src, dst, info.Mode())
}

func copyDir(src string, dst string, mode fs.FileMode) error {
	if err := os.MkdirAll(dst, mode.Perm()); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		childSrc := filepath.Join(src, entry.Name())
		childDst := filepath.Join(dst, entry.Name())
		if entry.Type()&os.ModeSymlink != 0 {
			if err := materializeLinkedChild(childSrc, childDst); err != nil {
				return err
			}
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.IsDir() {
			if err := copyDir(childSrc, childDst, info.Mode()); err != nil {
				return err
			}
			continue
		}
		if err := copyFileWithMode(childSrc, childDst, info.Mode()); err != nil {
			return err
		}
	}
	return nil
}

func materializeLinkedChild(src string, dst string) error {
	target, err := filepath.EvalSymlinks(src)
	if err != nil {
		return err
	}
	return copyPath(target, dst)
}

func copyFileWithMode(src string, dst string, mode fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode.Perm())
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

func appendSortedEnvArgs(args []string, env map[string]string, flag string) []string {
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		args = append(args, flag, key+"="+env[key])
	}
	return args
}
