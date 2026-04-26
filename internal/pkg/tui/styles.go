package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"swiftget.com/internal/pkg/download"
	"swiftget.com/internal/pkg/format"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4")).
			MarginBottom(1)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#626262"))

	progressBarFilled = lipgloss.NewStyle().
				Background(lipgloss.Color("#7D56F4")).
				Width(10)

	progressBarEmpty = lipgloss.NewStyle().
				Background(lipgloss.Color("#3C3C3C")).
				Width(10)
)

func renderJobRow(job *download.Job, width int) string {
    status := fmt.Sprintf("[%-7s]", job.Status)
    name := shortURL(job.URL, 40)
    speed := format.FormatSpeed(job.Speed)
    eta := format.FormatDuration(job.RemainingTime)
    percent := 0.0
    if job.TotalSize > 0 {
        percent = float64(job.Downloaded) / float64(job.TotalSize)
    }
    bar := renderProgressBar(percent, 20)
    percentStr := fmt.Sprintf("%5.1f%%", percent*100)
    sizeStr := fmt.Sprintf("%s / %s", format.FormatBytes(job.Downloaded), format.FormatBytes(job.TotalSize))

    return fmt.Sprintf("%-10s %-42s %-12s %-12s %-22s %-8s %s",
        status, name, speed, eta, bar, percentStr, sizeStr)
}

func renderProgressBar(percent float64, width int) string {
	filled := int(percent * float64(width))
	if filled < 0 {
		filled = 0
	}
	if filled > width {
		filled = width
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	return bar
}

func shortURL(url string, truncate int) string {
	if len(url) > truncate {
		return url[:truncate-3] + "..."
	}
	return url
}
