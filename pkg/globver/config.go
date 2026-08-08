package globver

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/orkward/selever/pkg/install"
)

// Config is the global.json configuration.
type Config map[string][]string // executable name → selector args

// ConfigPath returns the path to global.json.
func ConfigPath() (string, error) {
	p, err := install.ResolvePaths()
	if err != nil {
		return "", err
	}
	return filepath.Join(p.Config, "global.json"), nil
}

// LoadConfig reads the global configuration.
// Returns an empty config if the file does not exist.
func LoadConfig() (Config, error) {
	path, err := ConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Config{}, nil
		}
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return cfg, nil
}

// SaveConfig atomically writes the configuration under lock.
func SaveConfig(cfg Config) error {
	path, err := ConfigPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	lock, err := install.AcquireLock(install.LockPath(path), 10*time.Second)
	if err != nil {
		return fmt.Errorf("config lock: %w", err)
	}
	defer lock.Release()

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}

	return os.Rename(tmp, path)
}
