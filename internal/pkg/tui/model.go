package tui

import (
	"context"
	"fmt"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gen2brain/beeep"
	"swiftget.com/internal/pkg/download"
	"swiftget.com/internal/pkg/format"
)

type model struct {
	jobs     map[string]*download.Job
	mu       sync.RWMutex
	program  *tea.Program
	jobOrder []string
	width    int
	height   int
	ready    bool
	opt      *download.Options
}

func NewModel(jobs map[string]*download.Job, opt *download.Options) *model {
	order := make([]string, 0, len(jobs))
	for id := range jobs {
		order = append(order, id)
	}
	return &model{
		jobs:     jobs,
		jobOrder: order,
		opt:      opt,
	}
}

func (m *model) SetProgram(p *tea.Program) {
	m.mu.Lock()
	m.program = p
	m.mu.Unlock()
}

func (m *model) Init() tea.Cmd {
	cmds := make([]tea.Cmd, 0)
	m.mu.RLock()

	for _, job := range m.jobs {
		if job.Status == download.StatusPending || job.Status == download.StatusPaused {
			cmds = append(cmds, startDownloadCmd(job, m.opt, m.program))
		}
	}
	m.mu.RUnlock()
	return tea.Batch(cmds...)
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			// Cancel all jobs – do NOT set status manually!
			m.mu.Lock()
			for _, job := range m.jobs {
				if job.CancelFunc != nil {
					job.CancelFunc()
				}
			}
			m.mu.Unlock()
			beeep.Notify("Downlaods Paused Successfully", "All Downloads Have been Paused Successfully.", "")
			return m, nil

		case "r":
			return m, m.resumePaused()

		case "q":
			m.mu.Lock()
			for _, job := range m.jobs {
				if job.CancelFunc != nil {
					job.CancelFunc()
				}
			}
			m.mu.Unlock()
			beeep.Notify("App Closed Successfully", "App Have been closed Successfully And all downloads are paused.", "")
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		m.mu.Lock()
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		m.mu.Unlock()

	case progressMsg:
		m.mu.Lock()
		if job, ok := m.jobs[msg.jobID]; ok {
			job.SetDownloaded(msg.downloaded)
			job.SetTotalSize(msg.total)
		}
		m.mu.Unlock()

	case jobDoneMsg:
		m.mu.Lock()
		if job, ok := m.jobs[msg.jobID]; ok {
			if msg.err == nil {
				job.SetStatus(download.StatusCompleted)
			} else if msg.err == context.Canceled {
				job.SetStatus(download.StatusPaused)
			} else {
				job.SetStatus(download.StatusError)
				job.SetError(msg.err)
			}
		}
		if m.allJobsDone() {
			m.mu.Unlock()
			return m, m.notifyCompletion()
		}
		m.mu.Unlock()
	}
	return m, nil
}

func (m *model) View() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.ready {
		return "Loading..."
	}

	var s strings.Builder
	s.WriteString(titleStyle.Render("Rum – Download Manager") + "\n\n")
	s.WriteString(fmt.Sprintf("%-10s %-42s %-12s %-12s %-22s %-8s %s\n",
		"STATUS", "NAME", "SPEED", "ETA", "PROGRESS", "PCT", "SIZE"))

	for _, id := range m.jobOrder {
		job, ok := m.jobs[id]
		if !ok {
			continue
		}
		
		status := job.GetStatus()
		downloaded := job.GetDownloaded()
		total := job.GetTotalSize()
		speed := job.GetSpeed()
		eta := job.GetRemainingTime()
		name := job.GetFileName()
		if name == "" {
			name = shortURL(job.GetURL(), 50)
		} else {
			name = shortURL(name, 50)
		}

		speedStr := format.FormatSpeed(speed)
		etaStr := ""
		if eta > 0 {
			etaStr = eta.String() // "5m22s"
		} else if status == "running" {
			etaStr = "…"
		} else {
			etaStr = "--:--"
		}
		s.WriteString(renderJobRow(status, name, job.ID, speedStr, etaStr, downloaded, total, m.width) + "\n")
	}
	s.WriteString(helpStyle.Render("\nCtrl+C: pause • r: resume • q: quit"))
	return s.String()
}

func (m *model) pauseAllAndSave() tea.Cmd {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, job := range m.jobs {
		if (job.Status == download.StatusRunning || job.Status == download.StatusPending) && job.CancelFunc != nil {
			job.CancelFunc()
			job.Status = download.StatusPaused
		}
	}

	go download.SaveJobsToDisk()
	return nil
}

func (m *model) resumePaused() tea.Cmd {
	m.mu.Lock()
	defer m.mu.Unlock()

	var cmds []tea.Cmd
	for _, job := range m.jobs {
		if job.GetStatus() == download.StatusPaused {
			cmds = append(cmds, startDownloadCmd(job, m.opt, m.program))
		}
	}
	return tea.Batch(cmds...)
}

func (m *model) allJobsDone() bool {
	for _, job := range m.jobs {
		status := job.GetStatus()
		if status != download.StatusCompleted && status != download.StatusError {
			return false
		}
	}
	return true
}

func (m *model) notifyCompletion() tea.Cmd {
	return func() tea.Msg {
		beeep.Notify("Downloads Finished", "All Files have been downloaded successfully!", "")
		beeep.Beep(beeep.DefaultFreq, beeep.DefaultDuration)
		return nil
	}
}
