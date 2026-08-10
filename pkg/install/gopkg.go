package install

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// GopkgInstalled checks whether a Go package is already installed.
func GopkgInstalled(pkg, pkgVersion string) (string, bool) {
	p, err := ResolvePaths()
	if err != nil {
		return "", false
	}
	dir := gopkgInstallDir(p, pkg, pkgVersion)
	if _, err := os.Stat(dir); err == nil {
		return dir, true
	}
	return dir, false
}

// InstallGopkg builds and installs a Go package using a specific Go version.
// Returns the install directory and the bin directory path.
func InstallGopkg(ctx context.Context, pkg, pkgVersion, goVersion string) (installDir, binDir string, err error) {
	p, err := ResolvePaths()
	if err != nil {
		return "", "", err
	}

	// Ensure Go is installed.
	goDir, err := InstallGo(ctx, goVersion)
	if err != nil {
		return "", "", fmt.Errorf("go: %w", err)
	}

	installDir = gopkgInstallDir(p, pkg, pkgVersion)
	binDir = filepath.Join(installDir, "bin")

	if _, err := os.Stat(binDir); err == nil {
		return installDir, binDir, nil
	}

	lock, err := AcquireLock(LockPath(installDir), 0)
	if err != nil {
		return "", "", err
	}
	defer lock.Release()

	if _, err := os.Stat(binDir); err == nil {
		return installDir, binDir, nil
	}

	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return "", "", err
	}

	goBin := filepath.Join(goDir, "bin", "go")
	spec := pkg + "@" + pkgVersion

	// Use GOBIN to install directly into our bin directory.
	env := prependPath(os.Environ(), filepath.Join(goDir, "bin"))
	env = append(env, "GOBIN="+binDir)

	cmd := exec.CommandContext(ctx, goBin, "install", spec)
	cmd.Env = env
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr

	fmt.Fprintf(os.Stderr, "Building %s with Go %s\n", spec, goVersion)
	if err := cmd.Run(); err != nil {
		os.RemoveAll(installDir)
		return "", "", fmt.Errorf("go install: %w", err)
	}

	return installDir, binDir, nil
}

// gopkgInstallDir builds the install path for a Go package.
func gopkgInstallDir(p *Paths, pkg, pkgVersion string) string {
	sanitized := strings.TrimPrefix(pkgVersion, "v")
	ident := fmt.Sprintf("%s-%s", pkg, sanitized)
	return filepath.Join(p.Data, "gopkg", ident)
}
