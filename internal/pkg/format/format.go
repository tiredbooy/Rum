package format

import "fmt"

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
