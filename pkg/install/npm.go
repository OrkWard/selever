package install

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// NpmInstalled checks whether an npm package is already installed.
func NpmInstalled(pkg, pkgVersion string) (string, bool) {
	p, err := ResolvePaths()
	if err != nil {
		return "", false
	}
	dir := npmInstallDir(p, pkg, pkgVersion)
	if _, err := os.Stat(dir); err == nil {
		return dir, true
	}
	return dir, false
}

// InstallNpm installs an npm package using a specific Node.js version.
// Returns the install directory and the bin directory path.
func InstallNpm(ctx context.Context, pkg, pkgVersion, nodeVersion string) (installDir, binDir string, err error) {
	p, err := ResolvePaths()
	if err != nil {
		return "", "", err
	}

	// Ensure Node.js is installed.
	nodeDir, err := InstallNode(ctx, nodeVersion)
	if err != nil {
		return "", "", fmt.Errorf("node: %w", err)
	}

	installDir = npmInstallDir(p, pkg, pkgVersion)
	binDir = filepath.Join(installDir, "node_modules", ".bin")

	if _, err := os.Stat(binDir); err == nil {
		return installDir, binDir, nil
	}

	lock, err := AcquireLock(LockPath(installDir), 0)
	if err != nil {
		return "", "", err
	}
	defer lock.Release()

	// Double-check after acquiring lock.
	if _, err := os.Stat(binDir); err == nil {
		return installDir, binDir, nil
	}

	// Create a minimal package.json so npm doesn't walk up the tree.
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		return "", "", err
	}

	pkgJSON := filepath.Join(installDir, "package.json")
	if err := os.WriteFile(pkgJSON, []byte(`{"private":true}`+"\n"), 0o644); err != nil {
		return "", "", err
	}

	// Prepare environment: put the correct node at the front of PATH.
	nodeBin := filepath.Join(nodeDir, "bin")
	env := prependPath(os.Environ(), nodeBin)

	spec := pkg + "@" + pkgVersion

	cmd := exec.CommandContext(ctx, "npm", "install", "--no-save", "--prefix", installDir, spec)
	cmd.Env = env
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr

	fmt.Fprintf(os.Stderr, "Installing %s with Node.js %s\n", spec, nodeVersion)
	if err := cmd.Run(); err != nil {
		os.RemoveAll(installDir)
		return "", "", fmt.Errorf("npm install: %w", err)
	}

	// Clean up package.json and lock file to keep the install dir minimal.
	os.Remove(pkgJSON)
	os.Remove(filepath.Join(installDir, "package-lock.json"))

	return installDir, binDir, nil
}

// npmInstallDir builds the install path for an npm package.
// Scoped packages (e.g. @angular/cli) use their native directory structure.
func npmInstallDir(p *Paths, pkg, pkgVersion string) string {
	ident := fmt.Sprintf("%s-%s", pkg, pkgVersion)
	return filepath.Join(p.Data, "npm", ident)
}

// prependPath returns a copy of env with dir prepended to PATH.
func prependPath(env []string, dir string) []string {
	out := make([]string, 0, len(env)+1)
	prefix := "PATH="
	found := false
	for _, e := range env {
		if !found && len(e) >= len(prefix) && e[:len(prefix)] == prefix {
			out = append(out, prefix+dir+":"+e[len(prefix):])
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
