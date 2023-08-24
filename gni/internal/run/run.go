package run

import (
	"flag"
	"fmt"
	"os"

	"gni.dev/cmd/gni/internal/build"
)

func Run(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Please specify target (android)")
		os.Exit(1)
	}
	target := args[0]

	runFlags := flag.NewFlagSet("run", flag.ExitOnError)
	a := CreateArgs(runFlags)
	if err := runFlags.Parse(args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if err := os.Chdir(a.buildArgs.Chdir()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	m, err := build.DefaultMetadata()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	m.FixupAndroidVer()

	switch target {
	case "android":
		err = runAndroid(m, a)
	default:
		fmt.Fprintln(os.Stderr, "Unknown target:", target)
		os.Exit(1)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
