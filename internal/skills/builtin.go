package skills

import (
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/yoke233/ai-workflow/internal/skills/builtin"
)

// EnsureBuiltinSkills extracts embedded builtin skills to skillsRoot.
// It overwrites on version mismatch (reads SKILL.md frontmatter version).
// Skills whose on-disk version matches the embedded version are skipped.
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

// shouldSkip returns true if the on-disk skill has the same version as the embedded one.
func shouldSkip(embeddedFS fs.FS, skillsRoot, skillName string) bool {
	diskPath := filepath.Join(skillsRoot, skillName, "SKILL.md")
	diskContent, err := os.ReadFile(diskPath)
	if err != nil {
		return false // not present or unreadable → extract
	}
	diskMeta, diskErrs := parseSkillMD(skillName, string(diskContent))
	if len(diskErrs) > 0 || diskMeta == nil {
		return false // invalid → overwrite
	}

	embeddedContent, err := fs.ReadFile(embeddedFS, skillName+"/SKILL.md")
	if err != nil {
		return false
	}
	embeddedMeta, embeddedErrs := parseSkillMD(skillName, string(embeddedContent))
	if len(embeddedErrs) > 0 || embeddedMeta == nil {
		return false
	}

	return diskMeta.Version == embeddedMeta.Version
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
