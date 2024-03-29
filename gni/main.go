package main

import (
	"fmt"
	"os"

	"gni.dev/cmd/internal/build"
	"gni.dev/cmd/internal/dbg/dap"
	"gni.dev/cmd/internal/dbg/term"
	"gni.dev/cmd/internal/run"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: gni <command>")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "build":
		build.Run(os.Args[2:])
	case "run":
		run.Run(os.Args[2:])
	case "debug":
		term.Run(os.Args[2:])
	case "dap":
		dap.Run(os.Args[2:])
	default:
		fmt.Fprintln(os.Stderr, "Unknown command:", os.Args[1])
		os.Exit(1)
	}
}
