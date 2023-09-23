package lldb

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"strconv"
)

type vFile struct {
	c  *conn
	fd int
}

func openFile(c *conn, filename string) (*vFile, error) {
	encFilename := hex.EncodeToString([]byte(filename))
	resp, err := c.exec(fmt.Sprintf("vFile:open:%s,0,0", encFilename))
	if err != nil {
		return nil, err
	}
	fd, err := parseFileResp(resp, nil)
	if err != nil {
		return nil, err
	}
	return &vFile{c: c, fd: fd}, nil
}

func (f *vFile) close() error {
	_, err := f.c.exec(fmt.Sprintf("vFile:close:%x", f.fd))
	return err
}

func (f *vFile) ReadAt(p []byte, off int64) (n int, err error) {
	size := len(p)
	resp, err := f.c.exec(fmt.Sprintf("vFile:pread:%x,%x,%x", f.fd, size, off))
	if err != nil {
		return 0, err
	}
	return parseFileResp(resp, p)
}

func parseFileResp(resp []byte, p []byte) (int, error) {
	if len(resp) < 2 || resp[0] != 'F' {
		return 0, fmt.Errorf("unexpected file response: %s", resp)
	}
	if len(resp) > 2 && resp[1] == '-' && resp[2] == '1' {
		return 0, fmt.Errorf("file operation failed: %s", resp)
	}
	idx := bytes.IndexByte(resp, ';')
	if idx == -1 {
		n, _ := strconv.ParseInt(string(resp[1:]), 16, 64)
		return int(n), nil
	}
	n, _ := strconv.ParseInt(string(resp[1:idx]), 16, 64)
	if len(resp)-idx-1 != int(n) {
		return 0, fmt.Errorf("unexpected file len: %s", resp)
	}
	copy(p, resp[idx+1:])
	return int(n), nil
}
