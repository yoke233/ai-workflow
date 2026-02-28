package config

import "path/filepath"

const projectConfigRelativePath = ".ai-workflow/config.yaml"

func ProjectConfigPath(repoPath string) string {
	return filepath.Join(repoPath, projectConfigRelativePath)
}
