package lldb

import (
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

type LLDB struct {
	server     *os.Process
	tmpDir     string
	c          *conn
	connCloser io.Closer
}

func LaunchServer() (*LLDB, error) {
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

func (l *LLDB) Detach() error {
	if err := l.connCloser.Close(); err != nil {
		return err
	}
	return nil
}

func (l *LLDB) Run(program string, args []string) error {
	_, err := l.c.run(program, args)
	return err
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
