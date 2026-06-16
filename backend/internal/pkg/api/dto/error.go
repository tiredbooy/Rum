package dto

// ErrorResponse is the single, consistent error envelope used across the API.
// Keeping one shape lets clients (the frontend) handle errors uniformly.
//
//	{ "error": "human readable message", "code": "machine_code", "fields": {...} }
//
// `error` is a sanitized, client-safe message (never a raw internal error).
// `code` is a stable machine-readable code clients can branch on.
// `fields` carries optional per-field validation problems.
type ErrorResponse struct {
	Error  string            `json:"error"`
	Code   string            `json:"code,omitempty"`
	Fields map[string]string `json:"fields,omitempty"`
}

// Stable error codes. These are part of the API contract: add, don't repurpose.
const (
	CodeValidation   = "validation_error"
	CodeNotFound     = "not_found"
	CodeConflict     = "conflict"
	CodeInternal     = "internal_error"
	CodeRateLimited  = "rate_limited"
	CodeUnsupported  = "unsupported"
	CodePayloadLarge = "payload_too_large"
)
