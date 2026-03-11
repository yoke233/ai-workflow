package appcmd

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/yoke233/ai-workflow/internal/platform/appdata"
	"github.com/yoke233/ai-workflow/internal/platform/config"
)

const (
	DefaultServerPort  = 8080
	defaultFrontendDir = "/opt/ai-workflow/web/dist"
	repoFrontendDir    = "web/dist"
	frontendDirEnvVar  = "AI_WORKFLOW_FRONTEND_DIR"
)

func LoadConfig() (*config.Config, string, *config.Secrets, error) {
	dataDir, err := appdata.ResolveDataDir()
	if err != nil {
		return nil, "", nil, err
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, "", nil, err
	}
	cfgPath := filepath.Join(dataDir, "config.toml")
	secretsPath := secretsFilePath(dataDir)
	secrets, err := config.LoadSecrets(secretsPath)
	if err != nil {
		return nil, "", nil, err
	}
	if config.EnsureSecrets(secrets) {
		if err := config.SaveSecrets(secretsPath, secrets); err != nil {
			return nil, "", nil, err
		}
	}
	if _, statErr := os.Stat(cfgPath); errors.Is(statErr, os.ErrNotExist) {
		if err := os.WriteFile(cfgPath, config.DefaultsTOML(), 0o644); err != nil {
			return nil, "", nil, err
		}
	} else if statErr != nil {
		return nil, "", nil, statErr
	}
	cfg, err := config.LoadGlobal(cfgPath, secretsPath)
	if err != nil {
		return nil, "", nil, err
	}
	if config.EnsureSecrets(secrets) {
		config.ApplySecrets(cfg, secrets)
		_ = config.SaveSecrets(secretsPath, secrets)
	}
	if err := config.Validate(cfg); err != nil {
		return nil, "", nil, err
	}
	return cfg, dataDir, secrets, nil
}

func ResolveFrontendFS() (fs.FS, error) {
	rawDir, hasOverride := os.LookupEnv(frontendDirEnvVar)
	if hasOverride {
		return resolveFrontendDirFS(strings.TrimSpace(rawDir))
	}
	for _, candidate := range []string{defaultFrontendDir, repoFrontendDir} {
		frontendFS, err := resolveFrontendDirFS(candidate)
		if err == nil && frontendFS != nil {
			return frontendFS, nil
		}
	}
	return nil, nil
}

func resolveFrontendDirFS(frontendDir string) (fs.FS, error) {
	if frontendDir == "" {
		return nil, nil
	}
	info, err := os.Stat(frontendDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("resolve frontend dir %q: %w", frontendDir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("resolve frontend dir %q: not a directory", frontendDir)
	}
	return os.DirFS(frontendDir), nil
}

func secretsFilePath(dataDir string) string {
	tomlPath := filepath.Join(dataDir, "secrets.toml")
	if _, err := os.Stat(tomlPath); err == nil {
		return tomlPath
	}
	yamlPath := filepath.Join(dataDir, "secrets.yaml")
	if _, err := os.Stat(yamlPath); err == nil {
		return yamlPath
	}
	return tomlPath
}

func ExpandStorePath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return trimmed
	}
	if strings.HasPrefix(trimmed, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			trimmed = filepath.Join(home, trimmed[2:])
		}
	}
	if !filepath.IsAbs(trimmed) {
		if abs, err := filepath.Abs(trimmed); err == nil {
			return abs
		}
	}
	return trimmed
}
