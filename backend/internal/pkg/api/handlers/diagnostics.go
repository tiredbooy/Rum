package handlers

import (
	"bufio"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"

	"github.com/gin-gonic/gin"
	"github.com/tiredbooy/Rum/backend/internal/pkg/config"
	"github.com/tiredbooy/Rum/backend/internal/pkg/download"
	filesystem "github.com/tiredbooy/Rum/backend/internal/pkg/file-system"
)

// AppVersion is reported in diagnostics. Override at build time with
// -ldflags "-X github.com/tiredbooy/Rum/backend/internal/pkg/api/handlers.AppVersion=...".
var AppVersion = "1.0.0"

// diagLogLines is how many trailing log lines a diagnostics report carries.
const diagLogLines = 50

// JobCounts summarises the download list without exposing any URL or name.
type JobCounts struct {
	Total       int            `json:"total"`
	ByStatus    map[string]int `json:"by_status"`
	ByErrorCode map[string]int `json:"by_error_code"`
}

// Diagnostics is the support report the Settings "Copy diagnostics" button builds.
type Diagnostics struct {
	AppVersion string                 `json:"app_version"`
	OS         string                 `json:"os"`
	Arch       string                 `json:"arch"`
	GoVersion  string                 `json:"go_version"`
	Settings   map[string]interface{} `json:"settings"`
	Jobs       JobCounts              `json:"jobs"`
	Log        []string               `json:"log_tail,omitempty"`
}

// secretKeyRe matches setting keys that must never be reported even if added later.
var secretKeyRe = regexp.MustCompile(`(?i)pass|token|cookie|secret|auth|credential|api_?key`)

// secretParamRe matches key=value / key: value pairs that carry credentials.
var secretParamRe = regexp.MustCompile(`(?i)\b(token|access_token|auth|authorization|password|passwd|pwd|secret|api_?key|key|hash|credential|jwt|sig|signature|session|sid|cookie|set-cookie)\b(\s*[=:]\s*)(?:bearer\s+|basic\s+)?[^&\s;"']+`)

// userinfoRe matches user:pass@ inside a URL.
var userinfoRe = regexp.MustCompile(`(://)[^/\s:@]+(:[^/\s@]*)?@`)

// redactProxy hides the credentials of a proxy URL, keeping scheme and host.
func redactProxy(raw string) string {
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "[redacted]"
	}
	if u.User != nil {
		if _, hasPw := u.User.Password(); hasPw {
			u.User = url.UserPassword(u.User.Username(), "xxxxx")
		} else {
			u.User = url.User("xxxxx")
		}
	}
	return u.String()
}

// redactLine strips credentials from a free-form log line.
func redactLine(s string) string {
	s = userinfoRe.ReplaceAllString(s, "${1}xxxxx@")
	return secretParamRe.ReplaceAllString(s, "${1}${2}[redacted]")
}

// redactedSettings returns the settings as a generic map with secrets removed.
func redactedSettings(s config.Setting) map[string]interface{} {
	raw, _ := json.Marshal(s)
	m := map[string]interface{}{}
	_ = json.Unmarshal(raw, &m)
	for k, v := range m {
		switch {
		case k == "proxy":
			if str, ok := v.(string); ok {
				m[k] = redactProxy(str)
			}
		case secretKeyRe.MatchString(k):
			m[k] = "[redacted]"
		}
	}
	return m
}

// BuildDiagnostics assembles a report from already-loaded inputs (pure, testable).
func BuildDiagnostics(s config.Setting, jobs []*download.Job, logTail []string) Diagnostics {
	counts := JobCounts{Total: len(jobs), ByStatus: map[string]int{}, ByErrorCode: map[string]int{}}
	for _, j := range jobs {
		counts.ByStatus[j.GetStatus()]++
		if code, _ := j.GetErrorCode(); code != "" {
			counts.ByErrorCode[code]++
		}
	}
	lines := make([]string, 0, len(logTail))
	for _, l := range logTail {
		lines = append(lines, redactLine(l))
	}
	return Diagnostics{
		AppVersion: AppVersion,
		OS:         runtime.GOOS,
		Arch:       runtime.GOARCH,
		GoVersion:  runtime.Version(),
		Settings:   redactedSettings(s),
		Jobs:       counts,
		Log:        lines,
	}
}

// tailFile returns the last n lines of a text file, or nil if it can't be read.
func tailFile(path string, n int) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	ring := make([]string, 0, n)
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1<<20)
	for sc.Scan() {
		if len(ring) == n {
			ring = ring[1:]
		}
		ring = append(ring, sc.Text())
	}
	return ring
}

// GetDiagnostics returns a redacted support report.
//
//	GET /api/v1/diagnostics
func GetDiagnostics(c *gin.Context) {
	setting, ok := loadSettings(c)
	if !ok {
		return
	}
	var jobs []*download.Job
	if GlobalManager != nil {
		jobs = GlobalManager.GetAllJobs()
	}
	dir := filepath.Dir(filesystem.CreateMetadataFile("error.log"))
	tail := tailFile(filepath.Join(dir, "error.log"), diagLogLines)
	if len(tail) == 0 {
		tail = tailFile(filepath.Join(dir, "logs", "debug.log"), diagLogLines)
	}
	c.JSON(http.StatusOK, BuildDiagnostics(setting, jobs, tail))
}
