package lldb

import (
	"bufio"
	"bytes"
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

type processInfo struct {
	name   string
	pid    int
	triple string
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
	c.ack = (string(resp) != "OK")
	return err
}

func (c *conn) getFeatures(features string) (map[string]string, error) {
	resp, err := c.exec("qSupported:" + features)
	if err != nil {
		return nil, err
	}
	stub := make(map[string]string)
	ls := strings.Split(string(resp), ";")
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

func (c *conn) qXfer(kind, annex string) ([]byte, error) {
	buf := &bytes.Buffer{}
	for {
		resp, err := c.exec(fmt.Sprintf("qXfer:%s:read:%s:%x,%x", kind, annex, buf.Len(), c.packetSize))
		if err != nil {
			return nil, err
		}
		buf.Write(resp[1:])
		if resp[0] == 'l' {
			return buf.Bytes(), nil
		}
	}
}

func (c *conn) getProcessInfo() (processInfo, error) {
	resp, err := c.exec("qProcessInfo")
	if err != nil {
		return processInfo{}, err
	}
	return parseProcessInfo(resp, 16), nil
}

func (c *conn) getProcessInfoPID(pid int) (processInfo, error) {
	resp, err := c.exec(fmt.Sprintf("qProcessInfoPID:%d", pid))
	if err != nil {
		return processInfo{}, err
	}
	return parseProcessInfo(resp, 10), nil
}

func (c *conn) stopReply(resp []byte) (stopPacket, error) {
	switch resp[0] {
	case 'T':
		return stopPacket{}, nil
	default:
		return stopPacket{}, fmt.Errorf("unknown stop reply: %s", resp)
	}
}

func (c *conn) exec(cmd string) ([]byte, error) {
	if err := c.send(cmd); err != nil {
		return nil, err
	}
	return c.recv(cmd)
}

func (c *conn) send(cmd string) error {
	p := fmt.Sprintf("$%s#%02x", cmd, checksum([]byte(cmd)))

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

func checkForErr(cmd string, resp []byte) ([]byte, error) {
	if len(resp) == 0 {
		return nil, &GeneralError{cmd: cmd, code: "0"}
	} else if resp[0] == 'E' && len(resp) == 3 {
		return nil, &GeneralError{cmd: cmd, code: string(resp)}
	} else {
		return resp, nil
	}
}

func (c *conn) recv(cmd string) ([]byte, error) {
	for attempt := 0; attempt < maxRetransmits; {
		resp, err := c.br.ReadBytes('#')
		if err != nil {
			return nil, err
		}

		buf := make([]byte, 2)
		if _, err := io.ReadFull(c.br, buf); err != nil {
			return nil, err
		}

		if resp[0] != '$' {
			continue // ignore notify and other packets
		}

		payload := resp[1 : len(resp)-1]
		sum, err := strconv.ParseUint(string(buf), 16, 8)
		if err != nil {
			return nil, err
		}
		sumOK := (uint8(sum) == checksum(payload))

		payload = unescape(payload)

		if !c.ack {
			if sumOK {
				return checkForErr(cmd, payload)
			} else {
				return nil, fmt.Errorf("checksum mismatch: %s", payload)
			}
		}

		if sumOK {
			if err := c.sendACK(true); err != nil {
				return nil, err
			}
			return checkForErr(cmd, payload)
		}
		if err := c.sendACK(false); err != nil {
			return nil, err
		}
		attempt++
	}
	return nil, fmt.Errorf("failed to recv data after %d attempts", maxRetransmits)
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

func unescape(packet []byte) []byte {
	var buf bytes.Buffer
	for i := 0; i < len(packet); i++ {
		switch c := packet[i]; c {
		case 0x7d:
			if i+1 < len(packet) {
				buf.WriteByte(packet[i+1] ^ 0x20)
				i++
			}
		default:
			buf.WriteByte(c)
		}
		// lldb should not use RLE compression
	}
	return buf.Bytes()
}

func checksum(packet []byte) uint8 {
	var sum uint8
	for _, b := range packet {
		sum += b
	}
	return sum
}

func parseProcessInfo(resp []byte, base int) processInfo {
	var i processInfo
	ls := strings.Split(string(resp), ";")
	for _, f := range ls {
		kv := strings.Split(f, ":")
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "name":
			v, _ := hex.DecodeString(kv[1])
			i.name = string(v)
		case "pid":
			v, _ := strconv.ParseInt(kv[1], base, 32)
			i.pid = int(v)
		case "triple":
			v, _ := hex.DecodeString(kv[1])
			i.triple = string(v)
		}
	}
	return i
}
