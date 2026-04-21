package utils

import (
	"fmt"
	"os"

	"github.com/cheggaaa/pb/v3"
	"swiftget.com/internal/pkg/format"
)

// ANSI colors
var (
	red   = "\033[31m"
	green = "\033[32m"
	cyan  = "\033[36m"
	blue  = "\033[34m"
	reset = "\033[0m"
)

// Global progress bar instance
var globalBar *pb.ProgressBar

// --- Constants for template ---
const (
	barWidth        = 40
	barLeftBracket  = "["
	barRightBracket = "]"
	barFullChar     = "█"
	barPointerChar  = ">"
	barEmptyChar    = "_"
)

func NewProgressBar(totalSize int64, fileName string) *pb.ProgressBar {
	maxNameLength := 20
	displayFileName := fileName
	if len(displayFileName) > maxNameLength {
		displayFileName = displayFileName[:maxNameLength-3] + "..."
	}
	fmt.Printf("%sDownloading File:%s %s\n", cyan, reset, displayFileName)

	bar := pb.New64(totalSize)
	templateString := fmt.Sprintf(`{{cyan Current.}} / {{cyan .Total}}  {{blue}}{{bar . "%s" "%s" "%s" "%s" "%s"}}{{reset}}  {{red}}{{percent .}}  {{red}}{{speed .}}  {{red}}{{rtime .}}`,
		barLeftBracket, barFullChar, barPointerChar, barEmptyChar, barRightBracket)

	bar.SetTemplateString(templateString)
	bar.SetWidth(100)
	bar.Set(pb.Bytes, true)

	// برای چاپ خوانا قبل از شروع می‌تونیم سایز کل رو نشون بدیم:
	fmt.Printf("%s Total Size: %s\n", red, format.FormatSize(totalSize))

	bar.Start()
	return bar
}

func UpdateProgress(bar *pb.ProgressBar, n int64) {
	if bar != nil {
		bar.Add64(n)
	}
}

func FinishProgress(bar *pb.ProgressBar) {
	if bar != nil {
		bar.Finish()
		globalBar = nil
		fmt.Fprintln(os.Stderr, "\n✅ Download complete!\n")
	}
}
