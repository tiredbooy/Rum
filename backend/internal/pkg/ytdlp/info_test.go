package ytdlp

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/tiredbooy/Rum/backend/internal/pkg/ytdlp/fakeytdlp"
)

func fakeInfo(t *testing.T) *Info {
	t.Helper()
	info, err := ParseInfo([]byte(fakeytdlp.InfoJSON))
	if err != nil {
		t.Fatal(err)
	}
	return info
}

func keys(opts []Option) []string {
	var k []string
	for _, o := range opts {
		k = append(k, o.Key)
	}
	return k
}

func TestOptionsWithFFmpeg(t *testing.T) {
	opts := Options(fakeInfo(t), true)
	if want := []string{"best", "h720", "h480", "audio"}; !reflect.DeepEqual(keys(opts), want) {
		t.Fatalf("keys = %v, want %v", keys(opts), want)
	}
	if opts[0].Label != "Best (1080p)" || opts[3].Label != "Audio only" {
		t.Fatalf("labels: %+v", opts)
	}
	if opts[0].EstSize != 40000000+1500000 { // 1080p video + best audio
		t.Fatalf("best size = %d", opts[0].EstSize)
	}
}

func TestOptionsWithoutFFmpegAreSingleFile(t *testing.T) {
	opts := Options(fakeInfo(t), false)
	// only the progressive 360p exists; no tiers, audio stream is still listed
	if want := []string{"best", "audio"}; !reflect.DeepEqual(keys(opts), want) {
		t.Fatalf("keys = %v, want %v", keys(opts), want)
	}
	if opts[0].Label != "Best (360p)" || opts[0].EstSize != 3000000 {
		t.Fatalf("best = %+v", opts[0])
	}
}

func TestSelector(t *testing.T) {
	cases := []struct {
		key    string
		ffmpeg bool
		want   string
	}{
		{"best", true, "bv*+ba/b"},
		{"best", false, "b"},
		{"h720", true, "bv*[height<=720]+ba/b[height<=720]"},
		{"h720", false, "b[height<=720]"},
		{"audio", false, "ba/b"},
	}
	for _, c := range cases {
		if got := Selector(c.key, c.ffmpeg); got != c.want {
			t.Errorf("Selector(%s,%v) = %s, want %s", c.key, c.ffmpeg, got, c.want)
		}
	}
	for _, bad := range []string{"", "h", "h12", "best; rm", "h720]+x", "../x", "H720"} {
		if ValidKey(bad) {
			t.Errorf("ValidKey(%q)", bad)
		}
	}
}

func TestParseInfoPlaylistAndEmpty(t *testing.T) {
	in, err := ParseInfo([]byte(`{"title":"List","entries":[{"title":"One","formats":[{"format_id":"1","height":720,"vcodec":"x","acodec":"y"}]}]}`))
	if err != nil || in.Title != "One" {
		t.Fatalf("playlist: %+v %v", in, err)
	}
	if _, err := ParseInfo([]byte(`{"title":"x","formats":[]}`)); err == nil {
		t.Fatal("empty formats accepted")
	}
	if _, err := ParseInfo([]byte(`not json`)); err == nil {
		t.Fatal("garbage accepted")
	}
}

func TestFetchInfoUsesFakeAndSafeArgs(t *testing.T) {
	bin := fakeBin(t, "")
	argsFile := filepath.Join(t.TempDir(), "args")
	t.Setenv(fakeytdlp.ArgsEnv, argsFile)
	info, err := FetchInfo(context.Background(), bin, "https://example.com/v", "socks5://127.0.0.1:1080")
	if err != nil || info.Title != "Test Video: 100% real/fake" {
		t.Fatalf("info=%+v err=%v", info, err)
	}
	b, _ := os.ReadFile(argsFile)
	lines := strings.Fields(string(b))
	if lines[len(lines)-2] != "--" || lines[len(lines)-1] != "https://example.com/v" {
		t.Fatalf("args: %v", lines)
	}
	for _, want := range []string{"-J", "--no-playlist", "--ignore-config", "--proxy"} {
		if !strings.Contains(string(b), want) {
			t.Errorf("missing %s in %v", want, lines)
		}
	}
}

func TestFetchInfoErrors(t *testing.T) {
	bin := fakeBin(t, "unavailable")
	_, err := FetchInfo(context.Background(), bin, "https://example.com/v", "")
	if e, ok := AsError(err); !ok || e.Code != CodeVideoUnavailable {
		t.Fatalf("err = %v", err)
	}
	_, err = FetchInfo(context.Background(), filepath.Join(t.TempDir(), "nope"), "https://example.com/v", "")
	if e, ok := AsError(err); !ok || e.Code != CodeMissing {
		t.Fatalf("missing binary err = %v", err)
	}
	_, err = FetchInfo(context.Background(), bin, "file:///etc/passwd", "")
	if e, ok := AsError(err); !ok || e.Code != CodeVideoUnavailable {
		t.Fatalf("bad scheme err = %v", err)
	}
}
