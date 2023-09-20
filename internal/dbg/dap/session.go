package dap

import (
	"bufio"
	"fmt"
	"io"
	"os"
)

type Session struct {
	rw       io.ReadWriter
	handlers map[string]func(*request)
}

func NewSession(rw io.ReadWriter) *Session {
	s := &Session{rw: rw}
	s.handlers = map[string]func(*request){
		"initialize": s.onInitialize,
		"disconnect": s.onDisconnect,
	}
	return s
}

func (s *Session) Serve() error {
	r := bufio.NewReader(s.rw)
	for {
		m, err := readMessage(r)
		if err != nil {
			return err
		}
		r, ok := m.(*request)
		if !ok {
			s.replyErr(m, processingErr, "only requests are allowed", false)
			return io.EOF
		}

		fn, ok := s.handlers[r.Command]
		if !ok {
			s.replyErr(m, processingErr, "unknown command", false)
			continue
		}
		fn(r)
		if r.Command == "disconnect" {
			return io.EOF
		}
	}
}

func (s *Session) onInitialize(req *request) {
	resp := map[string]interface{}{
		"supportsFunctionBreakpoints": true,
	}
	s.reply(newResponse(req, resp))
}

func (s *Session) onDisconnect(req *request) {
	s.reply(newResponse(req, nil))
}

func (s *Session) reply(m message) {
	if err := writeMessage(s.rw, m); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}

func (s *Session) replyErr(incoming message, e gniDAPError, details string, show bool) {
	cmd := "unknown"
	req, ok := incoming.(*request)
	if ok {
		cmd = req.Command
	}

	resp := newErrResponse(incoming, int(e), cmd, e.String(), details, show)
	if err := writeMessage(s.rw, resp); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}
