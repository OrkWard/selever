package globver

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/orkward/selever/pkg/install"
)

// NpmShimSpec builds the launcher spec for an executable in an npm package's
// node_modules/.bin directory.
//
// When the target is a Node script, the shim execs the pinned interpreter
// directly rather than exporting its bin directory on PATH. That keeps the
// pin on the tool alone: a nested `pnpm run build` still resolves node from
// the project environment, as it should.
//
// Targets that are not Node scripts (shell or native wrappers) fall back to a
// plain PATH-based launcher.
func NpmShimSpec(exe, binDir, nodeBinDir string) ShimSpec {
	spec := ShimSpec{PathDirs: []string{binDir}}
	if nodeBinDir == "" {
		return spec
	}

	script := npmBinScript(binDir, exe)
	if script == "" {
		return spec
	}
	args, ok := nodeShebang(script)
	if !ok {
		return spec
	}

	spec.Interp = filepath.Join(nodeBinDir, install.ExeName("node"))
	spec.InterpArgs = args
	spec.Script = script
	return spec
}

// npmBinScript returns the script behind node_modules/.bin/<exe>, or "" when
// it cannot be found.
//
// On Unix the .bin entry is a symlink to the script. On Windows npm writes
// cmd-shim wrappers instead, so the script is looked up in the "bin" field of
// the installed packages' package.json.
func npmBinScript(binDir, exe string) string {
	if runtime.GOOS != "windows" {
		return filepath.Join(binDir, exe)
	}

	modules := filepath.Dir(binDir)
	for _, pkgDir := range npmPackageDirs(modules) {
		if rel, ok := packageBin(pkgDir, exe); ok {
			return filepath.Join(pkgDir, filepath.FromSlash(rel))
		}
	}
	return ""
}

// npmPackageDirs lists the package directories directly under a
// node_modules directory, descending into @scope directories.
func npmPackageDirs(modules string) []string {
	entries, err := os.ReadDir(modules)
	if err != nil {
		return nil
	}
	var dirs []string
	for _, e := range entries {
		name := e.Name()
		if !e.IsDir() || strings.HasPrefix(name, ".") {
			continue
		}
		if !strings.HasPrefix(name, "@") {
			dirs = append(dirs, filepath.Join(modules, name))
			continue
		}
		scoped, err := os.ReadDir(filepath.Join(modules, name))
		if err != nil {
			continue
		}
		for _, s := range scoped {
			if s.IsDir() {
				dirs = append(dirs, filepath.Join(modules, name, s.Name()))
			}
		}
	}
	return dirs
}

// packageBin returns the script path, relative to pkgDir, that the package
// declares for executable exe.
func packageBin(pkgDir, exe string) (string, bool) {
	bins, err := packageBins(pkgDir)
	if err != nil {
		return "", false
	}
	rel, ok := bins[exe]
	return rel, ok
}

// packageBins returns the executables a package declares in the "bin" field
// of its package.json, mapped to script paths relative to pkgDir.
func packageBins(pkgDir string) (map[string]string, error) {
	path := filepath.Join(pkgDir, "package.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var pkg struct {
		Name string          `json:"name"`
		Bin  json.RawMessage `json:"bin"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	bins := map[string]string{}
	if len(pkg.Bin) == 0 {
		return bins, nil
	}

	// "bin": "cli.js" is named after the package, without its scope.
	var single string
	if json.Unmarshal(pkg.Bin, &single) == nil {
		if single != "" {
			bins[pkg.Name[strings.LastIndex(pkg.Name, "/")+1:]] = single
		}
		return bins, nil
	}
	var m map[string]string
	if err := json.Unmarshal(pkg.Bin, &m); err != nil {
		return nil, fmt.Errorf("parse %s: bin: %w", path, err)
	}
	for name, rel := range m {
		if rel != "" {
			bins[name] = rel
		}
	}
	return bins, nil
}

// npmPackageExes returns the sorted executable names that pkg itself
// declares, excluding those its dependencies add to node_modules/.bin.
func npmPackageExes(binDir, pkg string) ([]string, error) {
	pkgDir := filepath.Join(filepath.Dir(binDir), filepath.FromSlash(pkg))
	bins, err := packageBins(pkgDir)
	if err != nil {
		return nil, err
	}
	if len(bins) == 0 {
		return nil, fmt.Errorf("%s declares no executables", pkg)
	}
	names := make([]string, 0, len(bins))
	for name := range bins {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

// nodeShebang reports whether path starts with a shebang naming node, and
// returns the interpreter flags that follow it.
//
// Recognised forms:
//
//	#!/usr/bin/env node
//	#!/usr/bin/env -S node --enable-source-maps
//	#!/usr/local/bin/node --max-old-space-size=4096
func nodeShebang(path string) ([]string, bool) {
	line, ok := readShebang(path)
	if !ok {
		return nil, false
	}

	fields := strings.Fields(line)
	if len(fields) == 0 {
		return nil, false
	}

	if filepath.Base(fields[0]) == "env" {
		fields = stripEnvArgs(fields[1:])
	}

	if len(fields) == 0 || filepath.Base(fields[0]) != "node" {
		return nil, false
	}
	return fields[1:], true
}

// stripEnvArgs drops the options `env` consumes itself, leaving the
// interpreter as the first element.
func stripEnvArgs(fields []string) []string {
	for len(fields) > 0 {
		switch {
		case fields[0] == "-S" || fields[0] == "--split-string":
			fields = fields[1:]
		case strings.HasPrefix(fields[0], "-S"):
			// Joined form: `-Snode --flag`.
			return append([]string{strings.TrimPrefix(fields[0], "-S")}, fields[1:]...)
		case strings.Contains(fields[0], "="):
			// VAR=value assignment consumed by env.
			fields = fields[1:]
		default:
			return fields
		}
	}
	return fields
}

// readShebang returns the contents of the first line after "#!", following
// symlinks. It reports false when the file has no shebang.
func readShebang(path string) (string, bool) {
	f, err := os.Open(path)
	if err != nil {
		return "", false
	}
	defer f.Close()

	buf := make([]byte, 512)
	n, _ := f.Read(buf)
	if n < 2 || buf[0] != '#' || buf[1] != '!' {
		return "", false
	}

	line := string(buf[2:n])
	if i := strings.IndexAny(line, "\r\n"); i >= 0 {
		line = line[:i]
	}
	return line, true
}
