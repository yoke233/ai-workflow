package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCmdConfigInitCreatesConfigFromTemplateFile(t *testing.T) {
	wd := t.TempDir()
	t.Chdir(wd)

	template := "server:\n  port: 18080\n"
	if err := os.MkdirAll(filepath.Join(wd, "configs"), 0o755); err != nil {
		t.Fatalf("mkdir configs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(wd, "configs", "defaults.yaml"), []byte(template), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}

	if err := cmdConfigInit(nil); err != nil {
		t.Fatalf("cmdConfigInit() error = %v", err)
	}

	gotPath := filepath.Join(wd, ".ai-workflow", "config.yaml")
	got, err := os.ReadFile(gotPath)
	if err != nil {
		t.Fatalf("read generated config: %v", err)
	}
	if string(got) != template {
		t.Fatalf("generated config mismatch\ngot:\n%s\nwant:\n%s", string(got), template)
	}
}

func TestCmdConfigInitReturnsErrorWhenConfigExists(t *testing.T) {
	wd := t.TempDir()
	t.Chdir(wd)

	cfgPath := filepath.Join(wd, ".ai-workflow", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatalf("mkdir data dir: %v", err)
	}
	if err := os.WriteFile(cfgPath, []byte("existing: true\n"), 0o644); err != nil {
		t.Fatalf("write existing config: %v", err)
	}

	err := cmdConfigInit(nil)
	if err == nil {
		t.Fatal("expected conflict error when config exists")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected already exists error, got %v", err)
	}
}

func TestCmdConfigInitForceOverwritesConfig(t *testing.T) {
	wd := t.TempDir()
	t.Chdir(wd)

	if err := os.MkdirAll(filepath.Join(wd, "configs"), 0o755); err != nil {
		t.Fatalf("mkdir configs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(wd, "configs", "defaults.yaml"), []byte("server:\n  port: 28080\n"), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}

	cfgPath := filepath.Join(wd, ".ai-workflow", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatalf("mkdir data dir: %v", err)
	}
	if err := os.WriteFile(cfgPath, []byte("server:\n  port: 8080\n"), 0o644); err != nil {
		t.Fatalf("write existing config: %v", err)
	}

	if err := cmdConfigInit([]string{"--force"}); err != nil {
		t.Fatalf("cmdConfigInit(--force) error = %v", err)
	}

	got, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read overwritten config: %v", err)
	}
	if string(got) != "server:\n  port: 28080\n" {
		t.Fatalf("expected overwritten template, got:\n%s", string(got))
	}
}

func TestCmdConfigInitFallbackWhenTemplateMissing(t *testing.T) {
	wd := t.TempDir()
	t.Chdir(wd)

	if err := cmdConfigInit(nil); err != nil {
		t.Fatalf("cmdConfigInit() fallback error = %v", err)
	}

	cfgPath := filepath.Join(wd, ".ai-workflow", "config.yaml")
	got, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read generated fallback config: %v", err)
	}
	text := string(got)
	if !strings.Contains(text, "store:") {
		t.Fatalf("fallback config should contain store section, got:\n%s", text)
	}
}

func TestCLIConfigCommandUsageError(t *testing.T) {
	err := runWithArgs([]string{"config"})
	if err == nil {
		t.Fatal("expected usage error for missing config subcommand")
	}
	if !strings.Contains(err.Error(), "usage: ai-flow config <init> [--force]") {
		t.Fatalf("unexpected usage error: %v", err)
	}
}

func TestCLIConfigInitCommandRoute(t *testing.T) {
	wd := t.TempDir()
	t.Chdir(wd)

	if err := os.MkdirAll(filepath.Join(wd, "configs"), 0o755); err != nil {
		t.Fatalf("mkdir configs: %v", err)
	}
	template := "server:\n  port: 39090\n"
	if err := os.WriteFile(filepath.Join(wd, "configs", "defaults.yaml"), []byte(template), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}

	if err := runWithArgs([]string{"config", "init"}); err != nil {
		t.Fatalf("runWithArgs(config init) error = %v", err)
	}

	cfgPath := filepath.Join(wd, ".ai-workflow", "config.yaml")
	got, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read generated config: %v", err)
	}
	if string(got) != template {
		t.Fatalf("generated config mismatch\ngot:\n%s\nwant:\n%s", string(got), template)
	}
}
