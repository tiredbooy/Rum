package download

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ResolveDestPath validates a per-download destination directory and returns the
// directory to use, or "" when dest is empty (use the global download folder).
//
// A relative dest is placed under base. Any ".." segment is rejected outright for
// both relative and absolute input, so a request can never climb out of base (or
// smuggle a traversal into an absolute path). An absolute dest is otherwise
// allowed, matching how category destinations behave: the user may pick any
// folder they can write to, but cannot reach it via path tricks. A dest that
// already exists as a regular file is rejected.
func ResolveDestPath(base, dest string) (string, error) {
	dest = strings.TrimSpace(dest)
	if dest == "" {
		return "", nil
	}
	if strings.ContainsRune(dest, 0) {
		return "", destPathError("contains an invalid character")
	}
	for _, seg := range strings.FieldsFunc(dest, func(r rune) bool { return r == '/' || r == '\\' }) {
		if seg == ".." {
			return "", destPathError("must not contain \"..\"")
		}
	}

	var resolved string
	if filepath.IsAbs(dest) {
		resolved = filepath.Clean(dest)
	} else {
		if strings.TrimSpace(base) == "" {
			return "", destPathError("needs a download folder to be relative to")
		}
		resolved = filepath.Join(base, dest)
		if !isWithinDir(base, resolved) {
			return "", destPathError("escapes the download folder")
		}
	}
	if st, err := os.Stat(resolved); err == nil && !st.IsDir() {
		return "", destPathError("is a file, not a folder")
	}
	return resolved, nil
}

func destPathError(why string) error {
	return &CodedError{
		Code: codeValidation,
		Kind: KindValidation,
		Err:  fmt.Errorf("invalid dest_path: %s", why),
	}
}

// applyJobOptions overlays a job's per-download overrides on opt. Unset (zero)
// job fields leave the global values untouched.
func (m *JobManager) applyJobOptions(opt *Options, job *Job) {
	if n := job.GetMaxRetries(); n > 0 {
		opt.MaxRetries = n
	}
	if kb := job.GetSpeedLimit(); kb > 0 {
		opt.SpeedLimit = kb
		opt.JobSpeedLimit = kb
	}
	if d := job.GetDestPath(); d != "" {
		opt.Out = d
	}
	opt.Downloader = jobDownloader(opt, job)
}

// jobDownloader returns the Downloader to use for job: the shared one when the
// job overrides neither User-Agent nor Referer, otherwise a fresh Downloader
// carrying the job's headers (falling back to the shared one's for the other).
func jobDownloader(opt *Options, job *Job) *Downloader {
	ua, ref := job.GetUserAgent(), job.GetReferer()
	if ua == "" && ref == "" {
		return opt.Downloader
	}
	if ua == "" {
		ua = opt.UserAgent
		if opt.Downloader != nil {
			ua = opt.Downloader.Headers["User-Agent"]
		}
	}
	if ref == "" {
		ref = opt.Referer
		if opt.Downloader != nil {
			ref = opt.Downloader.Headers["Referer"]
		}
	}
	return NewDownloader(ua, ref)
}

// rewriteSidecarURL re-keys the resume sidecar next to outputPath to newURL,
// keeping its segment progress and validators, so a resume after a link refresh
// still recognises its own partial data. Reports false when there is no usable
// sidecar.
func rewriteSidecarURL(outputPath, newURL string) bool {
	data, err := os.ReadFile(partsFilePath(outputPath))
	if err != nil {
		return false
	}
	var meta partsMeta
	if json.Unmarshal(data, &meta) != nil || len(meta.Segments) == 0 {
		return false
	}
	meta.URL = newURL
	savePartsMeta(outputPath, &meta)
	return true
}

var urlInText = regexp.MustCompile(`https?://[^\s"']+`)

// redactErr renders err for logs with every URL reduced to scheme://host/path,
// since net/http errors embed the full request URL, query token included.
func redactErr(err error) string {
	if err == nil {
		return ""
	}
	return urlInText.ReplaceAllStringFunc(err.Error(), RedactURL)
}
