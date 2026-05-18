package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

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
type tickMsg time.Time

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}
