package updater

import "testing"

const sampleRelease = `{
  "tag_name": "v1.2.0",
  "name": "Rum 1.2.0",
  "html_url": "https://evil.example/phish",
  "prerelease": false,
  "draft": false,
  "assets": [
    {"name": "Rum-1.2.0-linux-amd64.AppImage", "size": 100, "browser_download_url": "https://evil.example/a"},
    {"name": "Rum-1.2.0-windows-amd64-setup.exe", "size": 200, "browser_download_url": "https://evil.example/b"},
    {"name": "Rum-1.2.0-darwin-arm64.dmg", "size": 300, "browser_download_url": "https://evil.example/c"},
    {"name": "SHA256SUMS", "size": 10, "browser_download_url": "https://evil.example/d"},
    {"name": "../escape", "size": 1, "browser_download_url": "https://evil.example/e"}
  ]
}`

func TestParseReleaseUsesCanonicalURLs(t *testing.T) {
	r, err := ParseRelease([]byte(sampleRelease))
	if err != nil {
		t.Fatal(err)
	}
	if r.Tag != "v1.2.0" || r.Version != "1.2.0" {
		t.Errorf("tag/version = %q/%q", r.Tag, r.Version)
	}
	// A mirror controls the payload, so URLs must be rebuilt from the tag.
	if want := "https://github.com/tiredbooy/Rum/releases/tag/v1.2.0"; r.PageURL != want {
		t.Errorf("PageURL = %q, want %q", r.PageURL, want)
	}
	if len(r.Assets) != 4 {
		t.Fatalf("assets = %d, want 4 (unsafe name dropped)", len(r.Assets))
	}
	if want := "https://github.com/tiredbooy/Rum/releases/download/v1.2.0/SHA256SUMS"; r.Assets[3].URL != want {
		t.Errorf("asset URL = %q, want %q", r.Assets[3].URL, want)
	}
}

func TestParseReleaseRejects(t *testing.T) {
	for name, body := range map[string]string{
		"empty":      ``,
		"not json":   `<html>`,
		"no tag":     `{"assets":[]}`,
		"bad tag":    `{"tag_name":"latest"}`,
		"evil tag":   `{"tag_name":"v1.0.0/../../x"}`,
		"draft":      `{"tag_name":"v1.0.0","draft":true}`,
		"prerelease": `{"tag_name":"v1.0.0","prerelease":true}`,
	} {
		if _, err := ParseRelease([]byte(body)); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
}

func TestPickAsset(t *testing.T) {
	r, _ := ParseRelease([]byte(sampleRelease))
	cases := []struct{ goos, arch, want string }{
		{"linux", "amd64", "Rum-1.2.0-linux-amd64.AppImage"},
		{"windows", "amd64", "Rum-1.2.0-windows-amd64-setup.exe"},
		{"darwin", "arm64", "Rum-1.2.0-darwin-arm64.dmg"},
	}
	for _, c := range cases {
		a, ok := r.PickAsset(c.goos, c.arch)
		if !ok || a.Name != c.want {
			t.Errorf("%s/%s = %q,%v; want %q", c.goos, c.arch, a.Name, ok, c.want)
		}
	}
	if _, ok := r.PickAsset("darwin", "amd64"); ok {
		t.Error("wrong arch must not match")
	}
	if _, ok := r.PickAsset("plan9", "amd64"); ok {
		t.Error("unknown OS must not match")
	}
}
