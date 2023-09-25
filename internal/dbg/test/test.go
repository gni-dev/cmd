package test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

var tmpDir string

func Build(name string) string {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		fmt.Fprintln(os.Stderr, "cannot find source file")
		os.Exit(1)
	}

	fixt := filepath.Join(filepath.Dir(filename), "fixtures", name+".go")
	binary := filepath.Join(tmpDir, name)

	flags := []string{"build", "-gcflags=all=-N -l", "-o", binary, fixt}

	cmd := exec.Command("go", flags...)
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Fprintln(os.Stderr, "failed to build test binary: ", err)
		fmt.Fprintln(os.Stderr, string(out))
		os.Exit(1)
	}
	return binary
}

func Run(m *testing.M) int {
	var err error
	tmpDir, err = os.MkdirTemp("", "gni-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	code := m.Run()

	os.RemoveAll(tmpDir)
	return code
}
