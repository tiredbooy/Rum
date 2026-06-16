package download

import "strings"

// normalizeValidator trims whitespace and surrounding quotes from an HTTP
// validator (ETag) so a weak/quoted value compares equal to its bare form.
// ETags are returned quoted (e.g. `"abc123"`) and may carry a weak prefix
// (`W/"abc123"`); we strip both for a forgiving-but-correct comparison.
func normalizeValidator(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "W/")
	v = strings.TrimPrefix(v, "w/")
	v = strings.Trim(v, `"`)
	return strings.TrimSpace(v)
}

// ValidateResume reports whether a previously-saved partial download is still
// safe to resume against the current remote resource. It is intentionally
// conservative: when the available evidence is ambiguous it returns false so the
// caller restarts cleanly rather than risking a corrupt file stitched from two
// different resource versions.
//
// Inputs are the remote and saved HTTP validators plus sizes:
//   - remoteETag / savedETag: the current and stored ETag headers.
//   - remoteLastMod / savedLastMod: the current and stored Last-Modified headers.
//   - remoteSize / savedSize: the current total size and the size recorded for
//     the saved partial.
//
// Decision rules (a partial is resumable only if all that apply hold):
//  1. Size consistency. If both sizes are known (> 0) they must be equal. A
//     changed total size means the resource changed; do not resume.
//  2. Validator consistency. If a validator is present on BOTH sides it must
//     match: ETag is preferred; otherwise Last-Modified. A present-but-changed
//     validator means the resource changed; do not resume.
//  3. Fallback. If no validator is available on both sides, fall back to the
//     size check alone — and require sizes to be known and equal, since with no
//     validator a matching size is the only evidence we have.
func ValidateResume(remoteETag, remoteLastMod, savedETag, savedLastMod string, remoteSize, savedSize int64) bool {
	// Rule 1: size consistency when both are known.
	bothSizesKnown := remoteSize > 0 && savedSize > 0
	if bothSizesKnown && remoteSize != savedSize {
		return false
	}

	rETag := normalizeValidator(remoteETag)
	sETag := normalizeValidator(savedETag)
	rLM := strings.TrimSpace(remoteLastMod)
	sLM := strings.TrimSpace(savedLastMod)

	// Rule 2a: ETag present on both sides -> must match.
	if rETag != "" && sETag != "" {
		if rETag != sETag {
			return false
		}
		// ETag matches; still require size agreement if both known (already
		// checked above). Safe to resume.
		return true
	}

	// Rule 2b: Last-Modified present on both sides -> must match.
	if rLM != "" && sLM != "" {
		if rLM != sLM {
			return false
		}
		return true
	}

	// Rule 3: no shared validator. Resume only on a confident size match.
	// Requiring both sizes to be known avoids resuming on an unknown remote
	// size, where we cannot prove the partial still corresponds to the resource.
	return bothSizesKnown && remoteSize == savedSize
}
