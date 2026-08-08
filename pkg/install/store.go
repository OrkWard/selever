package install

import (
	"os"
	"path/filepath"
)

// Paths resolves the XDG directories used by selever.
type Paths struct {
	Data   string // immutable installations
	Cache  string // downloaded archives and metadata
	Config string // configuration files
}

// ResolvePaths returns the XDG-based Paths, creating directories as needed.
func ResolvePaths() (*Paths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	data := envOr("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	cache := envOr("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	config := envOr("XDG_CONFIG_HOME", filepath.Join(home, ".config"))

	p := &Paths{
		Data:   filepath.Join(data, "selever"),
		Cache:  filepath.Join(cache, "selever"),
		Config: filepath.Join(config, "selever"),
	}

	for _, d := range []string{p.Data, p.Cache, p.Config} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return nil, err
		}
	}

	return p, nil
}

// InstallDir returns the directory where a toolchain or package is installed.
// e.g. <data>/selever/node/22.14.0
func (p *Paths) InstallDir(tool, version string) string {
	return filepath.Join(p.Data, tool, version)
}

// BinDir returns the bin directory inside an installation.
func (p *Paths) BinDir(tool, version string) string {
	return filepath.Join(p.InstallDir(tool, version), "bin")
}

// CacheDir returns the cache directory for a tool.
func (p *Paths) CacheDir(tool string) string {
	return filepath.Join(p.Cache, tool)
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
