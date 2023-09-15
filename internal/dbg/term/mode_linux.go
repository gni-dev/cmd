package term

import (
	"syscall"
)

const (
	ioctlGetTermios = syscall.TCGETS
	ioctlSetTermios = syscall.TCSETS
)

func setRawMode(t *syscall.Termios) {
	t.Lflag &^= syscall.ECHO | syscall.ECHONL | syscall.ICANON | syscall.ISIG
	t.Cflag |= syscall.CS8
}
