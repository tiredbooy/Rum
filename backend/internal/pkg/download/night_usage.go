package download

import (
	"encoding/json"
	"log"
	"os"
	"time"

	filesystem "github.com/tiredbooy/Rum/backend/internal/pkg/file-system"
)

// nightUsageFile persists the night data cap's spent bytes so a restart in the
// middle of a window does not hand out a fresh allowance.
const nightUsageFile = "night_usage.json"

type nightUsageRecord struct {
	Window string `json:"window"` // the window's start date, local (YYYY-MM-DD)
	Used   int64  `json:"used"`
}

// nightWindowKey names a window by its start date, so a window that crosses
// midnight (or a DST change) stays one window.
func nightWindowKey(open time.Time) string { return open.Format("2006-01-02") }

// loadNightUsage reads the saved usage; a missing or corrupt file means none.
func loadNightUsage(path string) (window string, used int64) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", 0
	}
	var r nightUsageRecord
	if json.Unmarshal(data, &r) != nil || r.Used < 0 {
		return "", 0
	}
	return r.Window, r.Used
}

// saveNightUsage writes the usage atomically.
func saveNightUsage(path, window string, used int64) {
	data, _ := json.Marshal(nightUsageRecord{Window: window, Used: used})
	if err := filesystem.AtomicWriteFile(path, data, 0o600); err != nil {
		log.Printf("night: saving cap usage: %v", err)
	}
}
