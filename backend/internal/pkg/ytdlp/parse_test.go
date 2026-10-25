package ytdlp

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseProgressLineTemplate(t *testing.T) {
	p, ok := ParseProgressLine("RUMP|downloading|2500000|10000000|NA|1250000.5|6")
	if !ok || p.Status != "downloading" || p.Downloaded != 2500000 || p.Total != 10000000 ||
		p.Speed != 1250000.5 || p.ETA != 6*time.Second || p.Percent != -1 {
		t.Fatalf("got %+v ok=%v", p, ok)
	}
	// total_bytes missing: fall back to the estimate; "None" speed/eta are 0.
	p, ok = ParseProgressLine("RUMP|downloading|100|NA|900|None|None")
	if !ok || p.Total != 900 || p.Speed != 0 || p.ETA != 0 {
		t.Fatalf("got %+v ok=%v", p, ok)
	}
}

func TestParseProgressLineStock(t *testing.T) {
	p, ok := ParseProgressLine("[download]  45.3% of ~  10.00MiB at    1.00MiB/s ETA 01:05")
	if !ok || p.Percent != 45.3 || p.ETA != 65*time.Second || p.Status != "downloading" {
		t.Fatalf("got %+v ok=%v", p, ok)
	}
	p, ok = ParseProgressLine("[download] 100% of   10.00MiB in 00:08")
	if !ok || p.Status != "finished" {
		t.Fatalf("got %+v ok=%v", p, ok)
	}
}

func TestParseProgressLineIgnoresNoise(t *testing.T) {
	for _, l := range []string{"", "[youtube] abc: Downloading webpage", "RUMP|short", "[download] Destination: x.mp4"} {
		if _, ok := ParseProgressLine(l); ok {
			t.Errorf("%q parsed", l)
		}
	}
}

func TestTrackerCarriesFinishedStreams(t *testing.T) {
	tr := &Tracker{Estimate: 30}
	var last int64
	feed := func(status string, d, tot int64) (int64, int64) {
		dl, total := tr.Feed(Progress{Status: status, Downloaded: d, Total: tot, Percent: -1})
		if dl < last {
			t.Fatalf("progress went back: %d -> %d", last, dl)
		}
		last = dl
		return dl, total
	}
	feed("downloading", 5, 20)
	dl, _ := feed("finished", 20, 20)
	if dl != 20 {
		t.Fatalf("after video stream = %d, want 20", dl)
	}
	dl, total := feed("downloading", 5, 10)
	if dl != 25 || total != 30 {
		t.Fatalf("audio stream: %d/%d, want 25/30", dl, total)
	}
	dl, _ = feed("finished", 10, 10)
	if dl != 30 {
		t.Fatalf("end = %d", dl)
	}
}

func TestTrackerStockPercent(t *testing.T) {
	tr := &Tracker{Estimate: 1000}
	d, tot := tr.Feed(Progress{Percent: 50})
	if d != 500 || tot != 1000 {
		t.Fatalf("%d/%d", d, tot)
	}
}

func TestSafeBase(t *testing.T) {
	cases := map[string]string{
		"Test Video: 100% real/fake": "Test Video_ 100_ real_fake",
		"../../etc/passwd":           "_.._etc_passwd",
		"   ":                        "video",
		"سلام دنیا":                  "سلام دنیا",
		"a\x00b\nc":                  "abc",
	}
	for in, want := range cases {
		got := SafeBase(in)
		if got != want || strings.ContainsAny(got, `/\%`) {
			t.Errorf("SafeBase(%q) = %q, want %q", in, got, want)
		}
	}
	if n := len([]rune(SafeBase(strings.Repeat("x", 500)))); n != maxBaseRunes {
		t.Errorf("long title kept %d runes", n)
	}
}

func TestBuildArgsAreSafe(t *testing.T) {
	args := BuildArgs(Spec{Bin: "yt-dlp", URL: "https://example.com/watch?v=-evil", Dir: "/d", Base: "t",
		Selector: "b", Proxy: "http://127.0.0.1:8080", LimitKBps: 500})
	has := func(s string) int {
		for i, a := range args {
			if a == s {
				return i
			}
		}
		return -1
	}
	if has("--ignore-config") < 0 || has("--continue") < 0 || has("--no-playlist") < 0 {
		t.Fatalf("missing hardening/resume flags: %v", args)
	}
	for _, a := range args {
		if strings.HasPrefix(a, "--exec") || a == "--config-locations" {
			t.Fatalf("forbidden flag %q", a)
		}
	}
	dd := has("--")
	if dd < 0 || dd != len(args)-2 || args[len(args)-1] != "https://example.com/watch?v=-evil" {
		t.Fatalf("URL must be the last arg, right after --: %v", args)
	}
	if i := has("--proxy"); i < 0 || args[i+1] != "http://127.0.0.1:8080" {
		t.Fatalf("proxy missing: %v", args)
	}
	if i := has("--limit-rate"); i < 0 || args[i+1] != "500K" {
		t.Fatalf("limit missing: %v", args)
	}
	if i := has("-o"); i < 0 || !strings.HasSuffix(args[i+1], ".%(ext)s") {
		t.Fatalf("output template wrong: %v", args)
	}
}

func TestValidateURL(t *testing.T) {
	for _, u := range []string{"https://youtu.be/x", "http://aparat.com/v/abc"} {
		if ValidateURL(u) != nil {
			t.Errorf("%s rejected", u)
		}
	}
	for _, u := range []string{"", "file:///etc/passwd", "ftp://x/y", "javascript:alert(1)", "--exec=id", "https://", "youtu.be/x"} {
		if ValidateURL(u) == nil {
			t.Errorf("%q accepted", u)
		}
	}
}

func TestClassifyStderr(t *testing.T) {
	cases := map[string]string{
		"ERROR: [youtube] x: Video unavailable":                                                              CodeVideoUnavailable,
		"ERROR: [youtube] x: Private video. Sign in":                                                         CodeVideoUnavailable,
		"ERROR: Unsupported URL: https://example.com":                                                        CodeVideoUnavailable,
		"ERROR: Unable to download webpage: <urlopen error [Errno -3] Temporary failure in name resolution>": "dns",
		"ERROR: unable to connect to proxy":                                                                  "proxy",
		"ERROR: unable to write data: [Errno 28] No space left on device":                                    "disk_full",
		"ERROR: weird": CodeFailed,
	}
	for in, want := range cases {
		if got := ClassifyStderr(in).Code; got != want {
			t.Errorf("%q -> %s, want %s", in, got, want)
		}
	}
	if strings.Contains(ClassifyStderr("ERROR: token=secret").Message, "secret") {
		t.Error("message leaks stderr")
	}
}

func TestBuildArgsEscapesPercentInFolder(t *testing.T) {
	args := BuildArgs(Spec{URL: "https://example.com/v", Dir: "/home/u/100%/%(title)s", Base: "t", Selector: "b"})
	for i, a := range args {
		if a == "-o" {
			want := filepath.Join("/home/u/100%%/%%(title)s", "t") + ".%(ext)s"
			if args[i+1] != want {
				t.Fatalf("template = %q, want %q", args[i+1], want)
			}
			return
		}
	}
	t.Fatal("no -o")
}
