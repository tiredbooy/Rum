package ytdlp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/tiredbooy/Rum/backend/nodesktop"
	"net/url"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// InfoTimeout bounds one `yt-dlp -J` run.
const InfoTimeout = 60 * time.Second

const maxInfoBytes = 32 << 20

// Format is the part of a yt-dlp format entry the picker needs.
type Format struct {
	ID             string  `json:"format_id"`
	Ext            string  `json:"ext"`
	Height         int     `json:"height"`
	VCodec         string  `json:"vcodec"`
	ACodec         string  `json:"acodec"`
	Filesize       float64 `json:"filesize"`
	FilesizeApprox float64 `json:"filesize_approx"`
	TBR            float64 `json:"tbr"`
}

func (f Format) hasVideo() bool { return f.Height > 0 && f.VCodec != "none" }

// hasAudio is true unless the extractor says there is none. An unreported
// codec on a video format is treated as "has audio" (typical for direct mp4).
func (f Format) hasAudio() bool { return f.ACodec != "none" }

func (f Format) audioOnly() bool { return !f.hasVideo() && f.ACodec != "" && f.ACodec != "none" }

func (f Format) size() int64 {
	if f.Filesize > 0 {
		return int64(f.Filesize)
	}
	return int64(f.FilesizeApprox)
}

// Info describes one page's video.
type Info struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Thumbnail string   `json:"thumbnail"`
	Uploader  string   `json:"uploader"`
	Extractor string   `json:"extractor_key"`
	Duration  float64  `json:"duration"`
	Formats   []Format `json:"formats"`
	Entries   []Info   `json:"entries"`
}

// ValidateURL accepts only absolute http(s) URLs with a host.
func ValidateURL(raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return errors.New("link must start with http:// or https://")
	}
	return nil
}

// ParseInfo decodes `yt-dlp -J` output. A playlist-shaped answer yields its
// first entry (the run used --no-playlist, so it is the requested video).
func ParseInfo(data []byte) (*Info, error) {
	var in Info
	if err := json.Unmarshal(data, &in); err != nil {
		return nil, &Error{Code: CodeFailed, Message: "Couldn't read video details"}
	}
	if len(in.Formats) == 0 && len(in.Entries) > 0 {
		first := in.Entries[0]
		if in.Title != "" && first.Title == "" {
			first.Title = in.Title
		}
		in = first
	}
	if len(in.Formats) == 0 {
		return nil, &Error{Code: CodeVideoUnavailable, Message: "No video found at this link"}
	}
	in.Title = strings.TrimSpace(in.Title)
	if in.Title == "" {
		in.Title = "video"
	}
	return &in, nil
}

// InfoArgs is the argument list for listing formats. The URL follows `--` so it
// can never be read as an option; --ignore-config keeps user config out.
func InfoArgs(rawURL, proxy string) []string {
	args := []string{"--ignore-config", "-J", "--no-playlist"}
	if proxy != "" {
		args = append(args, "--proxy", proxy)
	}
	return append(args, "--", rawURL)
}

// FetchInfo runs yt-dlp -J for a page URL.
func FetchInfo(ctx context.Context, bin, rawURL, proxy string) (*Info, error) {
	if err := ValidateURL(rawURL); err != nil {
		return nil, &Error{Code: CodeVideoUnavailable, Message: "Link must start with http or https"}
	}
	if err := nodesktop.CheckBinary(bin); err != nil {
		return nil, &Error{Code: CodeFailed, Message: "Couldn't start yt-dlp"}
	}
	ctx, cancel := context.WithTimeout(ctx, InfoTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, InfoArgs(strings.TrimSpace(rawURL), proxy)...)
	setProcAttr(cmd)
	var out bytes.Buffer
	cmd.Stdout = &limitWriter{w: &out, n: maxInfoBytes}
	errb := &tailBuffer{max: 8192}
	cmd.Stderr = errb
	if err := cmd.Run(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, &Error{Code: "timeout", Message: "Server took too long", Retryable: true}
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if isNotFound(err) {
			return nil, missingError()
		}
		return nil, ClassifyStderr(errb.String())
	}
	return ParseInfo(out.Bytes())
}

// Option is one entry of the format picker.
type Option struct {
	Key     string `json:"key"`   // best | h1080 | h720 | h480 | audio
	Label   string `json:"label"` // "Best (1080p)", "720p", "Audio only"
	Height  int    `json:"height,omitempty"`
	EstSize int64  `json:"est_size,omitempty"`
}

var keyRe = regexp.MustCompile(`^(best|audio|h[1-9]\d{2,3})$`)

// ValidKey reports whether key is a picker key this package can build.
func ValidKey(key string) bool { return keyRe.MatchString(key) }

func keyHeight(key string) int {
	if !strings.HasPrefix(key, "h") {
		return 0
	}
	n, _ := strconv.Atoi(key[1:])
	return n
}

// Selector builds the yt-dlp -f expression for a picker key. Without ffmpeg only
// single-file formats can be fetched, since merging needs it.
func Selector(key string, ffmpeg bool) string {
	switch {
	case key == "audio":
		return "ba/b"
	case key == "best" && ffmpeg:
		return "bv*+ba/b"
	case key == "best":
		return "b"
	case ffmpeg:
		h := keyHeight(key)
		return fmt.Sprintf("bv*[height<=%d]+ba/b[height<=%d]", h, h)
	default:
		return fmt.Sprintf("b[height<=%d]", keyHeight(key))
	}
}

var tiers = []int{1080, 720, 480}

// videoFormats are the video formats Rum can fetch in this setup.
func videoFormats(fs []Format, ffmpeg bool) []Format {
	var out []Format
	for _, f := range fs {
		if f.hasVideo() && (ffmpeg || f.hasAudio()) {
			out = append(out, f)
		}
	}
	return out
}

// Options derives the picker choices from what the page really offers.
func Options(info *Info, ffmpeg bool) []Option {
	if info == nil {
		return nil
	}
	vids := videoFormats(info.Formats, ffmpeg)
	maxH := 0
	minH := 0
	for _, f := range vids {
		if f.Height > maxH {
			maxH = f.Height
		}
		if minH == 0 || f.Height < minH {
			minH = f.Height
		}
	}
	best := Option{Key: "best", Label: "Best", Height: maxH}
	if maxH > 0 {
		best.Label = fmt.Sprintf("Best (%dp)", maxH)
	}
	opts := []Option{best}
	for _, t := range tiers {
		if t < maxH && minH <= t {
			opts = append(opts, Option{Key: "h" + strconv.Itoa(t), Label: strconv.Itoa(t) + "p", Height: t})
		}
	}
	for _, f := range info.Formats {
		if f.audioOnly() {
			opts = append(opts, Option{Key: "audio", Label: "Audio only"})
			break
		}
	}
	for i := range opts {
		opts[i].EstSize = EstimateSize(info, opts[i].Key, ffmpeg)
	}
	return opts
}

// EstimateSize is a best-effort byte size for a key; 0 when unknown.
func EstimateSize(info *Info, key string, ffmpeg bool) int64 {
	if info == nil || !ValidKey(key) {
		return 0
	}
	if key == "audio" {
		return bestAudio(info.Formats).size()
	}
	vids := videoFormats(info.Formats, ffmpeg)
	limit := keyHeight(key)
	var pick *Format
	for i := range vids {
		f := &vids[i]
		if limit > 0 && f.Height > limit {
			continue
		}
		if pick == nil || f.Height > pick.Height || (f.Height == pick.Height && f.TBR > pick.TBR) {
			pick = f
		}
	}
	if pick == nil {
		return 0
	}
	total := pick.size()
	if ffmpeg && pick.ACodec == "none" {
		total += bestAudio(info.Formats).size()
	}
	return total
}

func bestAudio(fs []Format) Format {
	var a []Format
	for _, f := range fs {
		if f.audioOnly() {
			a = append(a, f)
		}
	}
	if len(a) == 0 {
		return Format{}
	}
	sort.SliceStable(a, func(i, j int) bool { return a[i].TBR > a[j].TBR })
	return a[0]
}
