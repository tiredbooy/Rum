package tui

import (
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"swiftget.com/internal/pkg/download"
)

func RunTUI(jobs map[string]*download.Job, jobOrder []string, opt *download.Options) {
	InitWorkerPool(opt.Parallel)
	m := NewModel(jobs, jobOrder, opt)
	p := tea.NewProgram(m,
		tea.WithInput(os.Stdin),
		tea.WithOutput(os.Stderr),
	)
	m.SetProgram(p)
	if _, err := p.Run(); err != nil {
		panic(err)
	}
}
