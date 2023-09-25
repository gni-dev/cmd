package proc

import (
	"debug/dwarf"
	"strings"
)

type Func struct {
	name          string
	lowpc, highpc uint64
}

func newFunc(d *dwarf.Data, e *dwarf.Entry) *Func {
	name, ok := e.Val(dwarf.AttrName).(string)
	ranges, _ := d.Ranges(e)
	if ok && len(ranges) > 0 {
		return &Func{
			name:   name,
			lowpc:  ranges[0][0],
			highpc: ranges[0][1],
		}
	}
	return nil
}

func (f *Func) Name() string {
	return f.name
}

func (f *Func) BaseName() string {
	dot := strings.LastIndex(f.name, ".")
	if dot != -1 {
		return f.name[dot+1:]
	}
	return f.name
}
