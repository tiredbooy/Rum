package dto

import "testing"

func TestCreateDownloadRequestValidate(t *testing.T) {
	tests := []struct {
		name      string
		req       CreateDownloadRequest
		wantField string // a field key expected in the result, "" means valid
	}{
		{
			name: "valid http url",
			req:  CreateDownloadRequest{URLs: []string{"https://example.com/file.zip"}},
		},
		{
			name:      "empty urls",
			req:       CreateDownloadRequest{URLs: []string{}},
			wantField: "urls",
		},
		{
			name:      "blank url entry",
			req:       CreateDownloadRequest{URLs: []string{"   "}},
			wantField: "urls[0]",
		},
		{
			name:      "relative url",
			req:       CreateDownloadRequest{URLs: []string{"/just/a/path"}},
			wantField: "urls[0]",
		},
		{
			name:      "bad scheme",
			req:       CreateDownloadRequest{URLs: []string{"ftp://example.com/x"}},
			wantField: "urls[0]",
		},
		{
			name:      "negative speed limit",
			req:       CreateDownloadRequest{URLs: []string{"https://a.com/x"}, SpeedLimit: -1},
			wantField: "speed_limit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.req.Validate()
			if tt.wantField == "" {
				if got != nil {
					t.Fatalf("expected valid, got %v", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("expected error for field %q, got nil", tt.wantField)
			}
			if _, ok := got[tt.wantField]; !ok {
				t.Fatalf("expected field %q in %v", tt.wantField, got)
			}
		})
	}
}

func TestSpeedLimitRequestValidate(t *testing.T) {
	neg := -5
	zero := 0
	if got := (&SpeedLimitRequest{}).Validate(); got == nil {
		t.Fatal("expected error when speed_limit_kb is nil")
	}
	if got := (&SpeedLimitRequest{SpeedLimitKB: &neg}).Validate(); got == nil {
		t.Fatal("expected error for negative speed limit")
	}
	if got := (&SpeedLimitRequest{SpeedLimitKB: &zero}).Validate(); got != nil {
		t.Fatalf("expected zero to be valid, got %v", got)
	}
}

func TestClampLimit(t *testing.T) {
	cases := map[int]int{
		0:               DefaultPageSize,
		-10:             DefaultPageSize,
		10:              10,
		MaxPageSize + 1: MaxPageSize,
	}
	for in, want := range cases {
		if got := ClampLimit(in); got != want {
			t.Errorf("ClampLimit(%d) = %d, want %d", in, got, want)
		}
	}
}
