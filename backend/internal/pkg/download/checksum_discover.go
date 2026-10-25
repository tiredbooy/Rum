package download

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

// Where a checksum came from, and the outcome stored on the job.
const (
	ChecksumSourceUser    = "user"
	ChecksumSourceHeader  = "header"
	ChecksumSourceSidecar = "sidecar"

	ChecksumVerified = "verified"
	ChecksumMismatch = "mismatch"
)

const (
	// discoverFileLimit caps a checksum file; real ones are a few hundred bytes
	// (a SHA256SUMS for a big release is tens of KB).
	discoverFileLimit = 1 << 20
	// discoverTimeout bounds the whole discovery, so a slow host cannot delay
	// the end of a download for long.
	discoverTimeout = 8 * time.Second
)

// Digest is an expected hash of a downloaded file.
type Digest struct {
	Algo   string // "sha256" | "md5"
	Hex    string // lowercase hex
	Source string // ChecksumSource*
}

// algoForHexLen picks the algorithm from a hex digest's length.
func algoForHexLen(n int) string {
	switch n {
	case 32:
		return "md5"
	case 64:
		return "sha256"
	}
	return ""
}

// DetectChecksumAlgo returns the algorithm a hex string looks like, or "".
func DetectChecksumAlgo(h string) string {
	h = strings.TrimSpace(h)
	if _, err := hex.DecodeString(h); err != nil {
		return ""
	}
	return algoForHexLen(len(h))
}

func isHexDigest(s string) bool { return DetectChecksumAlgo(s) != "" }

// ParseChecksumFile reads the common checksum file layouts for the file named
// fileName: a bare digest, "<hex>  <name>" / "<hex> *<name>" lines (sha256sum,
// md5sum), and BSD "SHA256 (<name>) = <hex>" lines. A line naming another file
// is ignored; a nameless digest is taken only when it is the single line.
func ParseChecksumFile(body []byte, fileName string) (Digest, bool) {
	lines := strings.Split(strings.ReplaceAll(string(body), "\r", ""), "\n")
	var nameless []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if h, name, ok := parseSumLine(line); ok {
			if name == "" {
				nameless = append(nameless, h)
			} else if sameFileName(name, fileName) {
				return Digest{Algo: algoForHexLen(len(h)), Hex: strings.ToLower(h), Source: ChecksumSourceSidecar}, true
			}
			continue
		}
		return Digest{}, false // not a checksum file (HTML, JSON, ...)
	}
	if len(nameless) == 1 {
		h := nameless[0]
		return Digest{Algo: algoForHexLen(len(h)), Hex: strings.ToLower(h), Source: ChecksumSourceSidecar}, true
	}
	return Digest{}, false
}

// parseSumLine splits one checksum line into digest and file name ("" if none).
func parseSumLine(line string) (hexDigest, name string, ok bool) {
	// BSD: "SHA256 (name) = hex"
	if i := strings.Index(line, " = "); i > 0 && strings.Contains(line[:i], "(") && strings.HasSuffix(line[:i], ")") {
		left := line[:i]
		open := strings.Index(left, "(")
		h := strings.TrimSpace(line[i+3:])
		if isHexDigest(h) {
			return h, left[open+1 : len(left)-1], true
		}
		return "", "", false
	}
	fields := strings.Fields(line)
	if len(fields) == 0 || !isHexDigest(fields[0]) {
		return "", "", false
	}
	if len(fields) == 1 {
		return fields[0], "", true
	}
	rest := strings.TrimSpace(strings.TrimPrefix(line, fields[0]))
	rest = strings.TrimPrefix(rest, "*")
	return fields[0], rest, true
}

func sameFileName(listed, want string) bool {
	norm := func(s string) string {
		s = strings.TrimPrefix(strings.ReplaceAll(s, "\\", "/"), "./")
		return path.Base(s)
	}
	return norm(listed) == norm(want)
}

// DigestFromHeaders reads a digest the server announced in response headers:
// Digest / Content-Digest / Repr-Digest (sha-256, md5), Content-MD5 and
// x-goog-hash (md5=). SHA-256 wins over MD5.
func DigestFromHeaders(h http.Header) (Digest, bool) {
	var md5Found *Digest
	for _, key := range []string{"Digest", "Content-Digest", "Repr-Digest"} {
		for _, v := range h.Values(key) {
			for _, part := range strings.Split(v, ",") {
				name, val, ok := strings.Cut(strings.TrimSpace(part), "=")
				if !ok {
					continue
				}
				val = strings.Trim(strings.TrimSpace(val), ":") // RFC 9530 byte sequences
				switch strings.ToLower(strings.TrimSpace(name)) {
				case "sha-256", "sha256":
					if d, ok := b64Digest("sha256", val); ok {
						return d, true
					}
				case "md5":
					if d, ok := b64Digest("md5", val); ok && md5Found == nil {
						md5Found = &d
					}
				}
			}
		}
	}
	if v := h.Get("Content-MD5"); v != "" && md5Found == nil {
		if d, ok := b64Digest("md5", strings.TrimSpace(v)); ok {
			md5Found = &d
		}
	}
	for _, v := range h.Values("X-Goog-Hash") {
		for _, part := range strings.Split(v, ",") {
			name, val, ok := strings.Cut(strings.TrimSpace(part), "=")
			if ok && strings.EqualFold(strings.TrimSpace(name), "md5") && md5Found == nil {
				// base64 padding contains '=', which Cut keeps in val.
				if d, ok := b64Digest("md5", strings.TrimSpace(val)); ok {
					md5Found = &d
				}
			}
		}
	}
	if md5Found != nil {
		return *md5Found, true
	}
	return Digest{}, false
}

func b64Digest(algo, b64 string) (Digest, bool) {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return Digest{}, false
	}
	want := map[string]int{"sha256": 32, "md5": 16}[algo]
	if len(raw) != want {
		return Digest{}, false
	}
	return Digest{Algo: algo, Hex: hex.EncodeToString(raw), Source: ChecksumSourceHeader}, true
}

// sidecarURLs lists the checksum files to try beside fileURL, best first. They
// are built from the file's own URL, so they are always on the same host.
func sidecarURLs(fileURL string) []string {
	u, err := url.Parse(fileURL)
	if err != nil || u.Host == "" || u.Path == "" || strings.HasSuffix(u.Path, "/") {
		return nil
	}
	with := func(p string) string {
		c := *u
		c.Path, c.RawPath, c.RawQuery, c.Fragment = p, "", "", ""
		return c.String()
	}
	dir := path.Dir(u.Path)
	if !strings.HasSuffix(dir, "/") {
		dir += "/"
	}
	return []string{
		with(u.Path + ".sha256"),
		with(u.Path + ".sha256sum"),
		with(dir + "SHA256SUMS"),
		with(dir + "sha256sums.txt"),
		with(u.Path + ".md5"),
	}
}

// DiscoverChecksum looks for a published checksum of the file at fileURL: first
// the digest headers of a HEAD, then <url>.sha256, .sha256sum, a SHA256SUMS /
// sha256sums.txt beside it, and .md5. Only the file's own host is contacted
// (redirects to another host are refused), through d's proxy, with small text
// files only. It is best effort: any failure just means "none found".
func DiscoverChecksum(ctx context.Context, d *Downloader, fileURL, fileName string) (Digest, bool) {
	if d == nil || d.Client == nil {
		return Digest{}, false
	}
	u, err := url.Parse(fileURL)
	if err != nil || u.Host == "" {
		return Digest{}, false
	}
	ctx, cancel := context.WithTimeout(ctx, discoverTimeout)
	defer cancel()

	client := *d.Client
	client.Timeout = 0
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if !strings.EqualFold(req.URL.Host, u.Host) {
			return http.ErrUseLastResponse
		}
		return RedirectPolicy(req, via)
	}

	// One request at a time, best candidate first, stopping at the first hit:
	// most hosts publish nothing, and a download must not open a burst of
	// connections just to learn that.
	if dg := headDigest(ctx, d, &client, fileURL); dg != nil {
		return *dg, true
	}
	for _, su := range sidecarURLs(fileURL) {
		if ctx.Err() != nil {
			break
		}
		if dg := fetchChecksumFile(ctx, d, &client, su, fileName); dg != nil {
			return *dg, true
		}
	}
	return Digest{}, false
}

func headDigest(ctx context.Context, d *Downloader, client *http.Client, fileURL string) *Digest {
	req, err := d.NewRequest(http.MethodHead, fileURL)
	if err != nil {
		return nil
	}
	resp, err := client.Do(req.WithContext(ctx))
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil
	}
	if dg, ok := DigestFromHeaders(resp.Header); ok {
		return &dg
	}
	return nil
}

func fetchChecksumFile(ctx context.Context, d *Downloader, client *http.Client, target, fileName string) *Digest {
	req, err := d.NewRequest(http.MethodGet, target)
	if err != nil {
		return nil
	}
	resp, err := client.Do(req.WithContext(ctx))
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	if ct := strings.ToLower(resp.Header.Get("Content-Type")); strings.Contains(ct, "html") {
		return nil
	}
	if resp.ContentLength > discoverFileLimit {
		return nil
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, discoverFileLimit+1))
	if err != nil || len(body) > discoverFileLimit {
		return nil
	}
	dg, ok := ParseChecksumFile(body, fileName)
	if !ok {
		return nil
	}
	return &dg
}
