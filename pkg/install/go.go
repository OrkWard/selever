package install

import (
	"context"
	"fmt"
	"os"
)

// GoInstalled checks whether a Go version is already installed.
func GoInstalled(version string) (string, bool) {
	p, err := ResolvePaths()
	if err != nil {
		return "", false
	}
	dir := p.InstallDir("go", version)
	bin := p.BinDir("go", version)
	if _, err := os.Stat(bin); err == nil {
		return dir, true
	}
	return dir, false
}

// InstallGo downloads and extracts a Go release.
func InstallGo(ctx context.Context, version string) (string, error) {
	p, err := ResolvePaths()
	if err != nil {
		return "", err
	}

	dir := p.InstallDir("go", version)
	if _, err := os.Stat(p.BinDir("go", version)); err == nil {
		return dir, nil // already installed
	}

	cacheDir := p.CacheDir("go")
	platform := goPlatform()
	arch := goArch()
	archiveName := fmt.Sprintf("go%s.%s-%s.tar.gz", version, platform, arch)
	url := fmt.Sprintf("https://go.dev/dl/%s", archiveName)

	lock, err := AcquireLock(LockPath(dir), 0)
	if err != nil {
		return "", err
	}
	defer lock.Release()

	return dir, FetchExtract(ctx, url, cacheDir, dir, "")
}
