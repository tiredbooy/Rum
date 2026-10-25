package ytdlp

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// intermediate matches yt-dlp's work files for a base name: "<base>.part",
// "<base>.ytdl", per-stream "<base>.f137.mp4" (+ ".part"/".ytdl") and fragment
// files "<base>.f137.mp4.part-Frag3".
var intermediate = regexp.MustCompile(`(\.part(-Frag\d+)?|\.ytdl|\.f(\d|hls|http|dash)[\w-]*\.\w+(\.part(-Frag\d+)?|\.ytdl)?)$`)

// CleanPartials removes yt-dlp's unfinished files for base in dir and returns
// how many it removed. A finished "<base>.mp4" is never touched.
func CleanPartials(dir, base string) int {
	if dir == "" || base == "" {
		return 0
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	n := 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasPrefix(name, base+".") {
			continue
		}
		if !intermediate.MatchString(strings.TrimPrefix(name, base)) {
			continue
		}
		if os.Remove(filepath.Join(dir, name)) == nil {
			n++
		}
	}
	return n
}
