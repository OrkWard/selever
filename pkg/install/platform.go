package install

import "runtime"

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
