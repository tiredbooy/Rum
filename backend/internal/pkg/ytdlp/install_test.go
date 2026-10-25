package ytdlp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func releaseServer(t *testing.T, asset string, body []byte, sumFor []byte) *httptest.Server {
	t.Helper()
	sum := sha256.Sum256(sumFor)
	mux := http.NewServeMux()
	mux.HandleFunc("/SHA2-256SUMS", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("deadbeef  other_asset\n" + hex.EncodeToString(sum[:]) + "  " + asset + "\n"))
	})
	mux.HandleFunc("/"+asset, func(w http.ResponseWriter, _ *http.Request) { w.Write(body) })
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestInstallVerifiesChecksum(t *testing.T) {
	body := []byte("#!/bin/sh\necho hi\n")
	srv := releaseServer(t, "yt-dlp_linux", body, body)
	dir := t.TempDir()
	path, err := Installer{BaseURL: srv.URL, GOOS: "linux", GOARCH: "amd64", Dir: dir}.Install(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if path != filepath.Join(dir, "yt-dlp") {
		t.Fatalf("path = %s", path)
	}
	got, _ := os.ReadFile(path)
	if string(got) != string(body) {
		t.Fatal("content differs")
	}
	if runtime.GOOS != "windows" {
		if st, _ := os.Stat(path); st.Mode().Perm()&0o111 == 0 {
			t.Fatalf("not executable: %v", st.Mode())
		}
	}
}

func TestInstallRejectsChecksumMismatch(t *testing.T) {
	srv := releaseServer(t, "yt-dlp_linux", []byte("tampered binary"), []byte("the real binary"))
	dir := t.TempDir()
	_, err := Installer{BaseURL: srv.URL, GOOS: "linux", GOARCH: "amd64", Dir: dir}.Install(context.Background())
	if e, ok := AsError(err); !ok || e.Code != CodeFailed {
		t.Fatalf("err = %v", err)
	}
	if ents, _ := os.ReadDir(dir); len(ents) != 0 {
		t.Fatalf("leftovers after failed install: %v", ents)
	}
}

func TestInstallKeepsOldBinaryOnFailure(t *testing.T) {
	srv := releaseServer(t, "yt-dlp_linux", []byte("bad"), []byte("good"))
	dir := t.TempDir()
	old := filepath.Join(dir, "yt-dlp")
	os.WriteFile(old, []byte("old"), 0o755)
	if _, err := (Installer{BaseURL: srv.URL, GOOS: "linux", GOARCH: "amd64", Dir: dir}).Install(context.Background()); err == nil {
		t.Fatal("expected error")
	}
	if b, _ := os.ReadFile(old); string(b) != "old" {
		t.Fatal("old binary replaced by unverified one")
	}
}

func TestInstallMissingEntryAndHTTPErrors(t *testing.T) {
	srv := releaseServer(t, "yt-dlp_linux", []byte("x"), []byte("x"))
	// windows asset is not in the sums list
	_, err := Installer{BaseURL: srv.URL, GOOS: "windows", GOARCH: "amd64", Dir: t.TempDir()}.Install(context.Background())
	if err == nil {
		t.Fatal("missing sums entry accepted")
	}
	_, err = Installer{BaseURL: srv.URL + "/nope", GOOS: "linux", GOARCH: "amd64", Dir: t.TempDir()}.Install(context.Background())
	if err == nil {
		t.Fatal("404 accepted")
	}
	if _, err = (Installer{GOOS: "plan9", GOARCH: "amd64", Dir: t.TempDir()}).Install(context.Background()); err == nil {
		t.Fatal("unsupported OS accepted")
	}
}

func TestAssetNameAndSums(t *testing.T) {
	cases := [][3]string{
		{"linux", "amd64", "yt-dlp_linux"}, {"linux", "arm64", "yt-dlp_linux_aarch64"},
		{"windows", "amd64", "yt-dlp.exe"}, {"darwin", "arm64", "yt-dlp_macos"}, {"linux", "riscv64", ""},
	}
	for _, c := range cases {
		if got := AssetName(c[0], c[1]); got != c[2] {
			t.Errorf("AssetName(%s,%s) = %q", c[0], c[1], got)
		}
	}
	h := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	if got, ok := ParseSums("junk\n"+h+" *yt-dlp.exe\r\n", "yt-dlp.exe"); !ok || got != h {
		t.Errorf("ParseSums binary-mode line: %q %v", got, ok)
	}
	if _, ok := ParseSums("short  yt-dlp.exe", "yt-dlp.exe"); ok {
		t.Error("short hash accepted")
	}
}

func TestLocateOrder(t *testing.T) {
	// configured path wins
	cfg := filepath.Join(t.TempDir(), "my-ytdlp")
	os.WriteFile(cfg, []byte("x"), 0o755)
	if f, err := Locate(cfg); err != nil || f.Source != SourceConfigured || f.Path != cfg {
		t.Fatalf("configured: %+v %v", f, err)
	}
	// nothing on PATH, nothing managed: missing
	t.Setenv("PATH", t.TempDir())
	if _, err := Locate(""); err == nil {
		t.Fatal("found yt-dlp from nowhere")
	} else if e, ok := AsError(err); !ok || e.Code != CodeMissing {
		t.Fatalf("err = %v", err)
	}
	// managed copy in the tools dir
	if err := os.MkdirAll(ToolsDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(ManagedPath(), []byte("x"), 0o755)
	defer os.Remove(ManagedPath())
	if f, err := Locate("/does/not/exist"); err != nil || f.Source != SourceManaged {
		t.Fatalf("managed: %+v %v", f, err)
	}
	// PATH beats the managed copy
	pdir := t.TempDir()
	os.WriteFile(filepath.Join(pdir, BinaryName(runtime.GOOS)), []byte("x"), 0o755)
	t.Setenv("PATH", pdir)
	if f, err := Locate(""); err != nil || f.Source != SourcePath {
		t.Fatalf("path: %+v %v", f, err)
	}
}

func TestUpdateManagedReinstalls(t *testing.T) {
	body := []byte("new build")
	srv := releaseServer(t, "yt-dlp_linux", body, body)
	dir := t.TempDir()
	bin := filepath.Join(dir, "yt-dlp")
	os.WriteFile(bin, []byte("old build"), 0o755)
	err := Update(context.Background(), Found{Path: bin, Source: SourceManaged},
		Installer{BaseURL: srv.URL, GOOS: "linux", GOARCH: "amd64", Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(bin); string(b) != "new build" {
		t.Fatalf("not updated: %q", b)
	}
}

func TestUpdateExternalRunsSelfUpdate(t *testing.T) {
	bin := fakeBin(t, "")
	if err := Update(context.Background(), Found{Path: bin, Source: SourcePath}, Installer{}); err != nil {
		t.Fatal(err)
	}
	if v := Version(context.Background(), bin); v != "2025.01.01" {
		t.Fatalf("version = %q", v)
	}
}
