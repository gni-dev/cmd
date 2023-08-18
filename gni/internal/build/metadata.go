package build

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

type Metadata struct {
	AppID   string
	Name    string
	Build   int
	Version string
	Android struct {
		MinSDK    int
		TargetSDK int
	}
}

func CreateDefaults(a Args) (Metadata, error) {
	m := Metadata{Build: 1, Version: "0.0.1"}

	cmd := exec.Command(
		"go",
		"list",
		"-C", a.Chdir,
		"-f", "{{.ImportPath}}",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		os.Stderr.Write(out)
		return Metadata{}, err
	}
	importPath := string(bytes.TrimSpace(out))
	if importPath == "" {
		return Metadata{}, fmt.Errorf("failed to get import path")
	}

	parts := strings.Split(importPath, "/")
	name := parts[len(parts)-1]
	if len(parts) > 1 {
		domain := strings.Split(parts[0], ".")
		if len(domain) > 1 {
			m.AppID = domain[len(domain)-1] + "." + domain[len(domain)-2] + "." + name
		} else {
			m.AppID = domain[0] + "." + name
		}
	} else {
		m.AppID = "local." + name
	}

	re := regexp.MustCompile(`[^A-Za-z_.]`)
	m.AppID = re.ReplaceAllString(m.AppID, "_")
	m.Name = name
	return m, nil
}

func (m *Metadata) fixupAndroidVer() {
	if m.Android.MinSDK < 19 {
		m.Android.MinSDK = 19
	}
	if m.Android.TargetSDK < 31 {
		m.Android.TargetSDK = 31
	}
}
