package build

import (
	"flag"
)

type Args struct {
	Chdir      string
	DestPath   string
	DebugBuild bool
}

func CreateArgs(f *flag.FlagSet) *Args {
	var args Args
	f.StringVar(&args.Chdir, "C", "dir", "change working directory before building")
	f.StringVar(&args.DestPath, "o", "", "output path")
	f.BoolVar(&args.DebugBuild, "debug", false, "build on debug mode")
	return &args
}
