package dbg

import (
	"flag"
	"fmt"
	"io"
	"os"

	"gni.dev/cmd/internal/dbg/dap"
	"gni.dev/cmd/internal/dbg/term"
)

func RunDebug(args []string) {
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
	t := term.New(screen, "(gni) ")
	if err := t.Run(argInit); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func RunDAP(args []string) {
	var port int
	dbgFlags := flag.NewFlagSet("dap", flag.ExitOnError)
	dbgFlags.IntVar(&port, "port", 0, "port to listen on")
	if err := dbgFlags.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	var err error
	if port > 0 {
		s := dap.NewServer(port)
		err = s.Run()
	} else {
		pipe := struct {
			io.Reader
			io.Writer
		}{os.Stdin, os.Stdout}
		s := dap.NewSession(pipe)
		s.Serve()
	}
	if err != nil && err != io.EOF {
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
