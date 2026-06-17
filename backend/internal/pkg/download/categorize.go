package download

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/tiredbooy/Rum/backend/internal/pkg/config"
	filesystem "github.com/tiredbooy/Rum/backend/internal/pkg/file-system"
)

// finalizeCategorize moves a finished download into its category directory when
// auto-organize is requested for this job. It is a no-op when opt.Categorize is
// false, when categories are globally disabled, or when no rule matches the
// file's extension.
//
// It resolves the destination via filesystem.CategorizeFile (which creates the
// destination directory and guards against path traversal), honors the
// configured file-conflict policy, performs an atomic same-filesystem rename
// (falling back to copy+remove across filesystems), and updates job.OutputPath
// to the new location. Errors are returned so the caller can decide whether to
// fail the download; callers currently log-and-continue so a categorize failure
// never loses an already-downloaded file.
func finalizeCategorize(opt Options, job *Job) error {
	if !opt.Categorize {
		return nil
	}

	var setting config.Setting
	_ = setting.LoadSettingMetadata()
	if !setting.EnableCategories {
		return nil
	}

	// A relative DestDir resolves against the download root (OutDir), not the
	// file's content-type subfolder, so "Videos" means <OutDir>/Videos.
	baseDir := opt.Out
	if strings.TrimSpace(baseDir) == "" {
		baseDir = setting.OutDir
	}

	rules := categoryRulesFor(setting.Categories, opt.Category, baseDir)
	if len(rules) == 0 {
		return nil
	}

	src := job.GetOutputPath()
	if src == "" {
		return nil
	}

	dest, err := filesystem.CategorizeFile(src, rules)
	if err != nil {
		return err
	}
	if dest == src {
		return nil // no matching category
	}

	finalDest, err := resolveConflict(dest, setting.FileConflict)
	if err != nil {
		return err
	}
	if finalDest == "" {
		// "skip": leave the file where it is.
		return nil
	}

	if err := moveFile(src, finalDest); err != nil {
		return err
	}
	job.SetOutputPath(finalDest)
	return nil
}

// categoryRulesFor converts config.CategoryRule rules into the filesystem mirror
// type. When category is non-empty, only the rule with that (case-insensitive)
// name is returned, forcing that category regardless of extension matching done
// later; otherwise all rules are returned for auto-detection by extension.
//
// A relative DestDir is rebased onto baseDir (the download root) so the resulting
// category folder is a clean top-level directory rather than nested under the
// content-type subfolder where the file currently sits. An absolute DestDir is
// left untouched.
func categoryRulesFor(cfgRules []config.CategoryRule, category, baseDir string) []filesystem.CategoryRule {
	category = strings.TrimSpace(category)
	out := make([]filesystem.CategoryRule, 0, len(cfgRules))
	for _, r := range cfgRules {
		if category != "" && !strings.EqualFold(r.Name, category) {
			continue
		}
		dest := r.DestDir
		if d := strings.TrimSpace(dest); d != "" && !filepath.IsAbs(d) && strings.TrimSpace(baseDir) != "" {
			dest = filepath.Join(baseDir, d)
		}
		out = append(out, filesystem.CategoryRule{
			Name:       r.Name,
			Extensions: r.Extensions,
			DestDir:    dest,
		})
	}
	return out
}

// resolveConflict applies the file-conflict policy to a destination that may
// already exist. It returns the path to write to, or "" when the move should be
// skipped. Policies: "overwrite" removes the existing file; "skip" returns "";
// anything else ("rename" / default) picks a non-colliding "name (n).ext".
func resolveConflict(dest, policy string) (string, error) {
	if !filesystem.IsFileExists(dest) {
		return dest, nil
	}
	switch policy {
	case "overwrite":
		if err := os.Remove(dest); err != nil {
			return "", fmt.Errorf("categorize: overwrite existing: %w", err)
		}
		return dest, nil
	case "skip":
		return "", nil
	default: // "rename"
		return uniquePath(dest), nil
	}
}

// uniquePath returns dest if free, else inserts " (n)" before the extension until
// a free name is found.
func uniquePath(dest string) string {
	ext := filepath.Ext(dest)
	stem := strings.TrimSuffix(dest, ext)
	for i := 1; ; i++ {
		candidate := fmt.Sprintf("%s (%d)%s", stem, i, ext)
		if !filesystem.IsFileExists(candidate) {
			return candidate
		}
	}
}

// moveFile renames src to dest, falling back to copy+remove when the two are on
// different filesystems (os.Rename returns EXDEV).
func moveFile(src, dest string) error {
	if err := os.Rename(src, dest); err == nil {
		return nil
	}
	// Cross-filesystem (or other rename failure): copy then remove the source.
	if err := copyFile(src, dest); err != nil {
		return err
	}
	if err := os.Remove(src); err != nil {
		// The copy succeeded; a failed source removal leaves a duplicate but does
		// not lose data. Surface it so the caller can log it.
		return fmt.Errorf("categorize: remove source after copy: %w", err)
	}
	return nil
}

func copyFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("categorize: open source: %w", err)
	}
	defer in.Close()

	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("categorize: create dest: %w", err)
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		_ = os.Remove(dest)
		return fmt.Errorf("categorize: copy: %w", err)
	}
	if err := out.Sync(); err != nil {
		out.Close()
		return fmt.Errorf("categorize: sync dest: %w", err)
	}
	return out.Close()
}
