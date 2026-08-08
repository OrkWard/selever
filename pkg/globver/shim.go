package globver

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

const shimMarker = "# globver-managed"

// binDir is the path to ~/.local/bin.
func binDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "bin"), nil
}

// CreateShim writes a launcher script for exeName that prepends dirs to PATH
// and execs the target binary. Returns an error if an unmanaged file already
// exists at the target path.
func CreateShim(exeName string, dirs []string) error {
	bin, err := binDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(bin, 0o755); err != nil {
		return err
	}

	target := filepath.Join(bin, exeName)

	// Check existing file.
	if data, err := os.ReadFile(target); err == nil {
		if !bytes.Contains(data, []byte(shimMarker)) {
			return fmt.Errorf("%s exists and is not managed by globver", target)
		}
	}

	script := buildShim(exeName, dirs)
	return os.WriteFile(target, script, 0o755)
}

// RemoveShim deletes a managed launcher script. Returns an error if the
// file exists but is not managed by globver, or silently succeeds if the
// file does not exist.
func RemoveShim(exeName string) error {
	bin, err := binDir()
	if err != nil {
		return err
	}

	target := filepath.Join(bin, exeName)

	data, err := os.ReadFile(target)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	if !bytes.Contains(data, []byte(shimMarker)) {
		return fmt.Errorf("%s is not managed by globver", target)
	}

	return os.Remove(target)
}

// IsShim returns true if the file at path is a globver-managed shim.
func IsShim(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return bytes.Contains(data, []byte(shimMarker))
}

// buildShim constructs a POSIX sh launcher script.
func buildShim(exeName string, dirs []string) []byte {
	var b bytes.Buffer
	b.WriteString("#!/bin/sh\n")
	b.WriteString(shimMarker + "\n")
	for _, d := range dirs {
		fmt.Fprintf(&b, "PATH=%s:$PATH\n", d)
	}
	b.WriteString("export PATH\n")
	fmt.Fprintf(&b, "exec %q \"$@\"\n", exeName)
	return b.Bytes()
}

// ExeNames returns the sorted list of executable names from the config.
func ExeNames(cfg Config) []string {
	var names []string
	for k := range cfg {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}
