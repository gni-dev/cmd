package dap

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type message interface {
	seq() int
}

type baseMessage struct {
	Seq  int    `json:"seq"`
	Type string `json:"type"`
}

func (m *baseMessage) seq() int { return m.Seq }

type request struct {
	baseMessage

	Command   string          `json:"command"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

type event struct {
	baseMessage

	Event string                 `json:"event"`
	Body  map[string]interface{} `json:"body,omitempty"`
}

type response struct {
	baseMessage

	RequestSeq int                    `json:"request_seq"`
	Success    bool                   `json:"success"`
	Command    string                 `json:"command"`
	Message    string                 `json:"message,omitempty"`
	Body       map[string]interface{} `json:"body,omitempty"`
}

type errorMessage struct {
	Id            int               `json:"id"`
	Format        string            `json:"format"`
	Variables     map[string]string `json:"variables,omitempty"`
	SendTelemetry bool              `json:"sendTelemetry,omitempty"`
	ShowUser      bool              `json:"showUser"`
	Url           string            `json:"url,omitempty"`
	UrlLabel      string            `json:"urlLabel,omitempty"`
}

var dapSeq int

func readHeader(r *bufio.Reader) (int64, error) {
	header, err := r.ReadString('\n')
	if err != nil {
		return 0, err
	}
	// skeep the trailing newline
	if _, err := r.ReadBytes('\n'); err != nil {
		return 0, err
	}
	arr := strings.Split(header, ":")
	if len(arr) != 2 {
		return 0, fmt.Errorf("invalid header: %s", header)
	}
	cl := strings.TrimSpace(arr[0])
	if !strings.EqualFold(cl, "Content-Length") {
		return 0, fmt.Errorf("invalid header: %s", header)
	}
	len := strings.TrimSpace(arr[1])
	return strconv.ParseInt(len, 10, 64)
}

func readMessage(r *bufio.Reader) (message, error) {
	len, err := readHeader(r)
	if err != nil {
		return nil, err
	}
	body := make([]byte, len)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, err
	}

	var m baseMessage
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, err
	}
	switch m.Type {
	case "request":
		return readRequest(body)
	case "event":
		return readEvent(body)
	case "response":
		return readResponse(body)
	default:
		return nil, fmt.Errorf("unknown message type: %s", body)
	}
}

func writeMessage(w io.Writer, m message) error {
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	if _, err := w.Write([]byte(fmt.Sprintf("Content-Length: %d\r\n\r\n", len(b)))); err != nil {
		return err
	}
	_, err = w.Write(b)
	return err
}

func readRequest(body []byte) (message, error) {
	var m request
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

func readEvent(body []byte) (message, error) {
	var m event
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

func readResponse(body []byte) (message, error) {
	var m response
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

func newEvent(name string, info map[string]interface{}) *event {
	dapSeq++
	return &event{
		baseMessage: baseMessage{
			Seq:  dapSeq,
			Type: "event",
		},
		Event: name,
		Body:  info,
	}
}

func newResponse(req *request, result map[string]interface{}) *response {
	dapSeq++
	return &response{
		baseMessage: baseMessage{
			Seq:  dapSeq,
			Type: "response",
		},
		RequestSeq: req.Seq,
		Success:    true,
		Command:    req.Command,
		Body:       result,
	}
}

func newErrResponse(req message, id int, cmd, msg, details string, show bool) *response {
	dapSeq++
	e := errorMessage{
		Id:       id,
		Format:   details,
		ShowUser: show,
	}
	return &response{
		baseMessage: baseMessage{
			Seq:  dapSeq,
			Type: "response",
		},
		RequestSeq: req.seq(),
		Success:    false,
		Command:    cmd,
		Message:    msg,
		Body:       map[string]interface{}{"error": e},
	}
}
