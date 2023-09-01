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
	serial string
}

func AndroidDevices(androidHome string) ([]string, error) {
	adbCmd, err := makeADBCmd(androidHome)
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(adbCmd, "devices")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to run %s:\n%s", cmd, string(out))
	}

	lines := strings.Split(string(out), "\n")
	if len(lines) < 2 {
		return nil, fmt.Errorf("failed to parse adb devices output: %s", out)
	}
	devices := make([]string, 0, len(lines)-1)
	for _, l := range lines[1:] {
		line := strings.TrimSpace(l)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) != 2 {
			return nil, fmt.Errorf("failed to parse adb devices output: %s", out)
		}
		if parts[1] != "device" {
			continue
		}
		devices = append(devices, parts[0])
	}
	return devices, nil
}

func makeADBCmd(androidHome string) (string, error) {
	exe := "adb"
	if runtime.GOOS == "windows" {
		exe += ".exe"
	}
	adbCmd := filepath.Join(androidHome, "platform-tools", exe)
	if _, err := os.Stat(adbCmd); err == nil {
		return adbCmd, nil
	}
	return "", fmt.Errorf("failed to find adb")
}

func NewADB(androidHome, serial string) (*ADB, error) {
	adbCmd, err := makeADBCmd(androidHome)
	if err != nil {
		return nil, err
	}
	return &ADB{adbCmd: adbCmd, serial: serial}, nil
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
	args = append([]string{"-s", a.serial, "shell"}, args...)
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
	args = append([]string{"-s", a.serial}, args...)
	cmd := exec.Command(a.adbCmd, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to run %s:\n%s", cmd, string(out))
	}
	return nil
}
