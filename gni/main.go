package main

import (
	"fmt"
	"os"

	"gni.dev/cmd/gni/internal/build"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: gni <command>")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "build":
		build.Run(os.Args[2:])
	default:
		fmt.Fprintln(os.Stderr, "Unknown command:", os.Args[1])
		os.Exit(1)
	}
}
