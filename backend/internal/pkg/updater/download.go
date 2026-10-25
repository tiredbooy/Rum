package updater

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// maxAssetBytes caps an update download so a hostile mirror cannot fill the disk.
const maxAssetBytes = 1 << 30

// DownloadVerified saves the release file built for goos/goarch into dir and
// checks it against the release's SHA256SUMS. A file that cannot be verified is
// deleted, never kept. It does not install or run anything.
func (c *Checker) DownloadVerified(ctx context.Context, rel Release, goos, goarch, dir string) (string, error) {
	asset, ok := rel.PickAsset(goos, goarch)
	if !ok {
		return "", ErrNoAsset
	}
	sumsAsset, ok := rel.Asset(ChecksumsAsset)
	if !ok {
		return "", ErrNoChecksum
	}
	sums, err := c.fetchFirst(ctx, sumsAsset.URL, maxSumsBytes)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrUnreachable, err)
	}
	want, ok := ParseSums(sums)[asset.Name]
	if !ok {
		return "", ErrNoChecksum
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	part, err := os.CreateTemp(dir, asset.Name+".*.part")
	if err != nil {
		return "", err
	}
	partPath := part.Name()
	defer os.Remove(partPath) // no-op once renamed
	err = c.fetchInto(ctx, asset.URL, part)
	if cerr := part.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrUnreachable, err)
	}
	if err := VerifyFile(partPath, want); err != nil {
		return "", err
	}
	final := filepath.Join(dir, asset.Name)
	if err := os.Rename(partPath, final); err != nil {
		return "", err
	}
	return final, nil
}

func (c *Checker) fetchFirst(ctx context.Context, primary string, limit int64) ([]byte, error) {
	var last error
	for _, src := range sourceURLs(primary, c.Mirrors) {
		body, err := c.get(ctx, src, limit)
		if err == nil {
			return body, nil
		}
		last = err
		if ctx.Err() != nil {
			break
		}
	}
	return nil, last
}

// fetchInto streams the first source that answers 200 into w. The per-source
// timeout only covers getting a response; the body may take as long as ctx allows.
func (c *Checker) fetchInto(ctx context.Context, primary string, w *os.File) error {
	var last error
	for _, src := range sourceURLs(primary, c.Mirrors) {
		if err := w.Truncate(0); err != nil {
			return err
		}
		if _, err := w.Seek(0, io.SeekStart); err != nil {
			return err
		}
		last = c.streamOne(ctx, src, w)
		if last == nil {
			return nil
		}
		if ctx.Err() != nil {
			break
		}
	}
	return last
}

func (c *Checker) streamOne(ctx context.Context, src string, w io.Writer) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	timer := time.AfterFunc(c.timeout(), cancel)
	defer timer.Stop()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, src, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Rum-update-check")
	resp, err := c.client().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	timer.Stop() // headers arrived; let the body take its time
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	n, err := io.Copy(w, io.LimitReader(resp.Body, maxAssetBytes+1))
	if err != nil {
		return err
	}
	if n > maxAssetBytes {
		return errors.New("download too large")
	}
	return nil
}
