package lldb

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

type MockServer struct {
	input  bytes.Buffer
	output *bytes.Buffer
}

func (s *MockServer) Read(data []byte) (int, error) {
	return s.input.Read(data)
}

func (s *MockServer) Write(data []byte) (int, error) {
	return s.output.Write(data)
}

func (s *MockServer) Append(data string) {
	s.input.WriteString(data)
}

var recvTests = []struct {
	input    string
	want     []byte
	hasError bool
	hasACK   bool
	wantOut  []byte
}{
	{
		input: "$test#c0",
		want:  []byte("test"),
	},
	{
		input:    "$test#XX",
		hasError: true,
	},
	{
		input:    "$test#c1",
		hasError: true,
	},
	{
		input: "%test#c0$test#c0",
		want:  []byte("test"),
	},
	{
		input:   "$test#c0",
		want:    []byte("test"),
		wantOut: []byte{'+'},
		hasACK:  true,
	},
	{
		input:   "$test#c1$test#c0",
		want:    []byte("test"),
		wantOut: []byte("-+"),
		hasACK:  true,
	},
	{
		input: "$test}]}\x03#1a",
		want:  []byte("test}#"),
	},
}

func TestConnSend(t *testing.T) {
	ms := &MockServer{}
	c := newConn(ms)

	for i, test := range recvTests {
		ms.Append(test.input)
		c.ack = test.hasACK

		ms.output = &bytes.Buffer{}
		resp, err := c.recv(test.input)

		assert.Equal(t, test.want, resp, "test #%d", i)
		fmt.Println(ms.output.Bytes())
		assert.Equal(t, test.wantOut, ms.output.Bytes(), "test #%d", i)
		if test.hasError {
			assert.NotNil(t, err, "test #%d", i)
		} else {
			assert.Nil(t, err, "test #%d", i)
		}
	}
}
