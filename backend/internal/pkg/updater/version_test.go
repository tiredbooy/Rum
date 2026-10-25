package updater

import "testing"

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.0.0", 0},
		{"v1.2.0", "1.2.0", 0},
		{"1.0.0", "1.0.1", -1},
		{"1.10.0", "1.9.9", 1},
		{"2.0.0", "1.99.99", 1},
		{"1.0", "1.0.0", 0},
		{"1.0.0-rc1", "1.0.0", -1},
		{"1.0.0-rc.2", "1.0.0-rc.10", -1},
		{"1.0.0+build5", "1.0.0", 0},
		{"V3", "2.9.9", 1},
	}
	for _, c := range cases {
		got, ok := CompareVersions(c.a, c.b)
		if !ok || got != c.want {
			t.Errorf("CompareVersions(%q,%q) = %d,%v; want %d", c.a, c.b, got, ok, c.want)
		}
	}
}

func TestCompareVersionsRejectsGarbage(t *testing.T) {
	for _, bad := range []string{"", "latest", "1.x.0", "v", "1.2.3.4", "-1.0.0"} {
		if _, ok := CompareVersions(bad, "1.0.0"); ok {
			t.Errorf("%q should not parse", bad)
		}
	}
}

func TestIsNewer(t *testing.T) {
	if !IsNewer("v1.1.0", "1.0.0") {
		t.Error("1.1.0 should be newer than 1.0.0")
	}
	if IsNewer("1.0.0", "1.0.0") || IsNewer("0.9.0", "1.0.0") || IsNewer("junk", "1.0.0") {
		t.Error("equal, older or unparseable must not be newer")
	}
}
