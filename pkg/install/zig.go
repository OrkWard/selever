package install

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

const (
	// zigIndexURL lists Zig releases and the current master build.
	zigIndexURL = "https://ziglang.org/download/index.json"
	// zlsIndexURL lists tagged ZLS releases in the same format.
	zlsIndexURL = "https://releases.zigtools.org/v1/zls/index.json"
)

// zigRelease is one entry of a Zig or ZLS release index. Platform keys
// ("x86_64-windows", ...) map to artifacts; other keys are metadata.
type zigRelease map[string]json.RawMessage

// zigArtifact is a platform download of a Zig or ZLS release.
type zigArtifact struct {
	Tarball string `json:"tarball"`
	Shasum  string `json:"shasum"`
}

// version returns the "version" field. Only the Zig master entry has one;
// for tagged releases the index key is the version.
func (r zigRelease) version() string {
	var v string
	json.Unmarshal(r["version"], &v)
	return v
}

func (r zigRelease) artifact(platform string) (zigArtifact, bool) {
	var a zigArtifact
	raw, ok := r[platform]
	if !ok || json.Unmarshal(raw, &a) != nil || a.Tarball == "" {
		return a, false
	}
	return a, true
}

// fetchZigIndex downloads a Zig-format release index, keyed by version.
func fetchZigIndex(url string) (map[string]zigRelease, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s returned %s", url, resp.Status)
	}

	var index map[string]zigRelease
	if err := json.NewDecoder(resp.Body).Decode(&index); err != nil {
		return nil, fmt.Errorf("parse %s: %w", url, err)
	}
	return index, nil
}

// InstallZig downloads and extracts an exact Zig release. Development
// versions are available only while they are the current master build.
func InstallZig(ctx context.Context, version string) (string, error) {
	return installZigStyle(ctx, "zig", zigIndexURL, version, true)
}

// InstallZls downloads and extracts an exact tagged ZLS release.
func InstallZls(ctx context.Context, version string) (string, error) {
	return installZigStyle(ctx, "zls", zlsIndexURL, version, false)
}

func installZigStyle(ctx context.Context, tool, indexURL, version string, strip bool) (string, error) {
	p, err := ResolvePaths()
	if err != nil {
		return "", err
	}

	dir := p.InstallDir(tool, version)
	if toolInstalled(tool, dir) {
		return dir, nil
	}

	index, err := fetchZigIndex(indexURL)
	if err != nil {
		return "", err
	}

	release, ok := index[version]
	if !ok {
		if master, ok2 := index["master"]; ok2 && master.version() == version {
			release, ok = master, true
		}
	}
	if !ok {
		return "", fmt.Errorf("%s %s is not in %s", tool, version, indexURL)
	}

	platform := ZigPlatform()
	a, ok := release.artifact(platform)
	if !ok {
		return "", fmt.Errorf("%s %s has no build for %s", tool, version, platform)
	}

	lock, err := AcquireLock(LockPath(dir), 0)
	if err != nil {
		return "", err
	}
	defer lock.Release()

	if toolInstalled(tool, dir) {
		return dir, nil
	}

	fetch := FetchExtract
	if !strip {
		fetch = FetchExtractFlat
	}
	return dir, fetch(ctx, a.Tarball, p.CacheDir(tool), dir, a.Shasum)
}
