package handlers

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiredbooy/Rum/backend/internal/pkg/config"
	"github.com/tiredbooy/Rum/backend/internal/pkg/download"
)

func TestRedactProxy(t *testing.T) {
	cases := map[string]string{
		"":                                    "",
		"http://user:hunter2@proxy.test:8080": "http://user:xxxxx@proxy.test:8080",
		"socks5://proxy.test:1080":            "socks5://proxy.test:1080",
		"http://tokenonly@proxy.test":         "http://xxxxx@proxy.test",
		"::not a url::":                       "[redacted]",
	}
	for in, want := range cases {
		got := redactProxy(in)
		if strings.Contains(got, "hunter2") || strings.Contains(got, "tokenonly") {
			t.Fatalf("%q leaked: %q", in, got)
		}
		if in != "::not a url::" && got != want {
			t.Fatalf("redactProxy(%q) = %q, want %q", in, got, want)
		}
		if in == "::not a url::" && got != want {
			t.Fatalf("unparseable proxy must be fully redacted, got %q", got)
		}
	}
}

func TestRedactLine(t *testing.T) {
	line := "GET https://h.test/f?token=abc123&x=1 Cookie: sid=zzz proxy http://u:pw@p.test:1 Authorization: Bearer qqq"
	got := redactLine(line)
	for _, leak := range []string{"abc123", "zzz", "pw@", "qqq"} {
		if strings.Contains(got, leak) {
			t.Fatalf("leaked %q in %q", leak, got)
		}
	}
	if !strings.Contains(got, "x=1") {
		t.Fatalf("non-secret params should survive: %q", got)
	}
}

func TestBuildDiagnosticsRedactsAndCounts(t *testing.T) {
	s := config.Setting{Proxy: "http://u:hunter2@proxy.test:8080", OutDir: "/dl"}
	e := &download.Job{ID: "1", Status: download.StatusError}
	e.SetError(errors.New("connection reset by peer"))
	jobs := []*download.Job{
		e,
		{ID: "2", Status: download.StatusPaused},
		{ID: "3", Status: download.StatusPaused},
	}
	d := BuildDiagnostics(s, jobs, []string{"line token=SECRET1"})
	raw, _ := json.Marshal(d)
	out := string(raw)
	if strings.Contains(out, "hunter2") || strings.Contains(out, "SECRET1") {
		t.Fatalf("diagnostics leaked a secret: %s", out)
	}
	if d.Jobs.Total != 3 || d.Jobs.ByStatus[download.StatusPaused] != 2 || d.Jobs.ByStatus[download.StatusError] != 1 {
		t.Fatalf("bad counts: %+v", d.Jobs)
	}
	if len(d.Jobs.ByErrorCode) != 1 {
		t.Fatalf("expected one error code bucket: %+v", d.Jobs.ByErrorCode)
	}
	if d.OS == "" || d.Arch == "" || d.GoVersion == "" || d.AppVersion == "" {
		t.Fatalf("missing environment fields: %+v", d)
	}
	if d.Settings["out_dir"] != "/dl" {
		t.Fatalf("plain settings should stay: %v", d.Settings)
	}
}

func TestTailFileReturnsLastLines(t *testing.T) {
	p := filepath.Join(t.TempDir(), "x.log")
	var b strings.Builder
	for i := 0; i < 100; i++ {
		b.WriteString("l")
		b.WriteString(string(rune('a' + i%26)))
		b.WriteString("\n")
	}
	_ = os.WriteFile(p, []byte(b.String()), 0o644)
	got := tailFile(p, 5)
	if len(got) != 5 {
		t.Fatalf("want 5 lines, got %d", len(got))
	}
	if tailFile(filepath.Join(t.TempDir(), "missing"), 5) != nil {
		t.Fatal("missing file => nil")
	}
}
