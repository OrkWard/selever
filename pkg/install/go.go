package install

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
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

	// Fetch checksum.
	checksumURL := url + ".sha256"
	sha256, err := fetchGoChecksum(checksumURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not fetch Go checksum: %v\n", err)
		sha256 = ""
	}

	lock, err := AcquireLock(LockPath(dir), 0)
	if err != nil {
		return "", err
	}
	defer lock.Release()

	return dir, FetchExtract(ctx, url, cacheDir, dir, sha256)
}

// fetchGoChecksum fetches the .sha256 file (which is just the hex string).
func fetchGoChecksum(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GET %s: %s", url, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// File format: <sha256hex>  <filename> or just <sha256hex>
	line := strings.TrimSpace(string(body))
	parts := strings.Fields(line)
	if len(parts) > 0 && isValidSHA256(parts[0]) {
		return parts[0], nil
	}
	return "", fmt.Errorf("invalid checksum file")
}
