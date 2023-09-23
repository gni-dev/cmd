package sys

import "encoding/binary"

const (
	_AT_NULL         = 0
	_AT_ENTRY        = 9
	_AT_SYSINFO_EHDR = 33

	ptrSize = 8
)

type AuxV struct {
	// The following fields are present in all ELF binaries.
	// See /usr/include/linux/auxvec.h
	Entry uint64
	Vdso  uint64
}

func ParseAuxV(auxv []byte) AuxV {
	var a AuxV
	for i := 0; auxv[i] != _AT_NULL; i += ptrSize * 2 {
		tag := binary.LittleEndian.Uint64(auxv[i:])
		val := binary.LittleEndian.Uint64(auxv[i+ptrSize:])
		switch tag {
		case _AT_ENTRY:
			a.Entry = val
		case _AT_SYSINFO_EHDR:
			a.Vdso = val
		}
	}
	return a
}
