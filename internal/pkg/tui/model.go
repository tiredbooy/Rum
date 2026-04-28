package tui

import (
	"context"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gen2brain/beeep"
	"swiftget.com/internal/pkg/download"
)

type model struct {
	jobs           map[string]*download.Job
	mu             sync.RWMutex
	program        *tea.Program
	jobOrder       []string
	width          int
	height         int
	ready          bool
	opt            *download.Options
	visibleStart   int
	autoScroll     bool
	lastUserScroll time.Time
}

func NewModel(jobs map[string]*download.Job, jobOrder []string, opt *download.Options) *model {
	// order := make([]string, 0, len(jobs))
	// for id := range jobs {
	// 	order = append(order, id)
	// }
	return &model{
		jobs:       jobs,
		jobOrder:   jobOrder,
		opt:        opt,
		autoScroll: true,
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

	for _, id := range m.jobOrder {
		job, ok := m.jobs[id]
		if !ok {
			continue
		}
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
		case "left", "h":
			m.mu.Lock()
			if m.visibleStart > 0 {
				m.visibleStart--
			}
			m.autoScroll = false
			m.lastUserScroll = time.Now()
			m.mu.Unlock()

		case "right", "l":
			m.mu.Lock()
			if m.visibleStart+maxVisible < len(m.jobOrder) {
				m.visibleStart++
			}
			m.autoScroll = false
			m.lastUserScroll = time.Now()
			m.mu.Unlock()
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
			m.updateVisibleStart()
		}
		m.mu.Unlock()

	case tickMsg:
		m.mu.Lock()
		if !m.autoScroll && time.Since(m.lastUserScroll) > 3*time.Second {
			m.autoScroll = true
		}
		m.mu.Unlock()
		return m, tickCmd()

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
		m.updateVisibleStart()
		if m.allJobsDone() {
			m.mu.Unlock()
			return m, m.notifyCompletion()
		}
		m.mu.Unlock()
	}
	return m, nil
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

func (m *model) updateVisibleStart() {
	if !m.autoScroll {
		return
	}

	for i, id := range m.jobOrder {
		status := m.jobs[id].GetStatus()
		if status != download.StatusCompleted && status != download.StatusError {
			m.visibleStart = i
			return
		}
	}
	if len(m.jobOrder) > maxVisible {
		m.visibleStart = len(m.jobOrder) - maxVisible
	}
}
