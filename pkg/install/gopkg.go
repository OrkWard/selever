package install

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

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

	pkgVersion = normalizeGopkgVersion(pkgVersion)

	installDir = gopkgInstallDir(p, pkg, pkgVersion)
	binDir = filepath.Join(installDir, "bin")

	// installDir only exists once an install finished, see the rename below.
	if _, err := os.Stat(installDir); err == nil {
		return installDir, binDir, nil
	}

	lock, err := AcquireLock(LockPath(installDir), 0)
	if err != nil {
		return "", "", err
	}
	defer lock.Release()

	if _, err := os.Stat(installDir); err == nil {
		return installDir, binDir, nil
	}

	// Build into a staging directory and rename it into place on success, so
	// a killed build cannot leave a half-populated installDir behind that the
	// check above would mistake for a finished install. go install creates
	// GOBIN itself, so staging needs no MkdirAll.
	staging := installDir + ".staging"
	os.RemoveAll(staging) // discard a previous crashed attempt

	goBin := filepath.Join(ToolBinDir("go", goDir), ExeName("go"))
	spec := pkg + "@" + pkgVersion

	// Use GOBIN to install directly into the staging bin directory.
	env := prependPath(os.Environ(), ToolBinDir("go", goDir))
	env = append(env, "GOBIN="+filepath.Join(staging, "bin"))

	cmd := exec.CommandContext(ctx, goBin, "install", spec)
	cmd.Env = env
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr

	fmt.Fprintf(os.Stderr, "Building %s with Go %s\n", spec, goVersion)
	if err := cmd.Run(); err != nil {
		os.RemoveAll(staging)
		return "", "", fmt.Errorf("go install: %w", err)
	}

	if err := os.Rename(staging, installDir); err != nil {
		os.RemoveAll(staging)
		return "", "", fmt.Errorf("install: %w", err)
	}

	return installDir, binDir, nil
}

// semverLike matches "1", "1.26", "1.26.1", "1.26.1-rc.1" and "1.26.1+meta".
var semverLike = regexp.MustCompile(`^[0-9]+(\.[0-9]+)*([-+][0-9A-Za-z.\-+]*)?$`)

// normalizeGopkgVersion adds the leading "v" Go requires on module versions,
// so "pkg@0.14.2" works like "pkg@v0.14.2". Branch names, commit hashes and
// queries such as "latest" are left untouched.
func normalizeGopkgVersion(v string) string {
	if semverLike.MatchString(v) {
		return "v" + v
	}
	return v
}

// gopkgInstallDir builds the install path for a Go package.
func gopkgInstallDir(p *Paths, pkg, pkgVersion string) string {
	sanitized := strings.TrimPrefix(pkgVersion, "v")
	ident := fmt.Sprintf("%s-%s", pkg, sanitized)
	return filepath.Join(p.Data, "gopkg", ident)
}
