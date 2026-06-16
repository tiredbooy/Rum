package download

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
)

func TestClassify(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantCode   string
		wantStatus int
	}{
		{"nil", nil, "", http.StatusOK},
		{"unknown", errors.New("boom"), "internal_error", http.StatusInternalServerError},

		{"not_found", ErrNotFound, "not_found", http.StatusNotFound},
		{"invalid_url", ErrInvalidURL, "validation_error", http.StatusBadRequest},
		{"unsupported_scheme", ErrUnsupportedScheme, "unsupported", http.StatusUnsupportedMediaType},
		{"disk_space", ErrInsufficientDiskSpace, "insufficient_disk_space", http.StatusInsufficientStorage},
		{"checksum", ErrChecksumMismatch, "checksum_mismatch", http.StatusUnprocessableEntity},
		{"conflict", ErrConflict, "conflict", http.StatusConflict},
		{"too_large", ErrTooLarge, "payload_too_large", http.StatusRequestEntityTooLarge},
		{"remote_changed", ErrRemoteChanged, "remote_changed", http.StatusConflict},

		// Wrapped sentinels must still classify via errors.Is.
		{"wrapped_not_found", fmt.Errorf("get job: %w", ErrNotFound), "not_found", http.StatusNotFound},
		{"wrapped_checksum", fmt.Errorf("verify: %w", ErrChecksumMismatch), "checksum_mismatch", http.StatusUnprocessableEntity},

		// CodedError directly.
		{"coded_conflict", NewCodedError(KindConflict, "conflict", ErrConflict), "conflict", http.StatusConflict},
		{"coded_disk", NewCodedError(KindDiskSpace, "insufficient_disk_space", nil), "insufficient_disk_space", http.StatusInsufficientStorage},

		// CodedError wrapped in another error: errors.As should still find it.
		{"wrapped_coded", fmt.Errorf("ctx: %w", NewCodedError(KindNotFound, "not_found", ErrNotFound)), "not_found", http.StatusNotFound},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			code, status := Classify(tc.err)
			if code != tc.wantCode {
				t.Errorf("Classify(%v) code = %q, want %q", tc.err, code, tc.wantCode)
			}
			if status != tc.wantStatus {
				t.Errorf("Classify(%v) status = %d, want %d", tc.err, status, tc.wantStatus)
			}
		})
	}
}

func TestWrap(t *testing.T) {
	err := Wrap(KindValidation, "bad field %q", "url")
	code, status := Classify(err)
	if code != "validation_error" || status != http.StatusBadRequest {
		t.Fatalf("Wrap classify = (%q,%d), want (validation_error,400)", code, status)
	}
	if got := err.Error(); got != `bad field "url"` {
		t.Errorf("Wrap message = %q, want %q", got, `bad field "url"`)
	}
}

func TestWrapDefaultKind(t *testing.T) {
	// An unnamed/zero kind maps to internal_error.
	err := Wrap(KindInternal, "kaboom")
	code, status := Classify(err)
	if code != "internal_error" || status != http.StatusInternalServerError {
		t.Fatalf("Wrap(KindInternal) classify = (%q,%d), want (internal_error,500)", code, status)
	}
}

func TestCodedErrorUnwrap(t *testing.T) {
	ce := NewCodedError(KindNotFound, "not_found", ErrNotFound)
	if !errors.Is(ce, ErrNotFound) {
		t.Error("errors.Is(CodedError, ErrNotFound) = false, want true")
	}
	if ce.Unwrap() != ErrNotFound {
		t.Error("Unwrap did not return the wrapped error")
	}
	if ce.Error() != ErrNotFound.Error() {
		t.Errorf("Error() = %q, want %q", ce.Error(), ErrNotFound.Error())
	}
}

func TestCodedErrorNilSafe(t *testing.T) {
	var ce *CodedError
	if got := ce.Error(); got != "<nil>" {
		t.Errorf("nil CodedError Error() = %q, want <nil>", got)
	}
	if ce.Unwrap() != nil {
		t.Error("nil CodedError Unwrap() should be nil")
	}
}
