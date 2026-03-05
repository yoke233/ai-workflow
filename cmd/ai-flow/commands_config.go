package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/yoke233/ai-workflow/internal/config"
	"gopkg.in/yaml.v3"
)

func cmdConfigInit(args []string) error {
	force := false
	for _, raw := range args {
		arg := strings.TrimSpace(raw)
		switch arg {
		case "":
			continue
		case "--force", "-f":
			force = true
		default:
			return fmt.Errorf("usage: ai-flow config init [--force]")
		}
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	dataDir := filepath.Join(cwd, ".ai-workflow")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return err
	}

	cfgPath := filepath.Join(dataDir, "config.yaml")
	if !force {
		if _, err := os.Stat(cfgPath); err == nil {
			return fmt.Errorf("config already exists: %s (use --force to overwrite)", cfgPath)
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}

	content, err := loadDefaultConfigTemplate(cwd)
	if err != nil {
		return err
	}
	if err := os.WriteFile(cfgPath, content, 0o644); err != nil {
		return err
	}
	fmt.Printf("Config initialized: %s\n", cfgPath)
	return nil
}

func loadDefaultConfigTemplate(cwd string) ([]byte, error) {
	templatePath := filepath.Join(cwd, "configs", "defaults.yaml")
	data, err := os.ReadFile(templatePath)
	if err == nil {
		return data, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read default config template: %w", err)
	}

	cfg := config.Defaults()
	encoded, err := yaml.Marshal(&cfg)
	if err != nil {
		return nil, fmt.Errorf("marshal default config: %w", err)
	}
	header := []byte("# Auto-generated fallback config template.\n")
	return append(header, encoded...), nil
}
