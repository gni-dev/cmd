package dap

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"gni.dev/cmd/internal/dbg"
	"gni.dev/cmd/internal/dbg/lldb"
)

type Session struct {
	rw       io.ReadWriter
	handlers map[string]func(*request)
	d        dbg.Debugger
}

func NewSession(rw io.ReadWriter) *Session {
	s := &Session{rw: rw}
	s.handlers = map[string]func(*request){
		"initialize":              s.onInitialize,
		"disconnect":              s.onDisconnect,
		"launch":                  s.onLaunch,
		"setBreakpoints":          s.onSetBreakpoints,
		"configurationDone":       s.onConfigurationDone,
		"setExceptionBreakpoints": s.setExceptionBreakpoints,
		"threads":                 s.onThreads,
		"pause":                   s.onPause,
		"stackTrace":              s.onStackTrace,
		"scopes":                  s.onScopes,
		"variables":               s.onVariables,
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
			s.replyErr(m, processingErr, fmt.Sprintf("unsupported command '%s'", r.Command), false)
			continue
		}
		fn(r)
		if r.Command == "disconnect" {
			return io.EOF
		}
	}
}

func (s *Session) onInitialize(req *request) {
	resp := map[string]any{
		"supportsConfigurationDoneRequest": true,
	}
	s.reply(newResponse(req, resp))
}

func (s *Session) onDisconnect(req *request) {
	if s.d != nil {
		s.d.Detach()
	}
	s.reply(newResponse(req, nil))
}

func (s *Session) onLaunch(req *request) {
	if s.d != nil {
		s.replyErr(req, launchErr, "debugger already running", true)
		return
	}
	args := struct {
		Program string   `json:"program"`
		Args    []string `json:"args"`
	}{}
	err := json.Unmarshal(req.Arguments, &args)
	if err != nil {
		s.replyErr(req, parseErr, err.Error(), false)
		return
	}

	s.d, err = lldb.LaunchServer()
	if err != nil {
		s.replyErr(req, launchErr, err.Error(), true)
		return
	}
	if err := s.d.Run(args.Program, args.Args); err != nil {
		s.replyErr(req, launchErr, err.Error(), true)
		return
	}
	s.reply(newEvent("initialized", nil))
	s.reply(newResponse(req, nil))
}

type breakpoint struct {
	ID       int    `json:"id,omitempty"`
	Verified bool   `json:"verified"`
	Message  string `json:"message,omitempty"`
	Source   string `json:"source,omitempty"`
	Line     int    `json:"line,omitempty"`
}

func (s *Session) onSetBreakpoints(req *request) {
	if s.d == nil {
		s.replyErr(req, setBreakpointsErr, "debugger not running", true)
		return
	}

	args := struct {
		Source struct {
			Path string `json:"path"`
		} `json:"source"`
		Breakpoints []struct {
			Line int `json:"line"`
		} `json:"breakpoints"`
	}{}
	err := json.Unmarshal(req.Arguments, &args)
	if err != nil {
		s.replyErr(req, parseErr, err.Error(), false)
		return
	}

	var got []breakpoint
	for _, inBp := range args.Breakpoints {
		bp := &dbg.Breakpoint{File: args.Source.Path, Line: inBp.Line}

		bp, err := s.d.SetBreakpoint(bp)
		resp := breakpoint{}
		if err != nil {
			resp.Verified = false
			resp.Message = err.Error()
		} else {
			resp.Verified = true
			resp.ID = bp.ID
			resp.Source = bp.File
			resp.Line = bp.Line
		}
		got = append(got, resp)
	}
	s.reply(newResponse(req, map[string]any{"breakpoints": got}))
}

func (s *Session) onConfigurationDone(req *request) {
	s.reply(newResponse(req, nil))
}

func (s *Session) setExceptionBreakpoints(req *request) {
	s.reply(newResponse(req, nil)) // just ignore
}

func (s *Session) onThreads(req *request) {
	resp := map[string]any{
		"threads": []map[string]any{
			{
				"id":   1,
				"name": "main",
			},
		},
	}
	s.reply(newResponse(req, resp))
}

func (s *Session) onPause(req *request) {
	s.reply(newResponse(req, nil))
	s.reply(newEvent("stopped", map[string]any{
		"reason":            "pause",
		"threadId":          1,
		"allThreadsStopped": true,
	}))
}

func (s *Session) onStackTrace(req *request) {
	resp := map[string]any{
		"stackFrames": []map[string]any{
			{
				"id":     1,
				"name":   "main",
				"source": map[string]any{"name": "main.go", "path": "./main.go"},
				"line":   10,
				"column": 0,
			},
		},
	}
	s.reply(newResponse(req, resp))
}

func (s *Session) onScopes(req *request) {
	resp := map[string]any{
		"scopes": []map[string]any{
			{
				"name":               "Local",
				"variablesReference": 1000,
				"expensive":          false,
			},
		},
	}
	s.reply(newResponse(req, resp))
}

func (s *Session) onVariables(req *request) {
	resp := map[string]any{
		"variables": []map[string]any{
			{
				"name":               "a",
				"value":              "1",
				"type":               "int",
				"variablesReference": 1001,
			},
		},
	}
	s.reply(newResponse(req, resp))
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
