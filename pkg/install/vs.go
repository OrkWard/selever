package install

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/cavaliergopher/grab/v3"
)

// MSVC host and target architectures, spelled as in Visual Studio.
var (
	vsHosts   = []string{"x64", "x86", "arm64"}
	vsTargets = []string{"x64", "x86", "arm", "arm64"}
)

// vsLockTimeout bounds the wait for a concurrent MSVC or Windows SDK install,
// which downloads hundreds of megabytes.
const vsLockTimeout = 60 * time.Minute

// NativeVSArch returns the Visual Studio name of the running architecture.
func NativeVSArch() string {
	switch runtime.GOARCH {
	case "386":
		return "x86"
	case "arm64":
		return "arm64"
	default:
		return "x64"
	}
}

// CheckVSArch validates and normalizes a host and target pair. An empty host
// selects the native architecture; an empty target selects the host.
func CheckVSArch(host, target string) (string, string, error) {
	host, target = strings.ToLower(host), strings.ToLower(target)
	if host == "" {
		host = NativeVSArch()
	}
	if target == "" {
		target = host
	}
	if !contains(vsHosts, host) {
		return "", "", fmt.Errorf("unsupported host %q (want one of %s)", host, strings.Join(vsHosts, ", "))
	}
	if !contains(vsTargets, target) {
		return "", "", fmt.Errorf("unsupported target %q (want one of %s)", target, strings.Join(vsTargets, ", "))
	}
	return host, target, nil
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func requireWindows(tool string) error {
	if runtime.GOOS != "windows" {
		return fmt.Errorf("%s is only available on Windows", tool)
	}
	return nil
}

// fetchPayloads downloads payloads into cacheDir/<sha256>/<file name>,
// verifying each checksum, and returns the local paths in order.
func fetchPayloads(ctx context.Context, cacheDir string, payloads []VSPayload) ([]string, error) {
	paths := make([]string, len(payloads))
	errs := make([]error, len(payloads))
	sem := make(chan struct{}, 6)
	var wg sync.WaitGroup

	for i, pl := range payloads {
		sum := strings.ToLower(pl.SHA256)
		paths[i] = filepath.Join(cacheDir, sum, payloadBase(pl.FileName))
		if _, err := os.Stat(paths[i]); err == nil {
			continue
		}
		wg.Add(1)
		go func(i int, pl VSPayload, sum string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			errs[i] = downloadVerified(ctx, pl.URL, sum, paths[i])
		}(i, pl, sum)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			return nil, fmt.Errorf("download %s: %w", payloadBase(payloads[i].FileName), err)
		}
	}
	return paths, nil
}

// payloadBase returns the last element of a payload file name such as
// "Installers\Windows SDK Desktop Libs x64-x86_en-us.msi".
func payloadBase(name string) string {
	if i := strings.LastIndexAny(name, `\/`); i >= 0 {
		return name[i+1:]
	}
	return name
}

func downloadVerified(ctx context.Context, url, sum, dst string) error {
	want, err := hex.DecodeString(sum)
	if err != nil || len(want) != sha256.Size {
		return fmt.Errorf("invalid sha256 %q", sum)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	part := dst + ".part"
	req, err := grab.NewRequest(part, url)
	if err != nil {
		return err
	}
	req = req.WithContext(ctx)
	req.SetChecksum(sha256.New(), want, true)

	fmt.Fprintf(os.Stderr, "Downloading %s\n", filepath.Base(dst))
	if err := grab.DefaultClient.Do(req).Err(); err != nil {
		return err
	}
	return os.Rename(part, dst)
}

// extractVSIX extracts the Contents/ tree of a VSIX (zip) payload into dest.
func extractVSIX(path, dest string) error {
	r, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		rel, ok := strings.CutPrefix(f.Name, "Contents/")
		if !ok || rel == "" || strings.HasSuffix(rel, "/") {
			continue
		}
		if u, err := url.PathUnescape(rel); err == nil {
			rel = u
		}
		if !filepath.IsLocal(filepath.FromSlash(rel)) {
			return fmt.Errorf("%s: unsafe entry %q", filepath.Base(path), f.Name)
		}
		target := filepath.Join(dest, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := writeZipFile(f, target); err != nil {
			return fmt.Errorf("%s: %w", filepath.Base(path), err)
		}
	}
	return nil
}

func writeZipFile(f *zip.File, target string) error {
	in, err := f.Open()
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(target)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// latestVersionDir returns the name of the highest-versioned subdirectory of
// dir for which ok holds.
func latestVersionDir(dir string, ok func(string) bool) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	best := ""
	for _, e := range entries {
		if !e.IsDir() || !ok(filepath.Join(dir, e.Name())) {
			continue
		}
		if best == "" || compareVersions(e.Name(), best) > 0 {
			best = e.Name()
		}
	}
	if best == "" {
		return "", fmt.Errorf("no usable version directory in %s", dir)
	}
	return best, nil
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// vsInfoFile records what an MSVC or Windows SDK installation contains. Its
// presence marks a completed installation.
const vsInfoFile = "selever.json"

type vsInfo struct {
	Version string `json:"version"` // on-disk version directory
	Host    string `json:"host"`
	Target  string `json:"target"`
}

func readVSInfo(dir string) (*vsInfo, error) {
	data, err := os.ReadFile(filepath.Join(dir, vsInfoFile))
	if err != nil {
		return nil, err
	}
	var info vsInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, err
	}
	return &info, nil
}

func writeVSInfo(dir string, info *vsInfo) error {
	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, vsInfoFile), append(data, '\n'), 0o644)
}

// finishStaging moves a completed staging directory into place.
func finishStaging(staging, dir string) error {
	if err := os.Rename(staging, dir); err != nil {
		return fmt.Errorf("install: %w", err)
	}
	return nil
}
