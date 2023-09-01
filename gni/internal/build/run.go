package build

import (
	"flag"
	"fmt"
	"os"
)

func Run(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Please specify target (apk)")
		os.Exit(1)
	}
	target := args[0]

	buildFlags := flag.NewFlagSet("build", flag.ExitOnError)
	a := CreateArgs(buildFlags)
	if err := buildFlags.Parse(args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if a.Chdir() != "." {
		if err := os.Chdir(a.Chdir()); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}

	m, err := DefaultMetadata()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	m.FixupAndroidVer()

	switch target {
	case "apk":
		err = AndroidAPK(m, a)
	default:
		fmt.Fprintln(os.Stderr, "Unknown target:", target)
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
