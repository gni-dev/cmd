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

	_, err = sym.LineToPC("symbols.go", 28)
	assert.NoError(t, err)

	_, err = sym.LineToPC("symbols.go", 24)
	assert.Error(t, err)
}
