package dbg

type Debugger interface {
	Run(program string, args []string) error
	Detach() error
}
