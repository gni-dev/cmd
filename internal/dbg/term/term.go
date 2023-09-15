package term

import (
	"bufio"
	"bytes"
	"container/list"
	"fmt"
	"io"
	"strings"
)

const (
	keyCtrlC     = 3
	keyEscape    = 27
	keyBackspace = 127
)

var (
	escRed   = []byte{keyEscape, '[', '3', '1', 'm'}
	escReset = []byte{keyEscape, '[', '0', 'm'}

	keyUp     = []byte{keyEscape, '[', 'A'}
	keyDown   = []byte{keyEscape, '[', 'B'}
	keyLeft   = []byte{keyEscape, '[', 'D'}
	keyRight  = []byte{keyEscape, '[', 'C'}
	keyHome   = []byte{keyEscape, '[', 'H'}
	keyEnd    = []byte{keyEscape, '[', 'F'}
	keyDelete = []byte{keyEscape, '[', '3', '~'}

	crlf = []byte{'\r', '\n'}
)

type Term struct {
	rw          io.ReadWriter
	prompt      string
	line        string
	cmd         *Commands
	r           *bufio.Reader
	history     *list.List
	historyCurr *list.Element
	pos         int
	maxWidth    int
}

func New(rw io.ReadWriter, prompt string) *Term {
	t := &Term{
		rw:      rw,
		prompt:  prompt,
		cmd:     DebuggerCommands(),
		r:       bufio.NewReaderSize(rw, 256),
		history: list.New(),
	}
	t.historyCurr = t.history.PushBack("") // dummy element
	return t
}

func (t *Term) Run(initCmd string) error {
	if initCmd != "" {
		if err := t.cmd.Process(initCmd); err != nil {
			return err
		}
	}
	for {
		line, err := t.readLine()
		if err == io.EOF {
			t.rw.Write(crlf)
			break
		}
		if err != nil {
			t.writeString(fmt.Sprintf("%sError reading line: %s%s\n", escRed, err, escReset))
			t.r.Reset(t.rw)
			continue
		}

		if line == "" {
			continue
		}

		if err := t.cmd.Process(line); err != nil {
			if err == io.EOF {
				break
			}
			t.writeString(fmt.Sprintf("%sCommand failed: %s%s\n", escRed, err, escReset))
		}
	}
	return t.cmd.Close()
}

func (t *Term) handleEscape() error {
	var seq []byte
	for {
		c, err := t.r.ReadByte()
		if err != nil {
			return err
		}
		seq = append(seq, c)
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '~' {
			break
		}
	}

	var err error
	switch {
	case bytes.Equal(seq, keyUp):
		if t.historyCurr != t.history.Front() {
			t.historyCurr = t.historyCurr.Prev()
			t.line = t.historyCurr.Value.(string)
			err = t.replaceLine()
		} else {
			t.doBeep()
		}
	case bytes.Equal(seq, keyDown):
		if t.historyCurr != t.history.Back() {
			t.historyCurr = t.historyCurr.Next()
			t.line = t.historyCurr.Value.(string)
			err = t.replaceLine()
		} else {
			t.doBeep()
		}
	case bytes.Equal(seq, keyLeft):
		err = t.moveCursor(t.pos - 1)
	case bytes.Equal(seq, keyRight):
		err = t.moveCursor(t.pos + 1)
	case bytes.Equal(seq, keyHome):
		err = t.moveCursor(0)
	case bytes.Equal(seq, keyEnd):
		err = t.moveCursor(t.maxWidth)
	case bytes.Equal(seq, keyDelete):
		err = t.eraseChar()
	default:
		_, err = t.rw.Write(seq)
	}
	return err
}

func (t *Term) appendHistory() {
	if (strings.TrimSpace(t.line)) == "" {
		return
	}
	t.historyCurr = t.history.Back()
	t.history.InsertBefore(t.line, t.history.Back())
	if t.history.Len() > 100 {
		t.history.Remove(t.history.Front())
	}
}

func (t *Term) readLine() (string, error) {
	if err := t.writeString(t.prompt); err != nil {
		return "", err
	}
	t.pos = 0
	t.maxWidth = 0
	t.line = ""
	for {
		b, err := t.r.Peek(1)
		if err != nil {
			return "", err
		}

		if b[0] == keyEscape {
			if err := t.handleEscape(); err != nil {
				return "", err
			}
			continue
		}

		r, _, err := t.r.ReadRune()
		if err != nil {
			return "", err
		}
		switch r {
		case keyCtrlC:
			return "", io.EOF
		case keyBackspace:
			t.moveCursor(t.pos - 1)
			if err := t.eraseChar(); err != nil {
				return "", err
			}
		case '\r':
		case '\n':
			if _, err := t.rw.Write(crlf); err != nil {
				return "", err
			}
			t.appendHistory()
			return t.line, nil
		default:
			if err := t.writeString(string(r)); err != nil {
				return "", err
			}
			t.line += string(r)
		}
	}
}

func (t *Term) writeString(str string) error {
	if str == "" {
		return nil
	}
	_, err := t.rw.Write([]byte(str))
	if err == nil {
		t.pos += len(str)
		if t.pos > t.maxWidth {
			t.maxWidth = t.pos
		}
	}
	return err
}

func (t *Term) replaceLine() error {
	t.moveCursor(0)
	_, err := t.rw.Write([]byte{keyEscape, '[', 'K'})
	if err != nil {
		return err
	}
	t.maxWidth = 0
	return t.writeString(t.line)
}

func (t *Term) moveCursor(pos int) error {
	if pos < 0 {
		pos = 0
	}
	if pos > t.maxWidth {
		pos = t.maxWidth
	}
	diff := pos - t.pos
	if diff == 0 {
		return nil
	}

	b := []byte{keyEscape, '['}
	if diff < 0 {
		b = append(b, []byte(fmt.Sprintf("%dD", -diff))...)
	} else {
		b = append(b, []byte(fmt.Sprintf("%dC", diff))...)
	}

	_, err := t.rw.Write(b)
	if err == nil {
		t.pos = pos
	}
	return err
}

func (t *Term) eraseChar() error {
	if t.pos == t.maxWidth || t.maxWidth == 0 {
		return nil
	}
	if _, err := t.rw.Write([]byte{keyEscape, '[', 'P'}); err != nil {
		return err
	}
	t.maxWidth = t.maxWidth - 1
	t.line = t.line[:t.pos] + t.line[t.pos+1:]
	return nil
}

func (t *Term) doBeep() error {
	_, err := t.rw.Write([]byte{'\a'})
	return err
}
