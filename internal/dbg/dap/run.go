package dap

import (
	"flag"
	"fmt"
	"io"
	"os"
)

func Run(args []string) {
	var port int
	dbgFlags := flag.NewFlagSet("dap", flag.ExitOnError)
	dbgFlags.IntVar(&port, "port", 0, "port to listen on")
	if err := dbgFlags.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	var err error
	if port > 0 {
		s := NewServer(port)
		err = s.Run()
	} else {
		pipe := struct {
			io.Reader
			io.Writer
		}{os.Stdin, os.Stdout}
		s := NewSession(pipe)
		err = s.Serve()
	}
	if err != nil && err != io.EOF {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
