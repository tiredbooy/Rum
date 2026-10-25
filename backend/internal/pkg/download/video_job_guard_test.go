package download

import "testing"

// yt-dlp jobs are excluded from the published-checksum logic.
func TestVideoJobsSkipChecksum(t *testing.T) {
	j := &Job{ID: "v", Kind: JobKindVideo}
	var m JobManager
	m.settleChecksum(nil, j, Options{}, nil)
	if st, _, _ := j.GetChecksumResult(); st != "" {
		t.Fatalf("video job got checksum result %q", st)
	}
}
