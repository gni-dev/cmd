package run

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type ADB struct {
	adbCmd string
}

func NewADB(androidHome string) (*ADB, error) {
	exe := "adb"
	if runtime.GOOS == "windows" {
		exe += ".exe"
	}
	adbCmd := filepath.Join(androidHome, "platform-tools", exe)
	if _, err := os.Stat(adbCmd); err == nil {
		return &ADB{adbCmd: adbCmd}, nil
	}
	return nil, fmt.Errorf("failed to find adb")
}

func (a *ADB) Install(fileName string, replace bool) error {
	args := []string{"install"}
	if replace {
		args = append(args, "-r")
	}
	args = append(args, fileName)
	return a.call(args...)
}

func (a *ADB) Uninstall(pkg string) error {
	return a.call("uninstall", pkg)
}

func (a *ADB) Push(local, remote string) error {
	return a.call("push", local, remote)
}

func (a *ADB) Pull(remote, local string) error {
	return a.call("pull", remote, local)
}

func (a *ADB) Shell(args ...string) (string, error) {
	args = append([]string{"shell"}, args...)
	cmd := exec.Command(a.adbCmd, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to run %s:\n%s", cmd, string(out))
	}
	return strings.TrimSpace(string(out)), nil
}

func (a *ADB) Forward(local, remote string) error {
	return a.call("forward", local, remote)
}

func (a *ADB) ForwardRemove(local string) error {
	return a.call("forward", "--remove", local)
}

func (a *ADB) GetProp(prop string) (string, error) {
	output, err := a.Shell("getprop", prop)
	if err != nil {
		return "", err
	}
	values := strings.Split(output, "\n")
	if len(values) != 1 {
		return "", fmt.Errorf("to many lines in output %s", output)
	}
	return values[0], nil
}

func (a *ADB) SetProp(prop, value string) error {
	return a.call("shell", "setprop", prop, value)
}

func (a *ADB) RunAs(pkg string, args ...string) (string, error) {
	args = append([]string{"run-as", pkg}, args...)
	return a.Shell(args...)
}

func (a *ADB) call(args ...string) error {
	cmd := exec.Command(a.adbCmd, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		os.Stderr.Write(out)
		return err
	}
	return nil
}
