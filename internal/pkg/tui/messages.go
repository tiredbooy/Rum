package tui

type progressMsg struct {
	jobID      string
	downloaded int64
	total      int64
}

type jobDoneMsg struct {
	jobID string
	err   error
}
type pauseAllMsg struct{}
type resumeAllMsg struct{}
type quitMsg struct{}
