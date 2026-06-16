package download

import (
	"errors"
	"fmt"
	"net/http"
)

// Sentinel errors for the engine error taxonomy. These are the canonical
// definitions referenced across the download package (and wrapped with %w by
// callers). Classify maps each to a stable machine code + HTTP status.
var (
	ErrNotFound              = errors.New("download not found")
	ErrInvalidURL            = errors.New("invalid url")
	ErrUnsupportedScheme     = errors.New("unsupported url scheme")
	ErrInsufficientDiskSpace = errors.New("insufficient disk space")
	ErrChecksumMismatch      = errors.New("checksum mismatch")
	ErrConflict              = errors.New("conflict")
	ErrTooLarge              = errors.New("payload too large")
	ErrRemoteChanged         = errors.New("remote resource changed")
)

// ErrKind is a coarse classification of an engine error. It groups errors by
// the kind of failure so callers can branch without matching every sentinel.
type ErrKind int

const (
	// KindInternal is the zero value: an unclassified/internal failure.
	KindInternal ErrKind = iota
	KindNotFound
	KindValidation
	KindConflict
	KindUnsupported
	KindTooLarge
	KindDiskSpace
	KindChecksum
	KindRemoteChanged
)

// Stable machine codes. Values reused from the dto package MUST match its
// constants exactly (validation_error, not_found, conflict, internal_error,
// unsupported, payload_too_large). The remaining three are new B3 codes.
const (
	codeValidation   = "validation_error"
	codeNotFound     = "not_found"
	codeConflict     = "conflict"
	codeInternal     = "internal_error"
	codeUnsupported  = "unsupported"
	codePayloadLarge = "payload_too_large"
	codeDiskSpace    = "insufficient_disk_space"
	codeChecksum     = "checksum_mismatch"
	codeRemoteChange = "remote_changed"
)

// CodedError carries a stable machine code and an ErrKind alongside an
// underlying error. It implements error and unwraps to Err so errors.Is /
// errors.As keep working through it.
type CodedError struct {
	Code string
	Kind ErrKind
	Err  error
}

// Error implements the error interface.
func (e *CodedError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Err == nil {
		if e.Code != "" {
			return e.Code
		}
		return "coded error"
	}
	return e.Err.Error()
}

// Unwrap exposes the wrapped error for errors.Is / errors.As.
func (e *CodedError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// NewCodedError builds a *CodedError with the given kind, code and wrapped err.
func NewCodedError(kind ErrKind, code string, err error) *CodedError {
	return &CodedError{Code: code, Kind: kind, Err: err}
}

// Wrap builds a CodedError whose code is derived from kind and whose message is
// formatted from format/args. The resulting error unwraps to the formatted
// error, so errors.Is against a sentinel embedded via %w still works.
func Wrap(kind ErrKind, format string, args ...any) error {
	return &CodedError{
		Code: codeForKind(kind),
		Kind: kind,
		Err:  fmt.Errorf(format, args...),
	}
}

// codeForKind returns the canonical machine code for an ErrKind.
func codeForKind(kind ErrKind) string {
	switch kind {
	case KindNotFound:
		return codeNotFound
	case KindValidation:
		return codeValidation
	case KindConflict:
		return codeConflict
	case KindUnsupported:
		return codeUnsupported
	case KindTooLarge:
		return codePayloadLarge
	case KindDiskSpace:
		return codeDiskSpace
	case KindChecksum:
		return codeChecksum
	case KindRemoteChanged:
		return codeRemoteChange
	default:
		return codeInternal
	}
}

// statusForCode maps a stable machine code to its HTTP status.
func statusForCode(code string) int {
	switch code {
	case codeValidation:
		return http.StatusBadRequest // 400
	case codeNotFound:
		return http.StatusNotFound // 404
	case codeConflict, codeRemoteChange:
		return http.StatusConflict // 409
	case codeUnsupported:
		return http.StatusUnsupportedMediaType // 415
	case codePayloadLarge:
		return http.StatusRequestEntityTooLarge // 413
	case codeDiskSpace:
		return http.StatusInsufficientStorage // 507
	case codeChecksum:
		return http.StatusUnprocessableEntity // 422
	case codeInternal:
		return http.StatusInternalServerError // 500
	default:
		return http.StatusInternalServerError // 500
	}
}

// Classify maps any engine error to a stable machine code and HTTP status.
//
//	nil          -> ("", 200)
//	*CodedError  -> its Code (and the status for that code)
//	a sentinel   -> the matching code/status
//	anything else-> (internal_error, 500)
func Classify(err error) (code string, httpStatus int) {
	if err == nil {
		return "", http.StatusOK // 200
	}

	// Prefer an explicit CodedError code if one is present in the chain.
	var ce *CodedError
	if errors.As(err, &ce) && ce != nil && ce.Code != "" {
		return ce.Code, statusForCode(ce.Code)
	}

	switch {
	case errors.Is(err, ErrNotFound):
		return codeNotFound, statusForCode(codeNotFound)
	case errors.Is(err, ErrInvalidURL):
		return codeValidation, statusForCode(codeValidation)
	case errors.Is(err, ErrUnsupportedScheme):
		return codeUnsupported, statusForCode(codeUnsupported)
	case errors.Is(err, ErrInsufficientDiskSpace):
		return codeDiskSpace, statusForCode(codeDiskSpace)
	case errors.Is(err, ErrChecksumMismatch):
		return codeChecksum, statusForCode(codeChecksum)
	case errors.Is(err, ErrConflict):
		return codeConflict, statusForCode(codeConflict)
	case errors.Is(err, ErrTooLarge):
		return codePayloadLarge, statusForCode(codePayloadLarge)
	case errors.Is(err, ErrRemoteChanged):
		return codeRemoteChange, statusForCode(codeRemoteChange)
	default:
		return codeInternal, statusForCode(codeInternal)
	}
}
