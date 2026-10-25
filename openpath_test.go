package main

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestOpenCommand(t *testing.T) {
	cases := []struct {
		goos string
		want []string
	}{
		{"linux", []string{"xdg-open", "/d/f.zip"}},
		{"darwin", []string{"open", "/d/f.zip"}},
		{"windows", []string{"explorer", "/d/f.zip"}},
	}
	for _, c := range cases {
		if got := openCommand(c.goos, "/d/f.zip"); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got %v want %v", c.goos, got, c.want)
		}
	}
}

func TestRevealCommands(t *testing.T) {
	got := revealCommands("windows", `C:\d\f.zip`)
	if want := [][]string{{"explorer", `/select,C:\d\f.zip`}}; !reflect.DeepEqual(got, want) {
		t.Errorf("windows: got %v want %v", got, want)
	}
	got = revealCommands("darwin", "/d/f.zip")
	if want := [][]string{{"open", "-R", "/d/f.zip"}}; !reflect.DeepEqual(got, want) {
		t.Errorf("darwin: got %v want %v", got, want)
	}
	got = revealCommands("linux", "/d/my file.zip")
	if len(got) != 2 || got[0][0] != "dbus-send" || got[1][0] != "xdg-open" {
		t.Fatalf("linux: unexpected %v", got)
	}
	if got[1][1] != "/d" {
		t.Errorf("linux fallback should open the parent dir, got %v", got[1])
	}
	joined := strings.Join(got[0], " ")
	if !strings.Contains(joined, "org.freedesktop.FileManager1.ShowItems") ||
		!strings.Contains(joined, "array:string:file:///d/my%20file.zip") {
		t.Errorf("linux dbus args wrong: %v", got[0])
	}
}

func TestValidateKnownPath(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "a.bin")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"", "relative/a.bin", "-rf", filepath.Join(dir, "missing"), "/x\x00y"} {
		if _, err := validateKnownPath(bad); err == nil {
			t.Errorf("%q should be rejected", bad)
		}
	}
	if isDir, err := validateKnownPath(file); err != nil || isDir {
		t.Errorf("file: isDir=%v err=%v", isDir, err)
	}
	if isDir, err := validateKnownPath(dir); err != nil || !isDir {
		t.Errorf("dir: isDir=%v err=%v", isDir, err)
	}
}

func TestRunFirstSuccessful(t *testing.T) {
	var calls []string
	run := func(argv []string) error {
		calls = append(calls, argv[0])
		if argv[0] == "dbus-send" {
			return errors.New("no bus")
		}
		return nil
	}
	if err := runFirst(run, revealCommands("linux", "/d/f")); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(calls, []string{"dbus-send", "xdg-open"}) {
		t.Errorf("calls = %v", calls)
	}
	fail := func([]string) error { return errors.New("x") }
	if err := runFirst(fail, revealCommands("linux", "/d/f")); err == nil {
		t.Error("expected error when all fail")
	}
}

func TestTargetForDownload(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "a.bin")
	_ = os.WriteFile(file, []byte("x"), 0o600)

	done := downloadItem{Status: "completed", OutputPath: file}
	if p, err := targetFor(actionFolder, done); err != nil || p != dir {
		t.Errorf("folder of completed: %q %v", p, err)
	}
	if p, err := targetFor(actionReveal, done); err != nil || p != file {
		t.Errorf("reveal: %q %v", p, err)
	}
	if p, err := targetFor(actionOpen, done); err != nil || p != file {
		t.Errorf("open: %q %v", p, err)
	}
	// Open file only for completed downloads.
	if _, err := targetFor(actionOpen, downloadItem{Status: "downloading", OutputPath: file}); err == nil {
		t.Error("open must be refused unless completed")
	}
	// disk_full: the destination volume folder.
	if p, err := targetFor(actionFolder, downloadItem{ErrorCode: "disk_full", DiskPath: dir}); err != nil || p != dir {
		t.Errorf("disk_full folder: %q %v", p, err)
	}
	// Unknown / missing output.
	if _, err := targetFor(actionFolder, downloadItem{Status: "completed", OutputPath: filepath.Join(dir, "gone", "x")}); err == nil {
		t.Error("missing dir must error")
	}
	if _, err := targetFor(actionFolder, downloadItem{Status: "queued"}); err == nil {
		t.Error("no path known must error")
	}
}

func TestExplorerRawCmdLine(t *testing.T) {
	got := explorerRawCmdLine(`C:\My Files\a b.zip`)
	want := `explorer /select,"C:\My Files\a b.zip"`
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestDiskFullFolderFallsBackToExistingParent(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "not", "yet")
	p, err := targetFor(actionFolder, downloadItem{ErrorCode: "disk_full", DiskPath: missing})
	if err != nil || p != dir {
		t.Fatalf("got %q %v, want %q", p, err, dir)
	}
}
