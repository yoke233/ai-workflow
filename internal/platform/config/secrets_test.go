package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadSecrets_StrictRejectsLegacyNestedPATFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "secrets.toml")
	content := `
[github]
commit_pat = "legacy-token"
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write secrets: %v", err)
	}

	_, err := LoadSecrets(path)
	if err == nil {
		t.Fatal("expected legacy nested PAT fields to be rejected")
	}
	if !strings.Contains(err.Error(), "strict mode") {
		t.Fatalf("expected strict-mode decode error, got %v", err)
	}
}

func TestLoadSecrets_TopLevelPATFieldsStillLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "secrets.toml")
	content := `
commit_pat = "commit-token"
merge_pat = "merge-token"
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write secrets: %v", err)
	}

	secrets, err := LoadSecrets(path)
	if err != nil {
		t.Fatalf("LoadSecrets error: %v", err)
	}
	if got := secrets.CommitPAT; got != "commit-token" {
		t.Fatalf("CommitPAT = %q, want commit-token", got)
	}
	if got := secrets.MergePAT; got != "merge-token" {
		t.Fatalf("MergePAT = %q, want merge-token", got)
	}
}
