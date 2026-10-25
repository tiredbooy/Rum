package download

// Stable machine error codes surfaced to the UI alongside the free-text error.
const (
	// ErrorCodeStalled marks a connection that stayed open but delivered no bytes
	// for the stall timeout; the engine reconnects and resumes from the offset.
	ErrorCodeStalled = "stalled"
)
