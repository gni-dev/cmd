package dap

import (
	"fmt"
	"io"
	"net"
	"os"
)

type Server struct {
	port int
}

func NewServer(port int) *Server {
	return &Server{port}
}

func (s *Server) Run() error {
	listen, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", s.port))
	if err != nil {
		return err
	}
	defer listen.Close()
	for {
		conn, err := listen.Accept()
		if err != nil {
			return err
		}
		sess := NewSession(conn)
		err = sess.Serve()
		if err == io.EOF {
			return err
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}
}
