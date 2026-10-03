package globver

import (
	"os"
	"path/filepath"
	"strings"
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

	script := filepath.Join(binDir, exe)
	args, ok := nodeShebang(script)
	if !ok {
		return spec
	}

	spec.Interp = filepath.Join(nodeBinDir, "node")
	spec.InterpArgs = args
	spec.Script = script
	return spec
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
