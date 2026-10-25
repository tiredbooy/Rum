package updater

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// Repo is the GitHub repository releases are read from.
const Repo = "tiredbooy/Rum"

// ChecksumsAsset is the release file listing "<sha256>  <name>" lines.
const ChecksumsAsset = "SHA256SUMS"

var (
	safeTag   = regexp.MustCompile(`^v?\d+(\.\d+){0,2}(-[0-9A-Za-z.]+)?$`)
	safeAsset = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+-]*$`)
)

// Asset is one downloadable release file.
type Asset struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
	URL  string `json:"url"`
}

// Release is the part of a GitHub release the updater uses.
type Release struct {
	Tag     string  `json:"tag"`
	Version string  `json:"version"`
	PageURL string  `json:"page_url"`
	Assets  []Asset `json:"assets"`
}

// ParseRelease reads a GitHub "latest release" payload. A mirror controls the
// bytes it returns, so no URL from the payload is trusted: the page and asset
// URLs are rebuilt from the validated tag and file names.
func ParseRelease(body []byte) (Release, error) {
	var raw struct {
		Tag        string `json:"tag_name"`
		Draft      bool   `json:"draft"`
		Prerelease bool   `json:"prerelease"`
		Assets     []struct {
			Name string `json:"name"`
			Size int64  `json:"size"`
		} `json:"assets"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return Release{}, fmt.Errorf("parse release: %w", err)
	}
	if !safeTag.MatchString(raw.Tag) {
		return Release{}, errors.New("release has no usable tag")
	}
	if raw.Draft || raw.Prerelease {
		return Release{}, errors.New("release is not final")
	}
	r := Release{
		Tag:     raw.Tag,
		Version: strings.TrimPrefix(raw.Tag, "v"),
		PageURL: "https://github.com/" + Repo + "/releases/tag/" + raw.Tag,
	}
	for _, a := range raw.Assets {
		if !safeAsset.MatchString(a.Name) {
			continue
		}
		r.Assets = append(r.Assets, Asset{
			Name: a.Name,
			Size: a.Size,
			URL:  "https://github.com/" + Repo + "/releases/download/" + raw.Tag + "/" + a.Name,
		})
	}
	return r, nil
}

var (
	osTokens = map[string][]string{
		"linux":   {"linux"},
		"windows": {"windows", "win64", "win32", "win"},
		"darwin":  {"darwin", "macos", "mac", "osx"},
	}
	archTokens = map[string][]string{
		"amd64": {"amd64", "x86_64", "x64"},
		"arm64": {"arm64", "aarch64"},
		"386":   {"386", "i386", "x86"},
	}
	// Preferred installer extension per OS, best first.
	extPrefs = map[string][]string{
		"linux":   {".appimage", ".deb", ".tar.gz"},
		"windows": {"setup.exe", ".exe", ".msi", ".zip"},
		"darwin":  {".dmg", ".zip", ".pkg"},
	}
)

func hasToken(name string, tokens []string) bool {
	for _, t := range tokens {
		for _, part := range strings.FieldsFunc(name, func(r rune) bool {
			return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9')
		}) {
			if part == t {
				return true
			}
		}
	}
	return false
}

// PickAsset chooses the release file built for the given GOOS/GOARCH by name
// (the checksums file is never chosen). It returns false when nothing matches.
func (r Release) PickAsset(goos, goarch string) (Asset, bool) {
	osT, archT := osTokens[goos], archTokens[goarch]
	if osT == nil || archT == nil {
		return Asset{}, false
	}
	best, bestScore := Asset{}, -1
	for _, a := range r.Assets {
		n := strings.ToLower(a.Name)
		if a.Name == ChecksumsAsset || !hasToken(n, osT) || !hasToken(n, archT) {
			continue
		}
		score := 0
		for i, ext := range extPrefs[goos] {
			if strings.HasSuffix(n, ext) {
				score = len(extPrefs[goos]) - i
				break
			}
		}
		if score > bestScore {
			best, bestScore = a, score
		}
	}
	return best, bestScore >= 0
}

// Asset returns the release file with exactly this name.
func (r Release) Asset(name string) (Asset, bool) {
	for _, a := range r.Assets {
		if a.Name == name {
			return a, true
		}
	}
	return Asset{}, false
}
