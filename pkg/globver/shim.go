package globver

import (
	"os"
	"path/filepath"
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
//
// On Windows the launcher runs the target by absolute path and PathDirs only
// locate it; PATH is left unchanged.
type ShimSpec struct {
	PathDirs   []string // dirs prepended to PATH (inherited by children)
	Interp     string   // absolute interpreter path; empty means PATH lookup
	InterpArgs []string // interpreter flags taken from the script's shebang
	Script     string   // absolute script path, passed to the interpreter
}

