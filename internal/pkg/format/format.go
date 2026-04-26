package format

import (
	"fmt"
	"math"
	"time"
)

func FormatSize(bytes int64) string {

	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	floatSize := float64(bytes)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", floatSize/GB)
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", floatSize/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", floatSize/KB)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

func FormatRemainingTime(remainingTimeSeconds float64) string {
	if remainingTimeSeconds < 0 {
		remainingTimeSeconds = 0
	}
	hours := math.Floor(remainingTimeSeconds / 3600)
	minutes := math.Floor(math.Mod(remainingTimeSeconds, 3600) / 60)
	seconds := math.Floor(remainingTimeSeconds) / 60

	if hours > 0 {
		return fmt.Sprintf("%02.fh %02.fm %02.fs", hours, minutes, seconds)
	} else if minutes > 0 {
		return fmt.Sprintf("%02.fm %02.fs", minutes, seconds)
	} else {
		return fmt.Sprintf("%02.fs", seconds)
	}
}

func FormatSpeed(bytesPerSec float64) string {
	if bytesPerSec <= 0 {
		return "0 B/s"
	}
	const unit = 1024
	if bytesPerSec < unit {
		return fmt.Sprintf("%.0f B/s", bytesPerSec)
	}
	div, exp := int64(unit), 0
	for n := bytesPerSec / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	switch exp {
	case 0:
		return fmt.Sprintf("%.1f KB/s", bytesPerSec/unit)
	case 1:
		return fmt.Sprintf("%.1f MB/s", bytesPerSec/float64(unit*unit))
	case 2:
		return fmt.Sprintf("%.1f GB/s", bytesPerSec/float64(unit*unit*unit))
	}
	return fmt.Sprintf("%.1f B/s", bytesPerSec)
}

func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	switch exp {
	case 0:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(unit))
	case 1:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(unit*unit))
	case 2:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(unit*unit*unit))
	}
	return fmt.Sprintf("%d B", bytes)
}

func FormatDuration(d time.Duration) string {
	if d <= 0 {
		return "--:--"
	}
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}
