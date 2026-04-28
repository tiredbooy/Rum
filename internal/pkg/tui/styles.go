package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"swiftget.com/internal/pkg/format"
	"swiftget.com/internal/pkg/utils"
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

func renderJobRow(status, name string, downloaded, total int64, width int) string {
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

	// Status colouring
	statusColor := statusColors[status]
	if statusColor == "" {
		statusColor = lipgloss.Color("#FFFFFF")
	}
	styledStatus := lipgloss.NewStyle().
		Foreground(statusColor).
		Bold(true).
		Render(fmt.Sprintf("%-10s", status))

	return fmt.Sprintf("%s %-*s %-10s %-8s %-20s %-7s %s",
		styledStatus, 40, shortURL(name, 40), " ", " ", bar, percentStr, sizeStr)
}

func renderProgressBar(percent float64, width int, status string) string {
	if width < 10 {
		width = 10
	}

	// Colour palette
	startColor := lipgloss.Color("#7D56F4") // Purple
	endColor := lipgloss.Color("#39FF14")   // Neon green

	// Unknown‑size downloads: animated snake
	if status == "running" && percent == 0 {
		pos := int(time.Now().UnixMilli()/200) % width
		var bar strings.Builder
		for i := 0; i < width; i++ {
			if i == pos {
				bar.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#FFAA00")).Render("█"))
			} else {
				bar.WriteString(" ")
			}
		}
		return "[" + bar.String() + "]"
	}
	filled := int(percent * float64(width))
	if filled < 0 {
		filled = 0
	}
	if filled > width {
		filled = width
	}

	var bar strings.Builder
	for i := 0; i < width; i++ {
		if i < filled {
			t := float64(i) / float64(width-1) // gradient stops
			if width <= 1 {
				t = 1
			}
			col := utils.InterpolateColor(startColor, endColor, t)
			bar.WriteString(lipgloss.NewStyle().Foreground(col).Render("█"))
		} else {
			bar.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#3C3C3C")).Render(" ")) // empty part colour
		}
	}
	return "[" + bar.String() + "]"
}

func shortURL(url string, truncate int) string {
	if len(url) > truncate {
		return url[:truncate-3] + "..."
	}
	return url
}

// "░"
