package install

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

// NodeInstalled checks whether a Node.js version is already installed.
func NodeInstalled(version string) (string, bool) {
	p, err := ResolvePaths()
	if err != nil {
		return "", false
	}
	dir := p.InstallDir("node", version)
	bin := p.BinDir("node", version)
	if _, err := os.Stat(bin); err == nil {
		return dir, true
	}
	return dir, false
}

// InstallNode downloads and extracts a Node.js release.
func InstallNode(ctx context.Context, version string) (string, error) {
	p, err := ResolvePaths()
	if err != nil {
		return "", err
	}

	dir := p.InstallDir("node", version)
	if _, err := os.Stat(p.BinDir("node", version)); err == nil {
		return dir, nil // already installed
	}

	cacheDir := p.CacheDir("node")
	platform := nodePlatform()
	arch := nodeArch()
	archiveName := fmt.Sprintf("node-v%s-%s-%s.tar.gz", version, platform, arch)
	url := fmt.Sprintf("https://nodejs.org/dist/v%s/%s", version, archiveName)

	// Fetch checksum.
	checksumURL := fmt.Sprintf("https://nodejs.org/dist/v%s/SHASUMS256.txt", version)
	sha256, err := fetchNodeChecksum(checksumURL, archiveName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not fetch checksum: %v\n", err)
		sha256 = ""
	}

	lock, err := AcquireLock(LockPath(dir), 0)
	if err != nil {
		return "", err
	}
	defer lock.Release()

	return dir, FetchExtract(ctx, url, cacheDir, dir, sha256)
}

// fetchNodeChecksum fetches SHASUMS256.txt and finds the line for archiveName.
func fetchNodeChecksum(url, archiveName string) (string, error) {
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

	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasSuffix(line, "  "+archiveName) {
			hash := strings.Fields(line)[0]
			if !isValidSHA256(hash) {
				return "", fmt.Errorf("invalid checksum in SHASUMS256.txt")
			}
			return hash, nil
		}
	}

	return "", fmt.Errorf("checksum not found for %s", archiveName)
}
