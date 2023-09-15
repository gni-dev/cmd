package dbg

import (
	"flag"
	"fmt"
	"io"
	"os"

	"gni.dev/cmd/internal/dbg/term"
)

var argInit string

func Run(args []string) {
	parseArgs(args)

	st := setRawTerminal()
	defer st.Restore()

	screen := struct {
		io.Reader
		io.Writer
	}{os.Stdin, os.Stdout}
	t := term.New(screen, "(gni) ")
	if err := t.Run(argInit); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func parseArgs(args []string) {
	dbgFlags := flag.NewFlagSet("dbg", flag.ExitOnError)
	dbgFlags.StringVar(&argInit, "init", "", "initial command to run")
	if err := dbgFlags.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func setRawTerminal() *term.State {
	if !term.IsTerminal(int(os.Stdout.Fd())) || !term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Fprintln(os.Stderr, "stdin and stdout must be terminals")
		os.Exit(1)
	}

	st, err := term.TerminalMode(int(os.Stdin.Fd()))
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to get terminal mode:", err)
		os.Exit(1)
	}
	return st
}
