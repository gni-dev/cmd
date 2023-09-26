package proc

import (
	"debug/dwarf"
	"fmt"
	"path/filepath"
	"strings"
)

type SymTable struct {
	cus      []*compileUnit
	cuRanges []compileUnitRange
	fileIdx  map[string][]*fileInfo
}

type compileUnitRange struct {
	lowpc, highpc uint64
	cu            *compileUnit
}

type ErrAmbiguous struct {
	Location   string
	Candidates []string
}

func (a *ErrAmbiguous) Error() string {
	return fmt.Sprintf("Location %q ambiguous: %s", a.Location, strings.Join(a.Candidates, ", "))
}

// LoadImage loads the debug information from the given DWARF data.
func (s *SymTable) LoadImage(d *dwarf.Data) error {
	r := d.Reader()
	for {
		e, err := r.Next()
		if err != nil {
			return err
		}
		if e == nil {
			break
		}
		switch e.Tag {
		case dwarf.TagCompileUnit:
			lang, _ := e.Val(dwarf.AttrLanguage).(int64)
			if lang != 22 { // DW_LANG_Go
				continue
			}

			cu := newCompileUnit()
			cu.name, _ = e.Val(dwarf.AttrName).(string)

			ranges, _ := d.Ranges(e)
			for _, r := range ranges {
				s.cuRanges = append(s.cuRanges, compileUnitRange{
					lowpc:  r[0],
					highpc: r[1],
					cu:     cu,
				})
			}
			s.cus = append(s.cus, cu)

			if err := cu.loadLines(d, e); err != nil {
				return err
			}

			if e.Children {
				if err := cu.loadDebugInfo(d, r); err != nil {
					return err
				}
			}
		default:
			r.SkipChildren()
		}
	}
	s.fileIdx = make(map[string][]*fileInfo)
	for _, cu := range s.cus {
		cu.buildFileIdx(s.fileIdx)
	}
	return nil
}

// LineToPC returns the PC for the given file and line.
func (s *SymTable) LineToPC(file string, line int) (uint64, string, error) {
	normalized := filepath.Join(string(filepath.Separator), file)
	files, ok := s.fileIdx[normalized]
	if !ok {
		return 0, "", fmt.Errorf("file %s not found", file)
	}

	if len(files) > 1 {
		var candidates []string
		for _, f := range files {
			candidates = append(candidates, f.name)
		}
		return 0, "", &ErrAmbiguous{
			Location:   file,
			Candidates: candidates,
		}
	}

	if pc, ok := files[0].lines[line]; ok {
		return pc, files[0].name, nil
	}
	return 0, "", fmt.Errorf("location %s:%d not found", file, line)
}
