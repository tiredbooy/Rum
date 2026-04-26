package tui

import (
	"fmt"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"swiftget.com/internal/pkg/download"
)

type model struct {
	Jobs     map[string]*download.Job
	Mu       sync.RWMutex
	Program  *tea.Program
	JobOrder []string
	Status   string
	Error    error
	Width    int
	Height   int
	Ready    bool
	Opt      *download.Options
}

func NewModel(jobs map[string]*download.Job, opt *download.Options) model {
	order := make([]string, 0, len(jobs))
	for id := range jobs {
		order = append(order, id)
	}
	return model{
		Jobs:     jobs,
		JobOrder: order,
		Status:   "downloading",
		Opt:      opt,
	}
}

func (m *model) Init() tea.Cmd {
	for _, job := range m.Jobs {
		if job.Status == "pending" || job.Status == "paused" {
			go runJob(job, m.Program, m.Opt)
		}
	}
	return nil
}
func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			for _, job := range m.Jobs {
				if (job.Status == "running" || job.Status == "pending") && job.CancelFunc != nil {
					job.CancelFunc()
					job.Status = "paused"
				}
			}
			download.SaveJobsToDisk()
			fmt.Println("Use 'q' to Terminate the app.")
			return m, nil
		case "r":
			fmt.Println("DEBUG: r pressed, resuming paused jobs")
			for _, job := range m.Jobs {
				if job.Status == "paused" {
					fmt.Printf("DEBUG: Resuming job %s\n", job.ID[:8])
					go runJob(job, m.Program, m.Opt)
				}
			}
			return m, nil
		case "q":
			// Cancel all running/pending jobs before quitting
			for _, job := range m.Jobs {
				if (job.Status == "running" || job.Status == "pending") && job.CancelFunc != nil {
					job.CancelFunc()
					job.Status = "paused"
				}
			}
			download.SaveJobsToDisk()
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		m.Width = msg.Width
	case progressMsg:
		m.Mu.Lock()
		if job, ok := m.Jobs[msg.jobID]; ok {
			job.Downloaded = msg.downloaded
			job.TotalSize = msg.total
		}
		m.Mu.Unlock()

	case jobDoneMsg:
		m.Mu.Lock()
		if job, ok := m.Jobs[msg.jobID]; ok {
			if msg.err == nil {
				job.Status = "completed"
			} else {
				job.Status = "error"
				job.Error = msg.err
			}
		}
		m.Mu.Unlock()
		return m, nil

	}

	return m, nil
}

func (m *model) View() string {
	if m.Width == 0 {
		return "Loading..."
	}
	s := titleStyle.Render("Rum – Download Manager") + "\n\n"
	headers := fmt.Sprintf("%-10s %-42s %-12s %-12s %-22s %-8s %s\n",
		"STATUS", "NAME", "SPEED", "ETA", "PROGRESS", "PCT", "SIZE")
	s += headers
	// headers = fmt.Sprintf("%-10s %-45s %-12s %-12s %s\n", "STATUS", "NAME", "SPEED", "ETA", "PROGRESS")
	// s += headers
	for _, id := range m.JobOrder {
		job := m.Jobs[id]
		s += renderJobRow(job, m.Width) + "\n"
	}
	s += helpStyle.Render("\nCtrl+C: pause • r: resume • q: quit")
	return s
}
