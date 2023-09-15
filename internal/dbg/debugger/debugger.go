package debugger

import "gni.dev/cmd/internal/dbg/lldb"

type Debugger struct {
	lldb *lldb.LLDB
}

func New() *Debugger {
	return &Debugger{}
}

func (d *Debugger) Launch(exec string) error {
	var err error
	d.lldb, err = lldb.LaunchServer()
	if err != nil {
		return err
	}

	return nil
}

func (d *Debugger) Detach() error {
	if d.lldb == nil {
		return nil
	}
	return d.lldb.Detach()
}
