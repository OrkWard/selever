//go:build windows

package globver

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// shimExe is kiennq/scoop-better-shimexe v3.1.2, the shim used by Scoop and
// hok. It reads <name>.shim next to itself and launches the target with the
// recorded args followed by its own arguments.
//
//go:embed assets/shim.exe
var shimExe []byte

// Launchers on Windows mirror hok:
//
//   - <name>.exe + <name>.shim when the target is an executable (including a
//     pinned node.exe running an npm script);
//   - <name>.cmd when the target is a batch file.
//
// The marker is stored in the .shim (where the shim ignores unknown lines)
// or the .cmd, so a launcher is managed when that file carries it.

// CreateShim writes the launcher files for exeName. Returns an error if an
// unmanaged file already exists at one of the launcher paths.
func CreateShim(exeName string, spec ShimSpec) error {
	bin, err := binDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(bin, 0o755); err != nil {
		return err
	}

	base := filepath.Join(bin, exeName)
	if err := checkManaged(base); err != nil {
		return err
	}

	target, args, err := shimTarget(exeName, spec)
	if err != nil {
		return err
	}

	if strings.EqualFold(filepath.Ext(target), ".exe") {
		if err := removeIfExists(base + ".cmd"); err != nil {
			return err
		}
		var conf strings.Builder
		fmt.Fprintf(&conf, "path = \"%s\"\r\n", target)
		if args != "" {
			fmt.Fprintf(&conf, "args = %s\r\n", args)
		}
		conf.WriteString(shimMarker + "\r\n")
		if err := writeIfChanged(base+".exe", shimExe); err != nil {
			return err
		}
		return writeIfChanged(base+".shim", []byte(conf.String()))
	}

	for _, ext := range []string{".exe", ".shim"} {
		if err := removeIfExists(base + ext); err != nil {
			return err
		}
	}
	sep := ""
	if args != "" {
		sep = " "
	}
	script := fmt.Sprintf("@echo off\r\nrem %s\r\n\"%s\"%s%s %%*\r\n", shimMarker, target, sep, args)
	return writeIfChanged(base+".cmd", []byte(script))
}

// RemoveShim deletes the managed launcher files for exeName. Returns an error
// if one of them exists but is not managed by globver.
func RemoveShim(exeName string) error {
	bin, err := binDir()
	if err != nil {
		return err
	}

	base := filepath.Join(bin, exeName)
	if err := checkManaged(base); err != nil {
		return err
	}
	for _, ext := range []string{".exe", ".shim", ".cmd"} {
		if err := removeIfExists(base + ext); err != nil {
			return err
		}
	}
	return nil
}

// IsShim returns true if the file at path is part of a globver-managed
// launcher.
func IsShim(path string) bool {
	if strings.EqualFold(filepath.Ext(path), ".exe") {
		path = strings.TrimSuffix(path, filepath.Ext(path)) + ".shim"
	}
	return hasMarker(path)
}

// shimTarget returns the file the launcher runs and the arguments placed
// before the caller's.
func shimTarget(exeName string, spec ShimSpec) (string, string, error) {
	if spec.Interp != "" && spec.Script != "" {
		args := append([]string{}, spec.InterpArgs...)
		args = append(args, `"`+spec.Script+`"`)
		return spec.Interp, strings.Join(args, " "), nil
	}

	for _, dir := range spec.PathDirs {
		for _, ext := range []string{".exe", ".cmd", ".bat"} {
			p := filepath.Join(dir, exeName+ext)
			if info, err := os.Stat(p); err == nil && !info.IsDir() {
				return p, "", nil
			}
		}
	}
	return "", "", fmt.Errorf("no executable named %s in %s", exeName, strings.Join(spec.PathDirs, ", "))
}

// checkManaged fails if any launcher file for base exists without the marker.
// An .exe counts as managed when its .shim carries the marker.
func checkManaged(base string) error {
	for _, p := range []string{base + ".exe", base + ".shim", base + ".cmd"} {
		if _, err := os.Stat(p); err != nil {
			continue
		}
		if !IsShim(p) {
			return fmt.Errorf("%s exists and is not managed by globver", p)
		}
	}
	return nil
}

func hasMarker(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return bytes.Contains(data, []byte(shimMarker))
}

// writeIfChanged skips identical content so a running launcher exe, which
// cannot be overwritten, does not fail a re-run.
func writeIfChanged(path string, content []byte) error {
	if old, err := os.ReadFile(path); err == nil && bytes.Equal(old, content) {
		return nil
	}
	return os.WriteFile(path, content, 0o755)
}

func removeIfExists(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
