package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"time"
)

// Open-folder / reveal / open-file support. The frontend passes a download ID,
// never a path: the path is read back from the engine, so only locations Rum
// itself produced can be opened. Commands run via exec.Command with an argv
// (no shell), and the path is always absolute so it can never parse as a flag.

type openAction int

const (
	actionFolder openAction = iota // open the containing folder
	actionReveal                   // select the file in the file manager
	actionOpen                     // open the file with its default app
)

// openCommand is the argv that opens path with the OS default handler.
func openCommand(goos, path string) []string {
	switch goos {
	case "darwin":
		return []string{"open", path}
	case "windows":
		return []string{"explorer", path}
	default:
		return []string{"xdg-open", path}
	}
}

// revealCommands are the argvs to try, in order, to show path selected in the
// file manager. Linux has no universal "select" verb: FileManager1.ShowItems
// works on most desktops and xdg-open on the parent folder is the fallback.
func revealCommands(goos, path string) [][]string {
	switch goos {
	case "darwin":
		return [][]string{{"open", "-R", path}}
	case "windows":
		return [][]string{{"explorer", "/select," + path}}
	default:
		u := url.URL{Scheme: "file", Path: path}
		return [][]string{
			{"dbus-send", "--session", "--print-reply", "--dest=org.freedesktop.FileManager1",
				"--type=method_call", "/org/freedesktop/FileManager1",
				"org.freedesktop.FileManager1.ShowItems",
				"array:string:" + u.String(), "string:"},
			{"xdg-open", filepath.Dir(path)},
		}
	}
}

// validateKnownPath requires an absolute, NUL-free path that exists, and
// reports whether it is a directory.
func validateKnownPath(p string) (isDir bool, err error) {
	if p == "" || strings.ContainsRune(p, 0) || !filepath.IsAbs(p) {
		return false, errors.New("invalid path")
	}
	st, err := os.Stat(filepath.Clean(p))
	if err != nil {
		return false, errors.New("path not found")
	}
	return st.IsDir(), nil
}

// targetFor picks the path an action applies to for a download and validates it.
func targetFor(action openAction, it downloadItem) (string, error) {
	var p string
	switch action {
	case actionFolder:
		if it.ErrorCode == "disk_full" && it.DiskPath != "" {
			p = it.DiskPath
		} else if it.OutputPath != "" {
			p = filepath.Dir(it.OutputPath)
		}
	case actionOpen:
		if it.Status != "completed" {
			return "", errors.New("download is not finished")
		}
		p = it.OutputPath
	default:
		p = it.OutputPath
	}
	if p == "" {
		return "", errors.New("no location known")
	}
	p = filepath.Clean(p)
	if action == actionFolder && it.ErrorCode == "disk_full" {
		p = nearestExisting(p)
	}
	isDir, err := validateKnownPath(p)
	if err != nil {
		return "", err
	}
	if (action == actionFolder) != isDir {
		return "", errors.New("unexpected path type")
	}
	return p, nil
}

// nearestExisting walks up from p to the closest existing ancestor, so a
// disk-full destination that was never created still opens its volume.
func nearestExisting(p string) string {
	for {
		if _, err := os.Stat(p); err == nil {
			return p
		}
		parent := filepath.Dir(p)
		if parent == p {
			return p
		}
		p = parent
	}
}

// explorerRawCmdLine is the exact Windows command line for revealing path.
// Go's argv quoting wraps the whole "/select,path" token in quotes, which
// explorer misparses for paths with spaces, so the line is built by hand.
func explorerRawCmdLine(path string) string {
	return `explorer /select,"` + path + `"`
}

// runFirst runs candidates until one succeeds; it returns the last error.
func runFirst(run func([]string) error, candidates [][]string) error {
	err := errors.New("no command")
	for _, argv := range candidates {
		if err = run(argv); err == nil {
			return nil
		}
	}
	return err
}

// execArgv starts argv without a shell. dbus-send is waited on (its failure
// selects the fallback); file managers are left running, and their exit codes
// are ignored (explorer exits 1 on success).
func execArgv(argv []string) error {
	if len(argv) == 0 {
		return errors.New("empty command")
	}
	if argv[0] == "dbus-send" {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		return exec.CommandContext(ctx, argv[0], argv[1:]...).Run()
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	if goruntime.GOOS == "windows" && argv[0] == "explorer" && len(argv) == 2 && strings.HasPrefix(argv[1], "/select,") {
		setRawCmdLine(cmd, explorerRawCmdLine(strings.TrimPrefix(argv[1], "/select,")))
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

// fetchDownload GETs one download from the loopback engine API.
func (a *App) fetchDownload(id string) (downloadItem, error) {
	var it downloadItem
	base := a.GetApiBase()
	if base == "" || id == "" {
		return it, errors.New("engine not ready")
	}
	resp, err := httpClient.Get(base + "/api/v1/downloads/" + url.PathEscape(id))
	if err != nil {
		return it, errors.New("engine not reachable")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return it, errors.New("download not found")
	}
	if err := json.NewDecoder(resp.Body).Decode(&it); err != nil {
		return it, errors.New("bad engine response")
	}
	return it, nil
}

func (a *App) openFor(id string, action openAction) error {
	it, err := a.fetchDownload(id)
	if err != nil {
		return err
	}
	p, err := targetFor(action, it)
	if err != nil {
		return err
	}
	var cmds [][]string
	if action == actionReveal {
		cmds = revealCommands(goruntime.GOOS, p)
	} else {
		cmds = [][]string{openCommand(goruntime.GOOS, p)}
	}
	if err := runFirst(execArgv, cmds); err != nil {
		log.Printf("desktop: open failed: %v", err)
		return fmt.Errorf("could not open the file manager")
	}
	return nil
}

// OpenDownloadFolder opens the folder holding a download (its destination
// volume's folder when the disk is full).
func (a *App) OpenDownloadFolder(id string) error { return a.openFor(id, actionFolder) }

// RevealDownload shows a download's file selected in the file manager.
func (a *App) RevealDownload(id string) error { return a.openFor(id, actionReveal) }

// OpenDownload opens a completed download with its default application.
func (a *App) OpenDownload(id string) error { return a.openFor(id, actionOpen) }
