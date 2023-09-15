//go:build darwin || freebsd || linux || netbsd || openbsd

package term

import (
	"syscall"
	"unsafe"
)

type State struct {
	t  syscall.Termios
	fd int
}

func IsTerminal(fd int) bool {
	_, err := getTermios(fd)
	return err == nil
}

func TerminalMode(fd int) (*State, error) {
	t, err := getTermios(fd)
	if err != nil {
		return nil, err
	}
	origState := &State{t: t, fd: fd}

	setRawMode(&t)
	if err := setTermios(fd, t); err != nil {
		return nil, err
	}

	return origState, nil
}

func (s *State) Restore() error {
	return setTermios(s.fd, s.t)
}

func getTermios(fd int) (syscall.Termios, error) {
	var res syscall.Termios
	_, _, err := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), ioctlGetTermios, uintptr(unsafe.Pointer(&res)))
	if err == 0 {
		return res, nil
	}
	return res, err
}

func setTermios(fd int, termios syscall.Termios) error {
	_, _, err := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), ioctlSetTermios, uintptr(unsafe.Pointer(&termios)))
	if err == 0 {
		return nil
	}
	return err
}
