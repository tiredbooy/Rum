package filesystem

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// AtomicWriteFile writes data to path atomically: it writes to a temp file in
// the SAME directory (so os.Rename stays on one filesystem) and renames it into
// place. This prevents a partially written / corrupt file if the process
// crashes mid-write — important for jobs.json and settings.json.
//
// The temp file is removed on any error so no orphaned *.tmp files accumulate.
func AtomicWriteFile(path string, data []byte, perm os.FileMode) (err error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("atomic write: mkdir %s: %w", dir, err)
	}

	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("atomic write: create temp: %w", err)
	}
	tmpName := tmp.Name()

	// Best-effort cleanup if we don't successfully rename.
	defer func() {
		if err != nil {
			_ = os.Remove(tmpName)
		}
	}()

	if _, err = tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("atomic write: write temp: %w", err)
	}
	// fsync so the rename can't expose a zero-length file after a crash.
	if syncErr := tmp.Sync(); syncErr != nil {
		_ = tmp.Close()
		err = fmt.Errorf("atomic write: sync temp: %w", syncErr)
		return err
	}
	if err = tmp.Close(); err != nil {
		return fmt.Errorf("atomic write: close temp: %w", err)
	}
	if err = os.Chmod(tmpName, perm); err != nil {
		return fmt.Errorf("atomic write: chmod temp: %w", err)
	}
	if err = os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("atomic write: rename: %w", err)
	}
	return nil
}

// SanitizeFileName strips any path components and dangerous characters from a
// filename derived from an untrusted source (URL path, Content-Disposition).
//
// It guarantees the result:
//   - contains no path separators (/, \) and no parent references (..),
//   - is not absolute,
//   - has control characters and Windows-reserved characters removed,
//   - falls back to "download" if nothing usable remains.
//
// The engine should run every remote-derived filename through this before
// joining it onto a download directory to prevent path-traversal writes.
func SanitizeFileName(name string) string {
	// Take only the final path element, defeating "../../etc/passwd" and
	// absolute paths regardless of OS-specific separators. (A drive-letter
	// prefix like "C:" is handled below when we strip ':' from the result.)
	name = strings.ReplaceAll(name, "\\", "/")
	name = name[strings.LastIndex(name, "/")+1:]

	// Remove control chars and characters illegal on common filesystems
	// (including ':' which also disposes of any "C:" drive remnant).
	var b strings.Builder
	for _, r := range name {
		if r < 0x20 || r == 0x7f {
			continue
		}
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|':
			continue
		default:
			b.WriteRune(r)
		}
	}
	cleaned := strings.TrimSpace(b.String())
	cleaned = strings.Trim(cleaned, ". ") // no leading/trailing dots or spaces

	if cleaned == "" || cleaned == "." || cleaned == ".." {
		return "download"
	}
	return cleaned
}

// SafeJoin joins base and an untrusted filename, sanitizing the filename and
// guaranteeing the result stays inside base. It returns an error if, after
// cleaning, the path would escape base (defense in depth).
func SafeJoin(base, untrusted string) (string, error) {
	name := SanitizeFileName(untrusted)
	full := filepath.Join(base, name)

	absBase, err := filepath.Abs(base)
	if err != nil {
		return "", fmt.Errorf("safe join: abs base: %w", err)
	}
	absFull, err := filepath.Abs(full)
	if err != nil {
		return "", fmt.Errorf("safe join: abs full: %w", err)
	}
	// absFull must equal absBase/<name>; verify it is within absBase.
	rel, err := filepath.Rel(absBase, absFull)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("safe join: path escapes base directory")
	}
	return full, nil
}
