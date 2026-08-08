package install

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

	sha256, err := fetchGoChecksum(version, archiveName)
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

// goRelease is a single release from https://go.dev/dl/?mode=json.
type goRelease struct {
	Version string `json:"version"`
	Files   []struct {
		Filename string `json:"filename"`
		SHA256   string `json:"sha256"`
	} `json:"files"`
}

// fetchGoChecksum fetches the SHA256 for a specific Go archive from the JSON API.
func fetchGoChecksum(version, archiveName string) (string, error) {
	resp, err := http.Get("https://go.dev/dl/?mode=json&include=all")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("go download index returned %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var releases []goRelease
	if err := json.Unmarshal(body, &releases); err != nil {
		return "", fmt.Errorf("parse go download index: %w", err)
	}

	fullVersion := "go" + version
	for _, r := range releases {
		if r.Version != fullVersion {
			continue
		}
		for _, f := range r.Files {
			if f.Filename == archiveName {
				return f.SHA256, nil
			}
		}
	}

	return "", fmt.Errorf("checksum not found for %s in go %s", archiveName, version)
}
