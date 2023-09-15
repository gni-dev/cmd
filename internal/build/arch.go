package build

type targetArch struct {
	abi    string
	triple string
}

var archMap = map[string]targetArch{
	"386":   {"x86", "i686-linux-android"},
	"amd64": {"x86_64", "x86_64-linux-android"},
	"arm":   {"armeabi-v7a", "armv7a-linux-androideabi"},
	"arm64": {"arm64-v8a", "aarch64-linux-android"},
}
