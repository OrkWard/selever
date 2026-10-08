package install

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// The Visual Studio installer manifest lists every MSVC toolset and Windows
// SDK package with its download payloads.

// vsChannelURL is the Visual Studio 2026 (18) stable channel. Its manifest
// also lists the older 14.29 (VS 2019) and 14.3x/14.4x (VS 2022) toolsets.
const vsChannelURL = "https://aka.ms/vs/18/stable/channel"

const vsManifestItemID = "Microsoft.VisualStudio.Manifests.VisualStudio"

// VSPayload is one downloadable file of a package.
type VSPayload struct {
	FileName string `json:"fileName"`
	SHA256   string `json:"sha256"`
	Size     int64  `json:"size"`
	URL      string `json:"url"`
}

// VSPackage is one installer package.
type VSPackage struct {
	ID           string                     `json:"id"`
	Version      string                     `json:"version"`
	Language     string                     `json:"language"`
	Dependencies map[string]json.RawMessage `json:"dependencies"`
	Payloads     []VSPayload                `json:"payloads"`
}

// VSManifest is the parsed Visual Studio manifest.
type VSManifest struct {
	Packages []VSPackage `json:"packages"`
	byID     map[string][]*VSPackage
}

func (m *VSManifest) index() {
	m.byID = make(map[string][]*VSPackage, len(m.Packages))
	for i := range m.Packages {
		p := &m.Packages[i]
		id := strings.ToLower(p.ID)
		m.byID[id] = append(m.byID[id], p)
	}
}

// Lookup returns the package with the given id, matched without regard to
// case. Among language variants it prefers the neutral one, then en-US.
func (m *VSManifest) Lookup(id string) *VSPackage {
	var best *VSPackage
	for _, p := range m.byID[strings.ToLower(id)] {
		switch {
		case p.Language == "":
			return p
		case strings.EqualFold(p.Language, "en-US"):
			best = p
		}
	}
	return best
}

// MSVCVersion is one MSVC toolset in the manifest.
type MSVCVersion struct {
	Version string // package id version, e.g. "14.44.17.14" or "14.51"
	Build   string // package build version, e.g. "14.44.35229"
}

var msvcToolsRe = regexp.MustCompile(`^microsoft\.vc\.(\d+\.\d+(?:\.\d+\.\d+)?)\.tools\.host([a-z0-9]+)\.target([a-z0-9]+)\.base$`)

// msvcToolsID returns the id of the compiler package for a toolset.
func msvcToolsID(version, host, target string) string {
	return fmt.Sprintf("microsoft.vc.%s.tools.host%s.target%s.base", version, host, target)
}

// MSVCVersions returns the toolsets that have a host-to-target compiler,
// newest first.
func (m *VSManifest) MSVCVersions(host, target string) []MSVCVersion {
	var out []MSVCVersion
	for _, p := range m.Packages {
		sm := msvcToolsRe.FindStringSubmatch(strings.ToLower(p.ID))
		if sm == nil || sm[2] != host || sm[3] != target {
			continue
		}
		out = append(out, MSVCVersion{Version: sm[1], Build: p.Version})
	}
	sort.Slice(out, func(i, j int) bool { return compareVersions(out[i].Version, out[j].Version) > 0 })
	return out
}

// WinSDK is one Windows SDK in the manifest.
type WinSDK struct {
	Build   string     // e.g. "26100"
	Package *VSPackage // the package holding the MSI and CAB payloads
}

var sdkComponentRe = regexp.MustCompile(`^microsoft\.visualstudio\.component\.windows1[01]sdk\.(\d+)$`)

// WinSDKs returns the Windows SDKs, newest first.
func (m *VSManifest) WinSDKs() []WinSDK {
	var out []WinSDK
	for _, p := range m.Packages {
		sm := sdkComponentRe.FindStringSubmatch(strings.ToLower(p.ID))
		if sm == nil {
			continue
		}
		for dep := range p.Dependencies {
			if sdk := m.Lookup(dep); sdk != nil && len(sdk.Payloads) > 0 {
				out = append(out, WinSDK{Build: sm[1], Package: sdk})
				break
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return compareVersions(out[i].Build, out[j].Build) > 0 })
	return out
}

// WinSDK returns the Windows SDK with the given build number, or nil.
func (m *VSManifest) WinSDK(build string) *WinSDK {
	for _, s := range m.WinSDKs() {
		if s.Build == build {
			return &s
		}
	}
	return nil
}

// compareVersions compares dotted numeric versions.
func compareVersions(a, b string) int {
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(pa) || i < len(pb); i++ {
		var va, vb int
		if i < len(pa) {
			va, _ = strconv.Atoi(pa[i])
		}
		if i < len(pb) {
			vb, _ = strconv.Atoi(pb[i])
		}
		if va != vb {
			return va - vb
		}
	}
	return 0
}

// LoadVSManifest returns the Visual Studio manifest, cached in the vs cache
// directory.
//
// A cached channel file younger than maxAge is used without network access;
// maxAge < 0 accepts any cached copy and 0 always refetches. When the network
// fails, an existing cache is used regardless of age.
func LoadVSManifest(maxAge time.Duration) (*VSManifest, error) {
	p, err := ResolvePaths()
	if err != nil {
		return nil, err
	}
	cacheDir := p.CacheDir("vs")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return nil, err
	}

	data, err := cachedChannel(filepath.Join(cacheDir, "channel.json"), maxAge)
	if err != nil {
		return nil, fmt.Errorf("Visual Studio channel: %w", err)
	}

	var channel struct {
		ChannelItems []struct {
			ID       string      `json:"id"`
			Payloads []VSPayload `json:"payloads"`
		} `json:"channelItems"`
	}
	if err := json.Unmarshal(data, &channel); err != nil {
		return nil, fmt.Errorf("parse Visual Studio channel: %w", err)
	}

	var payload *VSPayload
	for _, it := range channel.ChannelItems {
		if it.ID == vsManifestItemID && len(it.Payloads) > 0 {
			payload = &it.Payloads[0]
			break
		}
	}
	if payload == nil {
		return nil, fmt.Errorf("%s not found in Visual Studio channel", vsManifestItemID)
	}

	// The manifest is content-addressed by the hash the channel declares. The
	// served file does not match that hash or size, so the hash only names
	// the cache entry.
	manifestPath := filepath.Join(cacheDir, strings.ToLower(payload.SHA256)+".vsman")
	data, err = os.ReadFile(manifestPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Downloading %s\n", payload.URL)
		data, err = httpGet(payload.URL)
		if err != nil {
			return nil, fmt.Errorf("Visual Studio manifest: %w", err)
		}
		if err := writeAtomic(manifestPath, data); err != nil {
			return nil, err
		}
		pruneVSManifests(cacheDir, filepath.Base(manifestPath))
	}

	m := &VSManifest{}
	if err := json.Unmarshal(data, m); err != nil {
		return nil, fmt.Errorf("parse Visual Studio manifest: %w", err)
	}
	m.index()
	return m, nil
}

// loadVSManifestFor returns a manifest for which ok is true, preferring the
// cached copy and refetching only when the cache lacks what is wanted.
func loadVSManifestFor(ok func(*VSManifest) bool) (*VSManifest, error) {
	m, err := LoadVSManifest(-1)
	if err == nil && ok(m) {
		return m, nil
	}
	return LoadVSManifest(0)
}

func cachedChannel(path string, maxAge time.Duration) ([]byte, error) {
	info, statErr := os.Stat(path)
	if statErr == nil && maxAge != 0 && (maxAge < 0 || time.Since(info.ModTime()) < maxAge) {
		if data, err := os.ReadFile(path); err == nil {
			return data, nil
		}
	}

	data, err := httpGet(vsChannelURL)
	if err != nil {
		if old, rerr := os.ReadFile(path); rerr == nil {
			fmt.Fprintf(os.Stderr, "Warning: %v; using cached %s\n", err, path)
			return old, nil
		}
		return nil, err
	}
	return data, writeAtomic(path, data)
}

func httpGet(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

func writeAtomic(path string, data []byte) error {
	tmp := fmt.Sprintf("%s.%d.tmp", path, os.Getpid())
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// pruneVSManifests removes superseded manifests (about 18 MB each).
func pruneVSManifests(dir, keep string) {
	old, _ := filepath.Glob(filepath.Join(dir, "*.vsman"))
	for _, f := range old {
		if filepath.Base(f) != keep {
			os.Remove(f)
		}
	}
}
