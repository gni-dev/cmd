package lldb

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"io"
	"strconv"
	"strings"
)

const maxRetransmits = 5

type GeneralError struct {
	cmd  string
	code string
}

func (err *GeneralError) Error() string {
	cmd := err.cmd
	if len(cmd) > 10 {
		cmd = cmd[:10] + "..."
	}
	if err.code == "" {
		return fmt.Sprintf("lldb: %s failed", cmd)
	} else {
		return fmt.Sprintf("lldb: %s failed with code %s", cmd, err.code)
	}
}

type conn struct {
	remote     io.ReadWriter
	br         *bufio.Reader
	ack        bool
	packetSize int
}

type stopPacket struct {
}

func newConn(remote io.ReadWriter) *conn {
	return &conn{remote: remote, br: bufio.NewReader(remote)}
}

func (c *conn) handshake() error {
	c.ack = true
	c.packetSize = 256

	if err := c.sendACK(true); err != nil {
		return err
	}
	if err := c.disableACK(); err != nil {
		return err
	}
	stub, err := c.getFeatures("xmlRegisters=i386;multiprocess+")
	if err != nil {
		return err
	}
	if v, ok := stub["PacketSize"]; ok {
		if i, err := strconv.Atoi(v); err == nil {
			c.packetSize = i
		}
	}
	return nil
}

func (c *conn) disableACK() error {
	resp, err := c.exec("QStartNoAckMode")
	c.ack = (resp != "OK")
	return err
}

func (c *conn) getFeatures(features string) (map[string]string, error) {
	resp, err := c.exec("qSupported:" + features)
	if err != nil {
		return nil, err
	}
	stub := make(map[string]string)
	ls := strings.Split(resp, ";")
	for _, f := range ls {
		kv := strings.Split(f, "=")
		if len(kv) == 2 {
			stub[kv[0]] = kv[1]
		} else if len(f) > 0 {
			stub[f[:len(f)-1]] = f[len(f)-1:]
		}
	}
	return stub, nil
}

func (c *conn) run(program string, args []string) (stopPacket, error) {
	params := hex.EncodeToString([]byte(program))
	for _, arg := range args {
		params += ";" + hex.EncodeToString([]byte(arg))
	}
	resp, err := c.exec("vRun;" + params)
	if err != nil {
		return stopPacket{}, err
	}
	return c.stopReply(resp)
}

func (c *conn) stopReply(resp string) (stopPacket, error) {
	switch resp[0] {
	case 'T':
		return stopPacket{}, nil
	default:
		return stopPacket{}, fmt.Errorf("unknown stop reply: %s", resp)
	}
}

func (c *conn) exec(cmd string) (string, error) {
	if err := c.send(cmd); err != nil {
		return "", err
	}
	return c.recv(cmd)
}

func (c *conn) send(cmd string) error {
	p := fmt.Sprintf("$%s#%02x", cmd, checksum(cmd))

	for i := 0; i < maxRetransmits; i++ {
		if _, err := c.remote.Write([]byte(p)); err != nil {
			return err
		}

		if !c.ack {
			return nil
		}

		ok, err := c.recvACK()
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
	}
	return fmt.Errorf("failed to send %s after %d attempts", cmd, maxRetransmits)
}

func checkForErr(cmd string, resp string) (string, error) {
	if len(resp) == 0 {
		return "", &GeneralError{}
	} else if resp[0] == 'E' && len(resp) == 3 {
		return "", &GeneralError{code: resp}
	} else {
		return resp, nil
	}
}

func (c *conn) recv(cmd string) (string, error) {
	for i := 0; i < maxRetransmits; i++ {
		resp, err := c.br.ReadBytes('#')
		if err != nil {
			return "", err
		}

		buf := make([]byte, 2)
		if _, err := io.ReadFull(c.br, buf); err != nil {
			return "", err
		}

		if resp[0] == '%' {
			continue // ignore async notifications
		}

		payload := string(resp[1 : len(resp)-1])
		sum, err := strconv.ParseUint(string(buf), 16, 8)
		if err != nil {
			return "", err
		}
		sumOK := (uint8(sum) == checksum(payload))

		if !c.ack {
			if sumOK {
				return checkForErr(cmd, payload)
			} else {
				return "", fmt.Errorf("checksum mismatch: %s", resp)
			}
		}

		if sumOK {
			if err := c.sendACK(true); err != nil {
				return "", err
			}
			return checkForErr(cmd, payload)
		}
		if err := c.sendACK(false); err != nil {
			return "", err
		}
	}
	return "", fmt.Errorf("failed to recv data after %d attempts", maxRetransmits)
}

func (c *conn) sendACK(ack bool) error {
	var err error
	if ack {
		_, err = c.remote.Write([]byte{'+'})
	} else {
		_, err = c.remote.Write([]byte{'-'})
	}
	return err
}

func (c *conn) recvACK() (bool, error) {
	b, err := c.br.ReadByte()
	if b != '+' && b != '-' {
		return false, fmt.Errorf("invalid ack byte: %c", b)
	}
	return b == '+', err
}

func checksum(cmd string) uint8 {
	var sum uint8
	for _, b := range []byte(cmd) {
		sum += b
	}
	return sum
}
