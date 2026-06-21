package download

import (
	"path/filepath"
	"testing"

	"github.com/tiredbooy/Rum/backend/internal/pkg/config"
)

// TestCategoryDestDirConfinedToRoot guards B3: a relative category DestDir must
// not escape the download root via "..", while plain subfolders and explicit
// absolute paths keep working.
func TestCategoryDestDirConfinedToRoot(t *testing.T) {
	base := filepath.Join(t.TempDir(), "Downloads")
	rules := []config.CategoryRule{
		{Name: "evil", Extensions: []string{".mp4"}, DestDir: "../../../etc/cron.d"},
		{Name: "ok", Extensions: []string{".pdf"}, DestDir: "Documents"},
		{Name: "nested", Extensions: []string{".zip"}, DestDir: "a/b"},
		{Name: "abs", Extensions: []string{".iso"}, DestDir: "/mnt/storage"},
	}

	byName := map[string]string{}
	for _, r := range categoryRulesFor(rules, "", base) {
		byName[r.Name] = r.DestDir
	}

	if d := byName["evil"]; !isWithinDir(base, d) {
		t.Errorf("traversal dest %q escaped the download root %q", d, base)
	}
	if d := byName["ok"]; d != filepath.Join(base, "Documents") {
		t.Errorf("relative dest = %q, want %q", d, filepath.Join(base, "Documents"))
	}
	if d := byName["nested"]; d != filepath.Join(base, "a", "b") {
		t.Errorf("nested dest = %q, want %q", d, filepath.Join(base, "a", "b"))
	}
	if d := byName["abs"]; d != "/mnt/storage" {
		t.Errorf("absolute dest should be preserved as-is, got %q", d)
	}
}
