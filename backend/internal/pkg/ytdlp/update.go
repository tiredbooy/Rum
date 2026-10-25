package ytdlp

import (
	"bytes"
	"context"
	"github.com/tiredbooy/Rum/backend/nodesktop"
	"os/exec"
	"strings"
	"time"
)

// UpdateTimeout bounds `yt-dlp -U`.
const UpdateTimeout = 3 * time.Minute

// Version returns `yt-dlp --version` for bin, or "" when it cannot be run.
func Version(ctx context.Context, bin string) string {
	if nodesktop.CheckBinary(bin) != nil {
		return ""
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "--ignore-config", "--version")
	setProcAttr(cmd)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// Update brings yt-dlp up to date. Rum's own copy is re-downloaded (and
// re-verified); a copy found elsewhere updates itself with `yt-dlp -U`.
func Update(ctx context.Context, f Found, in Installer) error {
	if f.Source == SourceManaged {
		_, err := in.Install(ctx)
		return err
	}
	if err := nodesktop.CheckBinary(f.Path); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, UpdateTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, f.Path, "--ignore-config", "-U")
	setProcAttr(cmd)
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if isNotFound(err) {
			return missingError()
		}
		return &Error{Code: CodeFailed, Message: "Update failed; use your package manager"}
	}
	return nil
}
