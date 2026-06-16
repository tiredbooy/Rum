package dto

// DefaultPageSize / MaxPageSize cap the page size so a client can never ask the
// server to materialize an unbounded response (go-api-design: always cap page size).
const (
	DefaultPageSize = 50
	MaxPageSize     = 200
)

// ListPage is the *opt-in* paginated envelope for GET /downloads.
// It is only returned when the client passes a pagination param (limit/offset),
// so the existing frontend — which expects a bare array — is unaffected.
type ListPage struct {
	Items      []DownloadResponse `json:"items"`
	Total      int                `json:"total"`
	Limit      int                `json:"limit"`
	Offset     int                `json:"offset"`
	NextOffset *int               `json:"next_offset,omitempty"`
}

// ClampLimit normalizes a requested limit to [1, MaxPageSize], defaulting when <= 0.
func ClampLimit(limit int) int {
	if limit <= 0 {
		return DefaultPageSize
	}
	if limit > MaxPageSize {
		return MaxPageSize
	}
	return limit
}
