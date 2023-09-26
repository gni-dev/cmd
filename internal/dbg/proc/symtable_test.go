package proc

import (
	"debug/elf"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"gni.dev/cmd/internal/dbg/test"
)

func TestMain(m *testing.M) {
	os.Exit(test.Run(m))
}

func TestSymbols(t *testing.T) {
	fixt := test.Build("symbols")
	var sym SymTable

	elfFile, err := elf.Open(fixt)
	assert.NoError(t, err)
	defer elfFile.Close()

	dwarf, err := elfFile.DWARF()
	assert.NoError(t, err)

	assert.NoError(t, sym.LoadImage(dwarf))

	_, _, err = sym.LineToPC("symbols.go", 33)
	assert.NoError(t, err)

	_, _, err = sym.LineToPC("//symbols.go", 34)
	assert.NoError(t, err)

	_, _, err = sym.LineToPC("symbols.go", 9)
	assert.ErrorContains(t, err, "not found")

	_, _, err = sym.LineToPC("foo.go", 4)
	var errAmbiguous *ErrAmbiguous
	assert.ErrorAs(t, err, &errAmbiguous)
}
