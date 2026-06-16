package middlewares

import "testing"

// These cases all previously panicked at startup ("bad origin: origins must
// contain '*' or include http://,https://") because the default origin list and
// the server's Cors([]string{"*"}) call reached gin-contrib/cors with a
// custom-scheme origin (tauri://localhost). Cors must build without panicking.
func TestCorsDoesNotPanic(t *testing.T) {
	cases := map[string][]string{
		"server default call": {"*"},      // what cmd/server/main.go passes
		"nil -> dev defaults":  nil,        // includes tauri://localhost
		"empty slice":          {},         // -> dev defaults
		"explicit http":        {"http://localhost:5173"},
		"custom scheme only":   {"tauri://localhost"},
	}
	for name, origins := range cases {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("Cors(%v) panicked: %v", origins, r)
				}
			}()
			if h := Cors(origins); h == nil {
				t.Fatalf("Cors(%v) returned nil handler", origins)
			}
		})
	}
}
