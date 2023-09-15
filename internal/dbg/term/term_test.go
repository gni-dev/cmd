package term

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

type MockTerminal struct {
	input     io.Reader
	chunkSize int
	output    *bytes.Buffer
}

func NewMockTerminal(input string, ch int) *MockTerminal {
	return &MockTerminal{
		input:     strings.NewReader(input),
		chunkSize: ch,
		output:    &bytes.Buffer{},
	}
}

func (c *MockTerminal) Read(data []byte) (int, error) {
	b := make([]byte, c.chunkSize)
	_, err := c.input.Read(b)
	if err != nil {
		return 0, err
	}
	return copy(data, b), nil
}

func (c *MockTerminal) Write(data []byte) (int, error) {
	return c.output.Write(data)
}

var inputTests = []struct {
	input      string
	want       string
	err        error
	skeepLines int
}{
	{
		input: "hello\n",
		want:  "hello",
	},
	{
		input: "hello\r\n",
		want:  "hello",
	},
	{
		input: "aabb\x1b[D\x1b[D\177\n", // backspace
		want:  "abb",
	},
	{
		input: "a\177\x1b[C\177\n", // backspace
		want:  "",
	},
	{
		input: strings.Repeat("x", 200) + "\n",
		want:  strings.Repeat("x", 200),
	},
}

func TestInput(t *testing.T) {
	for i, test := range inputTests {
		for j := 1; j < len(test.input); j++ {
			screen := NewMockTerminal(test.input, j)
			tt := New(screen, "> ")
			for k := 0; k < test.skeepLines; k++ {
				_, err := tt.readLine()
				if err != nil {
					t.Errorf("line skeep %d failed %v", i, err)
				}
			}
			line, err := tt.readLine()
			if line != test.want {
				t.Errorf("line resulting %d failed %s != %s", i, line, test.want)
			}
			if err != test.err {
				t.Errorf("error resulting %d failed %v != %v", i, err, test.err)
			}
		}
	}
}

var renderTests = []struct {
	input string
	want  string
	err   error
}{
	{
		input: "hello\n",
		want:  "> hello\r\n",
	},
	{
		input: "hello\r\n",
		want:  "> hello\r\n",
	},
}

func TestRender(t *testing.T) {
	for i, test := range renderTests {
		for j := 1; j < len(test.input); j++ {
			screen := NewMockTerminal(test.input, j)
			tt := New(screen, "> ")
			_, err := tt.readLine()
			if err != test.err {
				t.Errorf("error resulting %d failed %v != %v", i, err, test.err)
			}
			if screen.output.String() != test.want {
				t.Errorf("line resulting %d failed %s != %s", i, screen.output.String(), test.want)
			}
		}
	}
}
