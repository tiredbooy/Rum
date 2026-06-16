package download

import "testing"

func TestValidateResume(t *testing.T) {
	const (
		etagA = `"abc123"`
		etagB = `"def456"`
		lmA   = "Wed, 21 Oct 2024 07:28:00 GMT"
		lmB   = "Thu, 22 Oct 2024 07:28:00 GMT"
	)

	tests := []struct {
		name                                     string
		remoteETag, remoteLM, savedETag, savedLM string
		remoteSize, savedSize                    int64
		want                                     bool
	}{
		{
			name:       "etag match, sizes match -> resume",
			remoteETag: etagA, savedETag: etagA,
			remoteSize: 1000, savedSize: 1000,
			want: true,
		},
		{
			name:       "etag mismatch -> no resume",
			remoteETag: etagA, savedETag: etagB,
			remoteSize: 1000, savedSize: 1000,
			want: false,
		},
		{
			name:       "weak etag equals strong etag (normalized)",
			remoteETag: `W/"abc123"`, savedETag: `"abc123"`,
			remoteSize: 1000, savedSize: 1000,
			want: true,
		},
		{
			name:       "etag match but size changed -> no resume",
			remoteETag: etagA, savedETag: etagA,
			remoteSize: 2000, savedSize: 1000,
			want: false,
		},
		{
			name:     "last-modified match, sizes match -> resume",
			remoteLM: lmA, savedLM: lmA,
			remoteSize: 1000, savedSize: 1000,
			want: true,
		},
		{
			name:     "last-modified mismatch -> no resume",
			remoteLM: lmA, savedLM: lmB,
			remoteSize: 1000, savedSize: 1000,
			want: false,
		},
		{
			name:       "etag preferred over last-modified: etag matches, LM differs -> resume",
			remoteETag: etagA, savedETag: etagA, remoteLM: lmA, savedLM: lmB,
			remoteSize: 1000, savedSize: 1000,
			want: true,
		},
		{
			name:       "etag present only remote, LM matches -> resume on LM",
			remoteETag: etagA, savedETag: "", remoteLM: lmA, savedLM: lmA,
			remoteSize: 1000, savedSize: 1000,
			want: true,
		},
		{
			name:       "no shared validator, sizes match -> resume",
			remoteSize: 1000, savedSize: 1000,
			want: true,
		},
		{
			name:       "no shared validator, sizes differ -> no resume",
			remoteSize: 1000, savedSize: 900,
			want: false,
		},
		{
			name:       "no validator, remote size unknown -> no resume (conservative)",
			remoteSize: -1, savedSize: 1000,
			want: false,
		},
		{
			name:       "no validator, saved size unknown -> no resume",
			remoteSize: 1000, savedSize: 0,
			want: false,
		},
		{
			name:       "etag match, remote size unknown -> resume (validator trusted)",
			remoteETag: etagA, savedETag: etagA,
			remoteSize: -1, savedSize: 1000,
			want: true,
		},
		{
			name:       "etag only on saved side, no LM, sizes match -> fall back to size match",
			savedETag:  etagA,
			remoteSize: 1000, savedSize: 1000,
			want: true,
		},
		{
			name:       "size mismatch dominates even with matching etag absent",
			remoteSize: 500, savedSize: 1000,
			want: false,
		},
		{
			name: "all empty / zero -> no resume",
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ValidateResume(tc.remoteETag, tc.remoteLM, tc.savedETag, tc.savedLM, tc.remoteSize, tc.savedSize)
			if got != tc.want {
				t.Errorf("ValidateResume() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestNormalizeValidator(t *testing.T) {
	tests := []struct{ in, want string }{
		{`"abc"`, "abc"},
		{`W/"abc"`, "abc"},
		{`w/"abc"`, "abc"},
		{`  "abc"  `, "abc"},
		{`abc`, "abc"},
		{``, ""},
		{`""`, ""},
	}
	for _, tc := range tests {
		if got := normalizeValidator(tc.in); got != tc.want {
			t.Errorf("normalizeValidator(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
