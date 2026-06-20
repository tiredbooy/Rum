package download

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// A Job with a non-nil error must serialize its Error as a plain JSON string and
// read back as a real error — the old `"error":{}` object form did not round-trip.
func TestJobErrorRoundTrip(t *testing.T) {
	j := &Job{ID: "1", URL: "http://x/y", Status: StatusError, Error: errors.New("boom")}

	data, err := json.Marshal(j)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(data), `"error":"boom"`) {
		t.Fatalf("expected error serialized as a string, got: %s", data)
	}

	var back Job
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if back.GetError() == nil || back.GetError().Error() != "boom" {
		t.Fatalf("error did not round-trip: %v", back.GetError())
	}
	if back.ID != "1" || back.Status != StatusError {
		t.Fatalf("fields lost on round-trip: id=%q status=%q", back.ID, back.Status)
	}
}

// The legacy `"error":{}` object (written by older builds for any non-nil error)
// must load instead of failing the whole decode and wiping the user's history.
func TestJobLegacyErrorObjectLoads(t *testing.T) {
	legacy := []byte(`{"id":"7","url":"http://x","status":"error","error":{}}`)
	var j Job
	if err := json.Unmarshal(legacy, &j); err != nil {
		t.Fatalf("legacy object error must not fail decode: %v", err)
	}
	if j.ID != "7" || j.Status != "error" {
		t.Fatalf("legacy job not loaded: id=%q status=%q", j.ID, j.Status)
	}
	if j.Error != nil {
		t.Fatalf("legacy {} error should decode to nil, got %v", j.Error)
	}

	// An array mixing a legacy errored job with a good one must load BOTH — this is
	// exactly the case that used to make the whole jobs.json fail to parse.
	arr := []byte(`[{"id":"7","status":"error","error":{}},{"id":"8","status":"completed"}]`)
	var jobs []*Job
	if err := json.Unmarshal(arr, &jobs); err != nil {
		t.Fatalf("legacy array must decode: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs recovered, got %d", len(jobs))
	}
}

// null and absent error fields decode to a nil error (no spurious error state).
func TestJobNullErrorDecodes(t *testing.T) {
	for _, in := range []string{
		`{"id":"1","status":"completed","error":null}`,
		`{"id":"1","status":"completed"}`,
	} {
		var j Job
		if err := json.Unmarshal([]byte(in), &j); err != nil {
			t.Fatalf("decode %s: %v", in, err)
		}
		if j.Error != nil {
			t.Fatalf("expected nil error for %s, got %v", in, j.Error)
		}
	}
}
