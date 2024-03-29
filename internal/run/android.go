package run

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"

	"gni.dev/cmd/internal/build"
)

const mainActivity = "dev.gni.GniActivity"

func runAndroid(m build.Metadata, a *Args) error {
	fmt.Printf("Building package %s...\n", m.Name)
	a.buildArgs.WaitDebugger(a.wait)
	if err := build.BuildAndroid(m, a.buildArgs); err != nil {
		return err
	}
	apk := filepath.Join(a.buildArgs.OutDir(), m.Name+".apk")

	androidHome, err := build.FindAndroidHome()
	if err != nil {
		return err
	}

	devices, err := AndroidDevices(androidHome)
	if err != nil {
		return err
	}
	if len(devices) == 0 {
		return errors.New("no android devices found")
	}

	fmt.Printf("Connecting to %s...\n", devices[0])
	adb, err := NewADB(androidHome, devices[0])
	if err != nil {
		return err
	}

	if pid, err := adb.RunAs(m.AppID, "pidof", "gdbserver"); err == nil {
		adb.RunAs(m.AppID, "kill", pid)
	}
	if pid, err := adb.RunAs(m.AppID, "pidof", m.AppID); err == nil {
		adb.RunAs(m.AppID, "kill", pid)
	}
	adb.Uninstall(m.AppID)
	if err := adb.Install(apk, true); err != nil {
		return err
	}

	if !a.buildArgs.DebugBuild() {
		_, err := adb.Shell("am", "start", fmt.Sprintf("%s/%s", m.AppID, mainActivity))
		return err
	}

	dbgDir := filepath.Join(a.buildArgs.OutDir(), "dbg")
	os.MkdirAll(dbgDir, 0755)
	dataDir, err := adb.RunAs(m.AppID, "pwd")
	if err != nil {
		return err
	}
	adb.RunAs(m.AppID, "chmod", "a+x", dataDir)

	if err := installGDBServer(dbgDir, dataDir, m.AppID, adb); err != nil {
		return err
	}
	if err := adb.Pull("/system/bin/app_process", dbgDir); err != nil {
		return err
	}

	intent := fmt.Sprintf("%s/%s", m.AppID, mainActivity)
	fmt.Printf("Launching activity %s...\n", intent)
	if _, err := adb.Shell("am", "start", "-W", intent); err != nil {
		return err
	}

	pid, err := adb.RunAs(m.AppID, "pidof", m.AppID)
	if err != nil {
		return err
	}

	debugSocket := path.Join(dataDir, "debug.sock")
	adb.RunAs(m.AppID, "rm", debugSocket)

	if err := adb.Forward("tcp:5039", "localfilesystem:"+debugSocket); err != nil {
		return err
	}
	fmt.Printf("Starting gdbserver...\n")
	if _, err := adb.RunAs(m.AppID, path.Join(dataDir, "gdbserver"), "--once", "--attach", "+"+debugSocket, pid); err != nil {
		return err
	}
	return adb.ForwardRemove("tcp:5039")
}

func installGDBServer(localFolder, remoteFolder, appID string, adb *ADB) error {
	localGDB := filepath.Join(localFolder, "gdbserver")
	if _, err := os.Stat(localGDB); errors.Is(err, os.ErrNotExist) {
		abi, err := adb.GetProp("ro.product.cpu.abi")
		if err != nil {
			return err
		}
		arch := abiToArch(abi)
		if err := fetchGDBServer(arch, localGDB); err != nil {
			return err
		}
	}

	if err := adb.Push(localGDB, "/data/local/tmp/gdbserver"); err != nil {
		return err
	}
	if _, err := adb.RunAs(appID, "cp", "/data/local/tmp/gdbserver", remoteFolder); err != nil {
		return err
	}
	if _, err := adb.RunAs(appID, "chmod", "+x", path.Join(remoteFolder, "gdbserver")); err != nil {
		return err
	}
	return nil
}

func abiToArch(abi string) string {
	switch abi {
	case "arm64-v8a":
		return "aarch64"
	case "armeabi-v7a":
		return "arm"
	case "x86":
		return "i686"
	case "x86_64":
		return "x86_64"
	default:
		return ""
	}
}

func fetchGDBServer(arch, path string) error {
	url := fmt.Sprintf("https://github.com/gni-dev/tools/releases/latest/download/gdbserver-%s-android.zip", arch)
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	archive, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		return err
	}

	if len(archive.File) != 1 || archive.File[0].Name != "gdbserver" {
		return fmt.Errorf("unexpected archive %s", url)
	}
	f := archive.File[0]

	dstFile, err := os.Create(path)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	arcFile, err := f.Open()
	if err != nil {
		return err
	}
	defer arcFile.Close()

	if _, err := io.Copy(dstFile, arcFile); err != nil {
		return err
	}
	return nil
}
