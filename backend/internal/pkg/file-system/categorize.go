package filesystem

import (
	"fmt"
	"path/filepath"
	"strings"
)

// CategoryRule is a filesystem-local mirror of config.CategoryRule. It is
// duplicated here (rather than imported) to avoid a config -> filesystem ->
// config import cycle: config already imports this package. The engine converts
// config.CategoryRule into this type at the call site.
type CategoryRule struct {
	Name       string
	Extensions []string // e.g. [".mp4", ".mkv"]
	DestDir    string   // absolute, or relative to the file's current directory
}

// CategorizeFile resolves the destination path for a finished download based on
// its extension and the given rules. It does NOT move the file — it returns the
// intended destination (and creates the destination directory) so the engine can
// perform the actual rename while honoring its conflict policy.
//
// Matching is case-insensitive on the file extension. The first rule whose
// Extensions contains the file's extension wins. If no rule matches, the
// original path is returned unchanged and no directory is created.
//
// Destination resolution:
//   - An absolute DestDir is used as-is (the user's explicit choice).
//   - A relative DestDir is resolved against the file's current directory and is
//     rejected if it would escape that directory (path-traversal guard).
//   - The filename is always sanitized and re-validated via SafeJoin so a crafted
//     basename cannot escape the destination directory.
func CategorizeFile(path string, rules []CategoryRule) (dest string, err error) {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == "" {
		return path, nil
	}

	var match *CategoryRule
	for i := range rules {
		for _, e := range rules[i].Extensions {
			if strings.ToLower(strings.TrimSpace(e)) == ext {
				match = &rules[i]
				break
			}
		}
		if match != nil {
			break
		}
	}
	if match == nil {
		return path, nil // no category for this extension: leave in place
	}

	fileDir := filepath.Dir(path)
	destDir, err := resolveDestDir(fileDir, match.DestDir)
	if err != nil {
		return "", err
	}

	// Guard the filename itself (defends against a crafted basename) and ensure
	// the final path stays within destDir.
	full, err := SafeJoin(destDir, filepath.Base(path))
	if err != nil {
		return "", err
	}

	// Create the destination directory so the engine's rename succeeds.
	CreateGroupFolder(destDir)

	return full, nil
}

// resolveDestDir turns a rule's DestDir into a concrete directory. An empty
// DestDir means "leave the file in its current directory". An absolute DestDir is
// cleaned and used directly. A relative DestDir is joined onto fileDir and
// rejected if it escapes fileDir.
func resolveDestDir(fileDir, destDir string) (string, error) {
	destDir = strings.TrimSpace(destDir)
	if destDir == "" {
		return fileDir, nil
	}
	if filepath.IsAbs(destDir) {
		return filepath.Clean(destDir), nil
	}

	joined := filepath.Clean(filepath.Join(fileDir, destDir))
	absBase, err := filepath.Abs(fileDir)
	if err != nil {
		return "", fmt.Errorf("categorize: abs base: %w", err)
	}
	absJoined, err := filepath.Abs(joined)
	if err != nil {
		return "", fmt.Errorf("categorize: abs dest: %w", err)
	}
	rel, err := filepath.Rel(absBase, absJoined)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("categorize: dest dir %q escapes the download directory", destDir)
	}
	return joined, nil
}
