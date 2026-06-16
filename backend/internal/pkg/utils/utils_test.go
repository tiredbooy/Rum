package utils

import "testing"

func TestConvertSizeToInt(t *testing.T) {
	cases := map[string]int64{
		"":            0,
		"   ":         0,
		"0":           0,
		"12345":       12345,
		" 12345 ":     12345,
		"12345 bytes": 12345,
		"+999":        999,
		"-5":          0,
		"abc":         0,
		"12.5":        12, // leading digits only
		"9999999999":  9999999999,
	}
	for in, want := range cases {
		if got := ConvertSizeToInt(in); got != want {
			t.Errorf("ConvertSizeToInt(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestUrlValidation(t *testing.T) {
	cases := map[string]string{
		"example.com":         "https://example.com",
		"http://example.com":  "http://example.com",
		"https://example.com": "https://example.com",
	}
	for in, want := range cases {
		if got := UrlValidation(in); got != want {
			t.Errorf("UrlValidation(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestGetProgress(t *testing.T) {
	if got := GetProgress(50, 100); got != 50 {
		t.Errorf("GetProgress(50,100) = %d, want 50", got)
	}
	if got := GetProgress(10, 0); got != 0 {
		t.Errorf("GetProgress with zero total should be 0, got %d", got)
	}
	if got := GetProgress(10, -1); got != 0 {
		t.Errorf("GetProgress with negative total should be 0, got %d", got)
	}
}

func TestGetUserAgentForQueueNoPanic(t *testing.T) {
	// Before the modulo fix, most inputs produced an out-of-range index panic.
	for _, id := range []string{"", "a", "queue-123", "ZZZZZZ", "long-queue-id-xyz"} {
		ua := GetUserAgentForQueue(id)
		if ua == "" {
			t.Errorf("empty UA for queue %q", id)
		}
	}
}

func TestGetRandomUserAgent(t *testing.T) {
	if GetRandomUserAgent() == "" {
		t.Error("random UA should not be empty")
	}
}

func TestHexToRGB(t *testing.T) {
	r, g, b := HexToRGB("#FFFFFF")
	if r != 255 || g != 255 || b != 255 {
		t.Errorf("white = %d,%d,%d", r, g, b)
	}
	r, g, b = HexToRGB("000000")
	if r != 0 || g != 0 || b != 0 {
		t.Errorf("black = %d,%d,%d", r, g, b)
	}
	// Bad input returns zeros, no panic.
	if r, g, b = HexToRGB("zzz"); r != 0 || g != 0 || b != 0 {
		t.Errorf("bad hex should be 0,0,0 got %d,%d,%d", r, g, b)
	}
}

func TestIsTerminal(t *testing.T) {
	for _, s := range []string{"completed", "failed", "cancelled"} {
		if !IsTerminal(s) {
			t.Errorf("%q should be terminal", s)
		}
	}
	for _, s := range []string{"downloading", "paused", "queued"} {
		if IsTerminal(s) {
			t.Errorf("%q should not be terminal", s)
		}
	}
}
