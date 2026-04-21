package format

import (
	"fmt"
	"math"
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
