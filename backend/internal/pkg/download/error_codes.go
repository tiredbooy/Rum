package download

// Stable machine codes persisted on a Job (Job.ErrorCode, "error_code" in JSON)
// so the UI can branch without parsing the free-text error message.
const (
	// ErrCodeBlockedPage: the server answered with a web page (captive portal,
	// ISP block page, login wall) where a file was expected.
	ErrCodeBlockedPage = "blocked_page"
)
