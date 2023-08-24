package build

import (
	"flag"
	"path/filepath"
)

type Args struct {
	chdir        string
	outDir       string
	debugBuild   bool
	waitDebugger bool
}

func CreateArgs(f *flag.FlagSet) *Args {
	var a Args
	f.StringVar(&a.chdir, "C", "dir", "Change working directory before building")
	f.StringVar(&a.outDir, "o", "", "Output path. Default is out/")
	f.BoolVar(&a.debugBuild, "debug", false, "Build on debug mode")
	return &a
}

func (a *Args) Chdir() string {
	return a.chdir
}

func (a *Args) OutDir() string {
	out := a.outDir
	if out == "" {
		out = "out"
	}
	if a.debugBuild {
		return filepath.Join(out, "debug")
	} else {
		return filepath.Join(out, "release")
	}
}

func (a *Args) BuildDir() string {
	return filepath.Join(a.OutDir(), "gnibuild")
}

func (a *Args) DebugBuild() bool {
	return a.debugBuild
}

func (a *Args) WaitDebugger(wait bool) {
	a.waitDebugger = wait
}
