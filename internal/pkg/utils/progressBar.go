package utils

import (
	"fmt"
	"os"

	"github.com/cheggaaa/pb/v3"
	"github.com/gen2brain/beeep"
)

var (
	red   = "\033[31m"
	green = "\033[32m"
	cyan  = "\033[36m"
	blue  = "\033[34m"
	reset = "\033[0m"
)

func NewProgressBar(totalSize, currentSize int64, fileName string) *pb.ProgressBar {
	// Truncate long file names
	const maxNameLen = 25
	displayName := fileName
	if len(displayName) > maxNameLen {
		displayName = displayName[:maxNameLen-3] + "..."
	}

	fmt.Printf("%s▶ Downloading:%s %s\n", cyan, reset, displayName)

	bar := pb.New64(totalSize)
	bar.SetCurrent(currentSize)

	bar.SetWidth(60)

	template := `{{ cyan "[" }}{{ magenta bar . }}{{ cyan "]" }} {{ percent . }} {{ blue "│" }} {{ speed . }} {{ blue "│" }} {{ rtime . }} {{ blue "│" }} {{ cyan Current. }} / {{ cyan Total. }}`

	bar.SetTemplateString(template)
	bar.SetWidth(100)
	bar.Set(pb.Bytes, true)
	bar.Start()

	return bar
}

func UpdateProgress(bar *pb.ProgressBar, n int64) {
	if bar != nil {
		bar.Add64(n)
	}
}

func FinishProgress(bar *pb.ProgressBar, fileName string) {
	if bar != nil {
		bar.Finish()
		fmt.Fprintln(os.Stderr, "\n✅ Download complete!\n")

		beeep.Notify("Download Completed", fmt.Sprintf("Your file '%s' Have been Downloaded.", fileName), "")
	}
}
