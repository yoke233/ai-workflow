package skills

import (
	"bytes"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/yoke233/ai-workflow/internal/skills/builtin"
)

// EnsureBuiltinSkills extracts embedded builtin skills to skillsRoot.
// It skips extraction only when the on-disk content already matches the embedded skill.
func EnsureBuiltinSkills(skillsRoot string) error {
	if err := os.MkdirAll(skillsRoot, 0o755); err != nil {
		return fmt.Errorf("create skills root: %w", err)
	}

	// Walk the embedded FS to discover top-level skill directories.
	entries, err := fs.ReadDir(builtin.AllBuiltinFS, ".")
	if err != nil {
		return fmt.Errorf("read builtin FS root: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue // skip embed.go etc.
		}
		skillName := entry.Name()
		if !IsValidName(skillName) {
			continue
		}

		if shouldSkip(builtin.AllBuiltinFS, skillsRoot, skillName) {
			slog.Debug("builtin skill up-to-date, skipping", "skill", skillName)
			continue
		}

		slog.Info("extracting builtin skill", "skill", skillName, "target", skillsRoot)
		if err := extractSkillDir(builtin.AllBuiltinFS, skillsRoot, skillName); err != nil {
			return fmt.Errorf("extract builtin skill %q: %w", skillName, err)
		}
	}
	return nil
}

// shouldSkip returns true if every embedded file for the skill matches the on-disk copy.
func shouldSkip(embeddedFS fs.FS, skillsRoot, skillName string) bool {
	matches := true
	err := fs.WalkDir(embeddedFS, skillName, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			matches = false
			return walkErr
		}
		if d.IsDir() {
			if path == skillName {
				return nil
			}
			diskDir := filepath.Join(skillsRoot, filepath.FromSlash(path))
			fi, err := os.Stat(diskDir)
			if err != nil || !fi.IsDir() {
				matches = false
				return fs.SkipAll
			}
			return nil
		}

		embeddedContent, err := fs.ReadFile(embeddedFS, path)
		if err != nil {
			matches = false
			return err
		}
		diskPath := filepath.Join(skillsRoot, filepath.FromSlash(path))
		diskContent, err := os.ReadFile(diskPath)
		if err != nil {
			matches = false
			return fs.SkipAll
		}
		if !bytes.Equal(normalizeLineEndings(diskContent), normalizeLineEndings(embeddedContent)) {
			matches = false
			return fs.SkipAll
		}
		return nil
	})
	if err != nil && err != fs.SkipAll {
		return false
	}
	return matches
}

// extractSkillDir writes all files from embeddedFS/<skillName>/ into skillsRoot/<skillName>/.
func extractSkillDir(embeddedFS fs.FS, skillsRoot, skillName string) error {
	return fs.WalkDir(embeddedFS, skillName, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		// Compute the target path on disk.
		rel := path // path is already relative to the FS root
		target := filepath.Join(skillsRoot, filepath.FromSlash(rel))

		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}

		content, err := fs.ReadFile(embeddedFS, path)
		if err != nil {
			return fmt.Errorf("read embedded %s: %w", path, err)
		}

		// Ensure parent directory exists.
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", filepath.Dir(target), err)
		}

		// Normalize line endings: embedded files use LF, convert to OS default.
		normalized := normalizeLineEndings(content)
		return os.WriteFile(target, normalized, 0o644)
	})
}

// normalizeLineEndings replaces \r\n with \n for consistency.
func normalizeLineEndings(b []byte) []byte {
	return []byte(strings.ReplaceAll(string(b), "\r\n", "\n"))
}
