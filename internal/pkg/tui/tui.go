package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"swiftget.com/internal/pkg/download"
)

func RunTUI(jobs map[string]*download.Job, opt *download.Options) {
	InitWorkerPool(opt.Parallel)

	m := NewModel(jobs, opt)
	p := tea.NewProgram(m, tea.WithAltScreen())
	m.SetProgram(p)
	if _, err := p.Run(); err != nil {
		panic(err)
	}

}
