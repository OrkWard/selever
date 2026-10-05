package install

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// nodeArch maps Go's runtime.GOARCH to Node.js's arch string.
func nodeArch() string {
	switch runtime.GOARCH {
	case "amd64":
		return "x64"
	case "arm64":
		return "arm64"
	default:
		return runtime.GOARCH
	}
}

// nodePlatform maps Go's runtime.GOOS to Node.js's platform string.
func nodePlatform() string {
	switch runtime.GOOS {
	case "darwin":
		return "darwin"
	case "linux":
		return "linux"
	case "windows":
		return "win"
	default:
		return runtime.GOOS
	}
}

// goArch maps Go's runtime.GOARCH to Go's download arch.
func goArch() string {
	switch runtime.GOARCH {
	case "amd64":
		return "amd64"
	case "arm64":
		return "arm64"
	default:
		return runtime.GOARCH
	}
}

// goPlatform maps Go's runtime.GOOS to Go's download platform.
func goPlatform() string {
	switch runtime.GOOS {
	case "darwin":
		return "darwin"
	case "linux":
		return "linux"
	default:
		return runtime.GOOS
	}
}

// archiveExt is the extension of Node.js and Go release archives.
func archiveExt() string {
	if runtime.GOOS == "windows" {
		return ".zip"
	}
	return ".tar.gz"
}

// ExeName appends the platform's executable suffix to name.
func ExeName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

// ToolBinDir returns the executable directory inside a toolchain
// installation. Node.js on Windows ships its executables at the archive root.
func ToolBinDir(tool, installDir string) string {
	if tool == "node" && runtime.GOOS == "windows" {
		return installDir
	}
	return filepath.Join(installDir, "bin")
}

// toolInstalled reports whether a toolchain's main executable exists.
func toolInstalled(tool, installDir string) bool {
	info, err := os.Stat(filepath.Join(ToolBinDir(tool, installDir), ExeName(tool)))
	return err == nil && !info.IsDir()
}

// prependPath returns a copy of env with dir prepended to PATH. Windows
// environment names are case-insensitive and usually spelled "Path".
func prependPath(env []string, dir string) []string {
	out := make([]string, 0, len(env)+1)
	found := false
	for _, e := range env {
		k, v, ok := strings.Cut(e, "=")
		if !found && ok && isPathKey(k) {
			out = append(out, k+"="+dir+string(os.PathListSeparator)+v)
			found = true
		} else {
			out = append(out, e)
		}
	}
	if !found {
		out = append(out, "PATH="+dir)
	}
	return out
}

func isPathKey(k string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(k, "PATH")
	}
	return k == "PATH"
}
