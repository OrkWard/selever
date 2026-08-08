package install

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolvePaths(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")

	p, err := ResolvePaths()
	if err != nil {
		t.Fatal(err)
	}

	if want := filepath.Join(home, ".local", "share", "selever"); p.Data != want {
		t.Errorf("Data = %q, want %q", p.Data, want)
	}
	if want := filepath.Join(home, ".cache", "selever"); p.Cache != want {
		t.Errorf("Cache = %q, want %q", p.Cache, want)
	}
	if want := filepath.Join(home, ".config", "selever"); p.Config != want {
		t.Errorf("Config = %q, want %q", p.Config, want)
	}
}

func TestResolvePathsXDG(t *testing.T) {
	dataDir := t.TempDir()
	cacheDir := t.TempDir()
	configDir := t.TempDir()

	t.Setenv("XDG_DATA_HOME", dataDir)
	t.Setenv("XDG_CACHE_HOME", cacheDir)
	t.Setenv("XDG_CONFIG_HOME", configDir)

	p, err := ResolvePaths()
	if err != nil {
		t.Fatal(err)
	}

	if want := filepath.Join(dataDir, "selever"); p.Data != want {
		t.Errorf("Data = %q, want %q", p.Data, want)
	}
	if want := filepath.Join(cacheDir, "selever"); p.Cache != want {
		t.Errorf("Cache = %q, want %q", p.Cache, want)
	}
	if want := filepath.Join(configDir, "selever"); p.Config != want {
		t.Errorf("Config = %q, want %q", p.Config, want)
	}
}

func TestInstallDir(t *testing.T) {
	p := &Paths{Data: "/data/selever"}
	dir := p.InstallDir("node", "22.14.0")
	if dir != "/data/selever/node/22.14.0" {
		t.Errorf("InstallDir = %q", dir)
	}
}

func TestBinDir(t *testing.T) {
	p := &Paths{Data: "/data/selever"}
	bin := p.BinDir("go", "1.21.0")
	if bin != "/data/selever/go/1.21.0/bin" {
		t.Errorf("BinDir = %q", bin)
	}
}

func TestResolvePathsCreatesDirs(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")

	_, err := ResolvePaths()
	if err != nil {
		t.Fatal(err)
	}

	for _, sub := range []string{".local/share/selever", ".cache/selever", ".config/selever"} {
		info, err := os.Stat(filepath.Join(home, sub))
		if err != nil {
			t.Errorf("expected dir %s to exist: %v", sub, err)
		} else if !info.IsDir() {
			t.Errorf("%s is not a directory", sub)
		}
	}
}
