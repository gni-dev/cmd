package lldb

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
)

const maxRetransmits = 5

type conn struct {
	remote io.ReadWriter
	br     *bufio.Reader
	ack    bool
}

func newConn(remote io.ReadWriter) *conn {
	return &conn{remote: remote, br: bufio.NewReader(remote)}
}

func (c *conn) handshake() error {
	c.ack = true

	if err := c.sendACK(true); err != nil {
		return err
	}
	if err := c.disableACK(); err != nil {
		return err
	}
	return nil
}

func (c *conn) exec(cmd string) (string, error) {
	if err := c.send(cmd); err != nil {
		return "", err
	}
	return c.recv()
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

func (c *conn) recv() (string, error) {
	for i := 0; i < maxRetransmits; i++ {
		res, err := c.br.ReadBytes('#')
		if err != nil {
			return "", err
		}

		buf := make([]byte, 2)
		if _, err := io.ReadFull(c.br, buf); err != nil {
			return "", err
		}

		if res[0] == '%' {
			continue // ignore async notifications
		}

		payload := string(res[1 : len(res)-1])
		sum, err := strconv.ParseUint(string(buf), 16, 8)
		if err != nil {
			return "", err
		}
		sumOK := (uint8(sum) == checksum(payload))

		if !c.ack {
			if sumOK {
				return payload, nil
			} else {
				return "", fmt.Errorf("checksum mismatch: %s", res)
			}
		}

		if sumOK {
			if err := c.sendACK(true); err != nil {
				return "", err
			}
			return payload, nil
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

func (c *conn) disableACK() error {
	res, err := c.exec("QStartNoAckMode")
	c.ack = (res != "OK")
	return err
}

func checksum(cmd string) uint8 {
	var sum uint8
	for _, b := range []byte(cmd) {
		sum += b
	}
	return sum
}
