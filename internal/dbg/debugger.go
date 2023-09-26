package dbg

const (
	ArchAMD64 = "amd64"
	ArchARM64 = "arm64"
)

type Breakpoint struct {
	ID   int
	Func string
	File string
	Line int
}

type Debugger interface {
	// Run the program with the given arguments.
	Run(program string, args []string) error
	// Detach from the running program.
	Detach() error
	// Set the given breakpoints in the program.
	SetBreakpoint(bp *Breakpoint) (*Breakpoint, error)
	// Continue execution.
	Continue() error
}
