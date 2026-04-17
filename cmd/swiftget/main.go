package main

import (
	"fmt"
	"os"

	"swiftget.com/internal/pkg/download"
)



func main() {
	if len(os.Args) < 2 {
		download.RunProgram(os.Args[1:])
		return
	}

	sub := os.Args[1]
	switch sub {
	case "get":
		download.RunProgram(os.Args[1:])
	case "version":
		fmt.Println("SwitftGet v0.0.1")
	case "help":
		printUsage()
	default:
		download.RunProgram(os.Args[1:])
		os.Exit(2)
		return
	}
}

func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  switftget URL [flags]")
	fmt.Println("  switftget version")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  switftget URL --out [OUTPUT PATH] --p 100 --b 1200(KB)")
}
