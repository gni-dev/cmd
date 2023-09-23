package term

import (
	"flag"
	"fmt"
	"io"
	"os"
)

func Run(args []string) {
	var argInit string
	dbgFlags := flag.NewFlagSet("debug", flag.ExitOnError)
	dbgFlags.StringVar(&argInit, "init", "", "initial command to run")
	if err := dbgFlags.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	st := setRawTerminal()
	defer st.Restore()

	screen := struct {
		io.Reader
		io.Writer
	}{os.Stdin, os.Stdout}
	t := New(screen, "(gni) ")
	if err := t.Run(argInit); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func setRawTerminal() *State {
	if !IsTerminal(int(os.Stdout.Fd())) || !IsTerminal(int(os.Stdin.Fd())) {
		fmt.Fprintln(os.Stderr, "stdin and stdout must be terminals")
		os.Exit(1)
	}

	st, err := TerminalMode(int(os.Stdin.Fd()))
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to get terminal mode:", err)
		os.Exit(1)
	}
	return st
}
