package tui

import (
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/tiredbooy/Rum/backend/internal/pkg/download"
)

func RunTUI(jobs map[string]*download.Job, jobOrder []string, opt *download.Options) {
	InitWorkerPool(opt.Parallel)
	m := NewModel(jobs, jobOrder, opt)
	p := tea.NewProgram(m,
		tea.WithInput(os.Stdin),
		tea.WithOutput(os.Stderr),
		tea.WithAltScreen(),
	)
	m.SetProgram(p)

	// Let the engine ask the TUI to quit (e.g. the "close" post-download
	// action). Quit() is safe to call from another goroutine and is a no-op
	// once the program has already stopped.
	download.SetQuitFunc(func() { p.Quit() })

	if _, err := p.Run(); err != nil {
		panic(err)
	}
}
