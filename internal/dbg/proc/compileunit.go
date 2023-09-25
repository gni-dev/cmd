package proc

import (
	"debug/dwarf"
	"io"
	"path/filepath"
	"strings"
)

type compileUnit struct {
	name string

	files []*fileInfo
	funcs []*Func
}

type fileInfo struct {
	name  string
	lines map[int]uint64
}

func newCompileUnit() *compileUnit {
	return &compileUnit{}
}

func (cu *compileUnit) loadLines(d *dwarf.Data, e *dwarf.Entry) error {
	r, err := d.LineReader(e)
	if err != nil {
		return err
	}

	files := make(map[string]*fileInfo)

	for {
		var l dwarf.LineEntry
		err := r.Next(&l)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		f, ok := files[l.File.Name]
		if ok {
			f.lines[l.Line] = l.Address
		} else {
			files[l.File.Name] = &fileInfo{
				name: l.File.Name,
				lines: map[int]uint64{
					l.Line: l.Address,
				},
			}
		}
	}
	for _, f := range files {
		cu.files = append(cu.files, f)
	}
	return nil
}

func (cu *compileUnit) buildFileIdx(m map[string][]*fileInfo) {
	for _, f := range cu.files {
		pos := len(f.name)
		for {
			pos = strings.LastIndex(f.name[:pos], string(filepath.Separator))
			if pos == -1 {
				break
			}
			name := f.name[pos:]
			m[name] = append(m[name], f)
		}
	}
}

func (cu *compileUnit) loadDebugInfo(d *dwarf.Data, r *dwarf.Reader) error {
	depth := 0

	for {
		e, err := r.Next()
		if err != nil {
			return err
		}
		if e == nil {
			break
		}

		switch e.Tag {
		case 0:
			if depth == 0 {
				return nil
			}
			depth--
		case dwarf.TagSubprogram:
			f := newFunc(d, e)
			if f != nil {
				cu.funcs = append(cu.funcs, f)
			}
			if e.Children {
				r.SkipChildren()
			}
		default:
			if e.Children {
				depth++
			}
		}
	}
	return nil
}
