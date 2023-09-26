package lldb

import (
	"debug/elf"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"gni.dev/cmd/internal/dbg"
	"gni.dev/cmd/internal/dbg/proc"
)

type LLDB struct {
	server     *os.Process
	tmpDir     string
	c          *conn
	connCloser io.Closer
	sym        proc.SymTable
	arch       string
	bpKind     int
	bpCnt      int
	bps        map[int]*dbg.Breakpoint
}

func LaunchServer() (dbg.Debugger, error) {
	path, err := exec.LookPath("lldb-server")
	if err != nil {
		return nil, fmt.Errorf("lldb-server unavailable: %v", err)
	}

	tmp, err := os.MkdirTemp("", "gni-*")
	if err != nil {
		return nil, err
	}
	sock := filepath.Join(tmp, "dbg.socket")

	c := exec.Command(path, "gdbserver", "unix://"+sock)
	c.SysProcAttr = &syscall.SysProcAttr{
		Pdeathsig: syscall.SIGTERM,
	}
	if err := c.Start(); err != nil {
		return nil, err
	}

	conn, err := tryConnect("unix", sock)
	if err != nil {
		return nil, err
	}

	lldbConn := newConn(conn)
	if err := lldbConn.handshake(); err != nil {
		conn.Close()
		return nil, err
	}

	return &LLDB{
		server:     c.Process,
		tmpDir:     tmp,
		c:          lldbConn,
		connCloser: conn,
	}, nil
}

func (l *LLDB) Run(program string, args []string) error {
	if _, err := l.c.run(program, args); err != nil {
		return err
	}
	if err := l.fetchFutures(); err != nil {
		return err
	}
	pi, err := l.c.getProcessInfo()
	if err != nil {
		return err
	}
	pi, err = l.c.getProcessInfoPID(pi.pid)
	if err != nil {
		return err
	}
	return l.readImage(pi.name)
}

func (l *LLDB) Detach() error {
	if err := l.connCloser.Close(); err != nil {
		return err
	}
	return nil
}

func (l *LLDB) SetBreakpoint(bp *dbg.Breakpoint) (*dbg.Breakpoint, error) {
	addr, filePath, err := l.sym.LineToPC(bp.File, bp.Line)
	if err != nil {
		return nil, err
	}
	if err := l.c.insertBreakpoint(addr, l.bpKind); err != nil {
		return nil, err
	}
	l.bpCnt++
	newBp := &dbg.Breakpoint{
		ID:   l.bpCnt,
		Func: bp.Func,
		File: filePath,
		Line: bp.Line,
	}
	l.bps[newBp.ID] = newBp
	return newBp, nil
}

func (l *LLDB) Continue() error {
	_, err := l.c.exec("c")
	return err
}

func (l *LLDB) readImage(filename string) error {
	f, err := openFile(l.c, filename)
	if err != nil {
		return err
	}
	defer f.close()

	elfFile, err := elf.NewFile(f)
	if err != nil {
		return err
	}

	dwarf, err := elfFile.DWARF()
	if err != nil {
		return err
	}
	return l.sym.LoadImage(dwarf)
}

func (l *LLDB) fetchFutures() error {
	target, err := l.c.readTargetFeatures()
	if err != nil {
		return err
	}
	switch {
	case strings.Contains(target.Arch, "x86_64"):
		l.arch = dbg.ArchAMD64
		l.bpKind = 1
	case strings.Contains(target.Arch, "aarch64"):
		l.arch = dbg.ArchARM64
		l.bpKind = 4
	default:
		return fmt.Errorf("unsupported architecture: %s", target.Arch)
	}
	return nil
}

func tryConnect(network, address string) (conn net.Conn, err error) {
	for i := time.Duration(100); i < 5000; i += 100 {
		conn, err = net.Dial(network, address)
		if err == nil {
			return
		}
		time.Sleep(i * time.Millisecond)
	}
	return
}
