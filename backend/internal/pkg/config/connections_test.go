package config

import "testing"

// TestConnectionsClampAndDefault locks in the connections setting bounds: unset
// or non-positive falls back to the default (8), and the value is clamped to
// [1, maxConnections].
func TestConnectionsClampAndDefault(t *testing.T) {
	cases := []struct {
		in, want int
	}{
		{0, defaultConnections},  // unset -> default 8
		{-5, defaultConnections}, // negative -> default 8
		{1, 1},                   // single stream allowed
		{8, 8},                   // exact default
		{16, maxConnections},     // exact max
		{17, maxConnections},     // over max -> clamped to 16
		{1000, maxConnections},   // way over -> clamped to 16
	}
	for _, tc := range cases {
		s := &Setting{Connections: tc.in, MaxParallel: 1, OutDir: "x", MaxRetries: 0}
		s.Validate()
		if s.Connections != tc.want {
			t.Errorf("Connections in=%d: got %d, want %d", tc.in, s.Connections, tc.want)
		}
	}

	// setDefaults must seed the IDM-style default.
	var d Setting
	d.setDefaults()
	if d.Connections != defaultConnections {
		t.Errorf("setDefaults Connections = %d, want %d", d.Connections, defaultConnections)
	}
}
