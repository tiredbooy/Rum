package format

import (
	"strings"
	"testing"
)

func TestCleanFileName(t *testing.T) {
	cases := map[string]string{
		"https://x.com/a/b/file.zip":           "file.zip",
		"https://x.com/a/b/file.zip?token=123": "file.zip",
		"https://x.com/file%20name.pdf":        "file name.pdf",
		"https://x.com/../../etc/passwd":       "passwd",
		"https://x.com/":                       "",
		"https://x.com":                        "",
		`https://x.com/a\b\evil.exe`:           "evil.exe",
		"https://x.com/bad:name*.txt":          "badname.txt",
	}
	for in, want := range cases {
		if got := CleanFileName(in); got != want {
			t.Errorf("CleanFileName(%q) = %q, want %q", in, got, want)
		}
	}
	// Result must never contain a separator.
	for _, in := range []string{"https://x/../../y", `http://x/a\b\c`} {
		if strings.ContainsAny(CleanFileName(in), `/\`) {
			t.Errorf("CleanFileName(%q) leaked a separator", in)
		}
	}
}

func TestExtractFileNameFromURL(t *testing.T) {
	cases := map[string]string{
		"https://x.com/d?filename=report.pdf":          "report.pdf",
		"https://x.com/d?filename=my%20file.zip":       "my file.zip",
		"https://x.com/d?filename=../../etc/passwd":     "passwd",
		"https://x.com/d":                              "",
		"://bad-url":                                   "",
	}
	for in, want := range cases {
		if got := ExtractFileNameFromURL(in); got != want {
			t.Errorf("ExtractFileNameFromURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestGetFolderName(t *testing.T) {
	cases := map[string]string{
		"video/mp4":                "videos",
		"audio/mpeg":               "audios",
		"image/png":                "images",
		"application/pdf":          "documents",
		"application/zip":          "compressed",
		"application/octet-stream": "others",
		"text/html; charset=utf-8": "code",
	}
	for in, want := range cases {
		if got := GetFolderName(in); got != want {
			t.Errorf("GetFolderName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFormatSize(t *testing.T) {
	if got := FormatSize(0); got != "0 B" {
		t.Errorf("FormatSize(0) = %q", got)
	}
	if got := FormatSize(1024); got != "1.00 KB" {
		t.Errorf("FormatSize(1024) = %q", got)
	}
	if got := FormatSize(1024 * 1024); got != "1.00 MB" {
		t.Errorf("FormatSize(1MB) = %q", got)
	}
}
