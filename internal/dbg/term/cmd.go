package term

import (
	"fmt"
	"io"
	"strings"

	"gni.dev/cmd/internal/dbg"
	"gni.dev/cmd/internal/dbg/lldb"
)

type command struct {
	aliases []string
	fn      func(args []string) error
}

type Commands struct {
	cmds []command
	d    dbg.Debugger
}

func DebuggerCommands() *Commands {
	c := &Commands{}
	c.cmds = append(c.cmds,
		command{
			aliases: []string{"exit", "quit", "q"},
			fn:      c.exit,
		},
		command{
			aliases: []string{"run", "r"},
			fn:      c.run,
		},
	)
	return c
}

func (c *Commands) Process(line string) error {
	args := strings.Fields(line)
	if len(args) == 0 {
		return fmt.Errorf("empty command")
	}

	for _, cmd := range c.cmds {
		for _, alias := range cmd.aliases {
			if args[0] == alias {
				return cmd.fn(args[1:])
			}
		}
	}
	return fmt.Errorf("unknown command '%s'", args[0])
}

func (c *Commands) Close() error {
	return c.d.Detach()
}

func (c *Commands) exit(args []string) error {
	return io.EOF
}

func (c *Commands) run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no executable specified")
	}
	var err error
	c.d, err = lldb.LaunchServer()
	if err != nil {
		return err
	}
	return c.d.Run(args[0], args[1:])
}
