package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"swiftget.com/internal/pkg/format"
)

const maxVisible = 8

var (
	statusColors = map[string]lipgloss.Color{
		"pending":   lipgloss.Color("#FFA500"), // orange
		"running":   lipgloss.Color("#00FF00"), // green
		"paused":    lipgloss.Color("#FFFF00"), // yellow
		"completed": lipgloss.Color("#00FFFF"), // cyan
		"error":     lipgloss.Color("#FF0000"), // red
		"waiting":   lipgloss.Color("#808080"), // grey
	}

	headerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#AAAAAA")).
			Bold(true).
			MarginBottom(1)

	footerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#626262")).
			MarginTop(1)

	batchInfoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#999999")).
			Italic(true)

	separatorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#444444"))
)

func (m *model) View() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.ready {
		return "Loading..."
	}

	totalJobs := len(m.jobs)
	completedJobs := 0
	for _, job := range m.jobs {
		if job.GetStatus() == "completed" {
			completedJobs++
		}
	}

	var s strings.Builder
	s.WriteString(titleStyle.Render("⬇ Rum – Download Manager") + "\n")
	s.WriteString(separatorStyle.Render(strings.Repeat("─", m.width)) + "\n\n")
	s.WriteString(headerStyle.Render(fmt.Sprintf("%-10s %-*s %-10s %-8s %-20s %-7s %s",
		"STATUS", 40, "NAME", "SPEED", "ETA", "PROGRESS", "PCT", "SIZE")) + "\n")

	start := m.visibleStart
	end := start + maxVisible
	if end > len(m.jobOrder) {
		end = len(m.jobOrder)
	}
	visibleIDs := m.jobOrder[start:end]

	for _, id := range visibleIDs {
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
			name = shortURL(job.GetURL(), 42)
		}

		statusColor := statusColors[status]
		if statusColor == "" {
			statusColor = lipgloss.Color("#FFFFFF")
		}
		styledStatus := lipgloss.NewStyle().
			Foreground(statusColor).
			Bold(true).
			Render(fmt.Sprintf("%-10s", status))

		speedStr := format.FormatSpeed(speed)
		if speed <= 0 && status == "running" {
			speedStr = "…"
		}
		etaStr := "--:--"
		if eta > 0 {
			etaStr = eta.String()
		} else if status == "running" {
			etaStr = "…"
		}

		percent := 0.0
		if total > 0 {
			percent = float64(downloaded) / float64(total)
			if percent > 1.0 {
				percent = 1.0
			}
		}
		bar := renderProgressBar(percent, 20, status)
		percentStr := fmt.Sprintf("%5.1f%%", percent*100)
		sizeStr := fmt.Sprintf("%s / %s", format.FormatBytes(downloaded), format.FormatBytes(total))
		if total <= 0 {
			sizeStr = fmt.Sprintf("%s / ?", format.FormatBytes(downloaded))
		}

		row := fmt.Sprintf("%s %-*s %-10s %-8s %-20s %-7s %s",
			styledStatus, 40, shortURL(name, 40), speedStr, etaStr, bar, percentStr, sizeStr)
		s.WriteString(row + "\n")
	}
	
	s.WriteString("\n" + batchInfoStyle.Render(fmt.Sprintf("Completed: %d/%d • Showing %d–%d of %d downloads",
		completedJobs, totalJobs, start+1, end, totalJobs)))
	if !m.autoScroll {
		s.WriteString(" " + lipgloss.NewStyle().Foreground(lipgloss.Color("#FFA500")).Render("[manual]"))
	}
	s.WriteString("\n")
	s.WriteString(separatorStyle.Render(strings.Repeat("─", m.width)) + "\n")
	s.WriteString(helpStyle.Render("Ctrl+C: pause • r: resume • q: quit • ->: Next page • <-: Prev Page"))

	return s.String()
}
