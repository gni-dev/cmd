package build

import (
	"archive/zip"
	"bytes"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"text/template"
	"time"
)

func AndroidAPK(tmpdir string, m Metadata, a Args) error {
	androidHome, err := findAndroidHome()
	if err != nil {
		return err
	}
	platform, err := findLast(filepath.Join(androidHome, "platforms"))
	if err != nil {
		return err
	}
	buildTools, err := findLast(filepath.Join(androidHome, "build-tools"))
	if err != nil {
		return err
	}

	builDir := filepath.Join(tmpdir, "build")

	if err := compileAndroid(builDir, androidHome, platform, m, a); err != nil {
		return err
	}
	if err := packAndroid(builDir, buildTools, platform, m, a, false); err != nil {
		return err
	}
	return signDebugApk(builDir, buildTools, m.Name, a.DestPath)
}

func compileAndroid(builDir, androidHome, platform string, m Metadata, a Args) error {
	ndkRoot, err := findNDK(androidHome)
	if err != nil {
		return err
	}

	arch := archMap["amd64"]

	ndkBin := filepath.Join(ndkRoot, "toolchains", "llvm", "prebuilt", runtime.GOOS+"-x86_64", "bin")
	clang, err := findNdkCompiler(ndkBin, arch.triple, m.Android.MinSDK)
	if err != nil {
		return err
	}

	lib := filepath.Join(builDir, "lib", arch.abi, "libgni.so")
	cmd := exec.Command(
		"go",
		"build",
		"-C", a.Chdir,
		"-buildmode=c-shared",
		"-o", lib,
	)
	if !a.DebugBuild {
		cmd.Args = append(cmd.Args, "-ldflags=-s -w")
	}
	cmd.Env = append(
		os.Environ(),
		"GOOS=android",
		"GOARCH=amd64",
		"GOARM=7",
		"CGO_ENABLED=1",
		"CC="+clang,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		os.Stderr.Write(out)
		return err
	}

	javaC, err := findJavaCompiler()
	if err != nil {
		return err
	}

	cmd = exec.Command(
		"go",
		"list",
		"-C", a.Chdir,
		"-f", "{{.Dir}}",
		"gni.dev/gni/internal/backend",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		os.Stderr.Write(out)
		return err
	}
	androidSrcPath := string(bytes.TrimSpace(out))
	androidSrc, err := filepath.Glob(filepath.Join(androidSrcPath, "*.java"))
	if err != nil {
		return err
	}
	if len(androidSrc) == 0 {
		return fmt.Errorf("no java files foundc at %s", androidSrcPath)
	}
	cmd = exec.Command(
		javaC,
		"-target", "1.8",
		"-source", "1.8",
		"-Xlint:-options",
		"-sourcepath", androidSrcPath,
		"-bootclasspath", filepath.Join(platform, "android.jar"),
		"-d", filepath.Join(builDir, "classes"),
	)
	cmd.Args = append(cmd.Args, androidSrc...)
	if out, err := cmd.CombinedOutput(); err != nil {
		os.Stderr.Write(out)
		return err
	}
	return nil
}

func packAndroid(builDir, buildTools, platform string, m Metadata, a Args, bundle bool) error {
	var classes []string
	filepath.WalkDir(filepath.Join(builDir, "classes"), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if filepath.Ext(d.Name()) == ".class" {
			classes = append(classes, path)
		}
		return nil
	})
	cmd := exec.Command(
		filepath.Join(buildTools, "d8"),
		"--lib", filepath.Join(platform, "android.jar"),
		"--output", builDir,
		"--min-api", strconv.Itoa(m.Android.MinSDK),
	)
	cmd.Args = append(cmd.Args, classes...)
	if out, err := cmd.CombinedOutput(); err != nil {
		os.Stderr.Write(out)
		return err
	}

	manifest := filepath.Join(builDir, "AndroidManifest.xml")
	fm := template.FuncMap{
		"debuggable": func() bool {
			return a.DebugBuild
		},
	}
	tmpl, _ := template.New("manifest").Funcs(fm).Parse(androidManifest)
	f, err := os.Create(manifest)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := tmpl.Execute(f, m); err != nil {
		return err
	}

	baseAPK := filepath.Join(builDir, "base.apk")
	cmd = exec.Command(
		filepath.Join(buildTools, "aapt2"),
		"link",
		"--manifest", manifest,
		"-o", baseAPK,
		"-I", filepath.Join(platform, "android.jar"),
	)
	if bundle {
		cmd.Args = append(cmd.Args, "--proto-format")
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		os.Stderr.Write(out)
		return err
	}

	baseZip, err := zip.OpenReader(baseAPK)
	if err != nil {
		return err
	}
	defer baseZip.Close()

	f, err = os.Create(filepath.Join(builDir, "app.zip"))
	if err != nil {
		return err
	}
	defer f.Close()

	appZip := zip.NewWriter(f)
	for _, f := range baseZip.File {
		path := f.Name
		if path == "AndroidManifest.xml" && bundle {
			path = "manifest/AndroidManifest.xml"
		}
		w, err := appZip.Create(path)
		if err != nil {
			return err
		}
		r, err := f.Open()
		if err != nil {
			return err
		}
		if _, err := io.Copy(w, r); err != nil {
			return err
		}
	}

	dex := "classes.dex"
	if bundle {
		dex = "dex/classes.dex"
	}
	if err := addToZip(appZip, filepath.Join(builDir, "classes.dex"), dex); err != nil {
		return err
	}

	for _, a := range archMap {
		libPath := filepath.Join("lib", a.abi, "libgni.so")
		libFullPath := filepath.Join(builDir, libPath)
		if _, err := os.Stat(libFullPath); errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err := addToZip(appZip, libFullPath, libPath); err != nil {
			return err
		}
	}

	return appZip.Close()
}

func findJavaCompiler() (string, error) {
	javaHome := os.Getenv("JAVA_HOME")
	if javaHome == "" {
		return exec.LookPath("javac")
	}
	javac := filepath.Join(javaHome, "bin", "javac")
	if _, err := os.Stat(javac); err != nil {
		return "", err
	}
	return javac, nil
}

func findAndroidHome() (string, error) {
	androidHome := os.Getenv("ANDROID_HOME")
	if androidHome == "" {
		androidHome = os.Getenv("ANDROID_SDK_ROOT")
		if androidHome == "" {
			return "", fmt.Errorf("ANDROID_HOME is not set")
		}
	}
	return androidHome, nil
}

func findLast(path string) (string, error) {
	dir, err := os.Open(path)
	if err != nil {
		return "", err
	}
	children, err := dir.Readdirnames(-1)
	if err != nil {
		return "", err
	}
	if len(children) == 0 {
		return "", fmt.Errorf("%w in %s", os.ErrNotExist, path)
	}
	return filepath.Join(path, children[len(children)-1]), nil
}

func findNDK(androidHome string) (string, error) {
	ndkRoot, err := findLast(filepath.Join(androidHome, "ndk"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			ndkRoot = os.Getenv("ANDROID_NDK_ROOT")
			if ndkRoot == "" {
				return "", fmt.Errorf("no NDK found. Please set ANDROID_NDK_ROOT")
			}
		} else {
			return "", err
		}
	}
	return ndkRoot, nil
}

func findNdkCompiler(ndkBin, triple string, minSDK int) (string, error) {
	comps, err := filepath.Glob(filepath.Join(ndkBin, triple+"*-clang"))
	if err != nil {
		return "", err
	}

	if len(comps) == 0 {
		return "", fmt.Errorf("no compilers found for architecture %s in %s", triple, ndkBin)
	}

	suitableCompiler := comps[0]
	for _, c := range comps {
		patt := filepath.Join(ndkBin, triple+"%d-clang")
		var ver int
		if n, err := fmt.Sscanf(c, patt, &ver); n < 1 || err != nil {
			continue
		}
		if ver > minSDK {
			break
		}
		suitableCompiler = c
	}
	return suitableCompiler, nil
}

func addToZip(z *zip.Writer, fileName, path string) error {
	src, err := os.Open(fileName)
	if err != nil {
		return err
	}
	defer src.Close()
	w, err := z.Create(filepath.ToSlash(path))
	if err != nil {
		return err
	}
	_, err = io.Copy(w, src)
	return err
}

func signDebugApk(builDir, buildTools, name, dst string) error {
	src := filepath.Join(builDir, "app.apk")
	st, err := os.Stat(dst)
	if err != nil {
		return err
	}
	if st.Mode().IsDir() {
		dst = filepath.Join(dst, name+".apk")
	}

	cmd := exec.Command(
		filepath.Join(buildTools, "zipalign"),
		"-p", "4",
		filepath.Join(builDir, "app.zip"),
		src,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		os.Stderr.Write(out)
		return err
	}

	certPEM := filepath.Join(builDir, "cert.pem")
	keyPEM := filepath.Join(builDir, "key.pem")

	block, _ := pem.Decode([]byte(debugCert))
	if block == nil {
		panic("invalid debug certificate")
	}
	pk, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return err
	}

	ca := &x509.Certificate{
		SerialNumber: big.NewInt(2023),
		Subject: pkix.Name{
			CommonName: "Android Debug",
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().AddDate(10, 0, 0),
		KeyUsage:  x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
	}
	der, err := x509.CreateCertificate(rand.Reader, ca, ca, &pk.PublicKey, pk)
	if err != nil {
		return err
	}
	if err := os.WriteFile(certPEM, der, 0644); err != nil {
		return err
	}

	der, err = x509.MarshalPKCS8PrivateKey(pk)
	if err != nil {
		return err
	}
	if err := os.WriteFile(keyPEM, der, 0644); err != nil {
		return err
	}

	cmd = exec.Command(
		filepath.Join(buildTools, "apksigner"),
		"sign",
		"--cert", certPEM,
		"--key", keyPEM,
		"--out", dst,
		src,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		os.Stderr.Write(out)
		return err
	}
	return nil
}

const androidManifest = `<?xml version="1.0" encoding="utf-8"?>
<manifest xmlns:android="http://schemas.android.com/apk/res/android"
	package="{{.AppID}}"
	android:versionCode="{{.Build}}"
	android:versionName="{{.Version}}" >

	<uses-sdk
		android:minSdkVersion="{{.Android.MinSDK}}"
		android:targetSdkVersion="{{.Android.TargetSDK}}" />

	<application
		android:label="{{.Name}}"
		android:debuggable="{{debuggable}}" >
		<activity
			android:name="dev.gni.GniActivity"
			android:theme="@android:style/Theme.NoTitleBar.Fullscreen"
			android:label="{{.Name}}" >
			<intent-filter>
				<action android:name="android.intent.action.MAIN" />
				<category android:name="android.intent.category.LAUNCHER" />
			</intent-filter>
		</activity>
	</application>
</manifest>`

const debugCert = `-----BEGIN RSA PRIVATE KEY-----
MIIEogIBAAKCAQEAsntq5mmR2BV1CXypkk7EmVQVFwW4BioBlO9nTFLS6Vc0LWUU
1mcxjNdJWuzZI7J3GUx94paPvRfeo0aX1e/pw+tMfjo0LUTCWQBbznusx/3TqpOz
RrCp88nJ8hiFRiLke7u7zv2rkaI/VJ5JF684Y5sbhcHaybgwGs9w2ncZKHf7evy7
26ADt6PjEOAnqJN6MJH9/ePQMPFfO/GsJUQ/5aDIqpT8EPHIhUoIfLQkhE4LLqjJ
axsBR5rhGxnUwtIbHd7IO/l+bKq++dqEQv12YUSxVcLQ5RqYXH172MfC4F7QxrLA
FpE6we33jOkg3iAdqPpT4zFg7sfNuemcvx0q/wIDAQABAoIBAAcngVJ5GtqBia52
q8lslN7civfgR88fce7JZeeeTkwCLdo/+gTaIBdYLd2SLuYKalG+SjGB/YMD6O28
j6uIsWMkFG3e4WaLIgs1Q3jUZkmh+BEXWJFV1YorJYgpyXXVQjlffhi+/FibG1TF
/4IOiQEdH45OBfoeAvegJxLqwTxo+a2k7AXIA+9GnKMp1cSu6drP8Yi8G7phy29X
WLIpr7YLbBKCdn+TcJazuTAwdKXtzsE2KtRLF31qhmEqGOcMoEMCl8hH5i6YQ1aV
BNYHl0RV+vp2Vgf79sEwm2kq+2lRJaPFQ0MSRBTa1KHcr4NX67Axt5/rle2pbMK9
Z31JPkkCgYEA1gsV/Q9W5LAwYa4YRm7ZmdpfV0umN9BQtvaUv8KG+zZscqQ0Ui9L
JsMreqFnH1SstUYVeLKxY8XqzTbIZf+ob2DElMjDoqZ6HelsALuwBLb64AIh1EO9
sy6HVAkgNgrSDrPVzNaGDak815H1vFcSkzvjruwRxg85kjHCQFjoD8sCgYEA1XfV
bodJotmYvVbYjOGQsz6JittmL8QrinAqJga1rfTFO22GR0I6vTmo2ILy5S9kEhTs
L3H21z6kHH9pTSDCouf+i6d5cYGKxsrRYmQU5PSFVCiGh6jd8aNaX2EbwEIm1H/r
LLZ9qhzLGLOWMcDlSnzu5xJKmFDB7+UaxIFsAx0CgYB3DHVvae+3hHN0cONZkWAA
HaA3qoDJvFiYWu+C9Iwk/zE0VjYvm9Rdu+Hb9BeqKmtg65kXp7PYPYWKHDU73gVt
5VGRO1Tsi1GSf3its7aD+M3yd90e9Yp2NaPZTrYWuM/6k3WP16V5xa5sa+dUmM1h
DMdnTC/ajC9GK9zR82EnHwKBgEKS/+5bpPxz7m3GYvz08CLmsxCqQiFNheLD/nEj
kI+zEbvp+YHJxvXywJTdqhEOCaCWA978JOaWM6prlhSmzezue3VkgryCkRxUbp7H
5bhOBjLr/KDcanOM5Ydviq8YMnH9fwPP2jsuhayrfYEAzsG/WuaXzsnYDdPaWNHG
J0CFAoGASBf/vbUJfFDAhNtAnMIzkbV+Zn4G3mArn8IgEompPNpbGuyRHui6lIg8
LSmNh9Qk0+bf6ump3VWKdz+fQ9fzhxRs3DjC2Y1K2+MFj+BABYY4T8W/g9sHCJTZ
CfJQeMyjJ4JlUzBwExOT6T8jqhU1hhf+AOHzl0nqJeH9Fqvnjpw=
-----END RSA PRIVATE KEY-----`
