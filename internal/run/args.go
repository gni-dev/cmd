package run

import (
	"flag"

	"gni.dev/cmd/internal/build"
)

type Args struct {
	buildArgs *build.Args

	wait bool
}

func CreateArgs(f *flag.FlagSet) *Args {
	a := &Args{buildArgs: build.CreateArgs(f)}
	f.BoolVar(&a.wait, "wait", false, "Wait for the SIGCONT signal before running the app")
	return a
}
