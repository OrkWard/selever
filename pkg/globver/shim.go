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

// ShimSpec describes how a launcher should invoke its target.
//
// PathDirs are prepended to PATH and therefore inherited by every child
// process. A pinned language runtime must never go here: a tool such as
// `pnpm run build` would then leak that runtime into the project's own
// scripts, which must keep resolving whatever node the environment provides.
// Instead, Interp names the interpreter to exec directly, so only the tool
// itself is pinned.
type ShimSpec struct {
	PathDirs   []string // dirs prepended to PATH (inherited by children)
	Interp     string   // absolute interpreter path; empty means PATH lookup
	InterpArgs []string // interpreter flags taken from the script's shebang
	Script     string   // absolute script path, passed to the interpreter
}

// CreateShim writes a launcher script for exeName. Returns an error if an
// unmanaged file already exists at the target path.
func CreateShim(exeName string, spec ShimSpec) error {
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

	return os.WriteFile(target, buildShim(exeName, spec), 0o755)
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
func buildShim(exeName string, spec ShimSpec) []byte {
	var b bytes.Buffer
	b.WriteString("#!/bin/sh\n")
	b.WriteString(shimMarker + "\n")
	for _, d := range spec.PathDirs {
		fmt.Fprintf(&b, "PATH='%s':$PATH\n", d)
	}
	if len(spec.PathDirs) > 0 {
		b.WriteString("export PATH\n")
	}

	// Single-quoted so paths with spaces work; a literal ' is unsupported.
	b.WriteString("exec ")
	if spec.Interp != "" && spec.Script != "" {
		fmt.Fprintf(&b, "'%s'", spec.Interp)
		for _, a := range spec.InterpArgs {
			fmt.Fprintf(&b, " '%s'", a)
		}
		fmt.Fprintf(&b, " '%s'", spec.Script)
	} else {
		fmt.Fprintf(&b, "'%s'", exeName)
	}
	b.WriteString(" \"$@\"\n")
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
