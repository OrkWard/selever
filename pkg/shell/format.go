package shell

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FormatPATH returns the shell code that prepends dir to PATH.
// When shell is Auto it acts as Sh.
func FormatPATH(shell Name, dir string) string {
	if c := Canonical(string(shell)); c != "" {
		shell = c
	}
	switch shell {
	case Fish:
		return fmt.Sprintf("set -gx PATH %q $PATH;\n", dir)
	case Pwsh:
		return fmt.Sprintf("$env:PATH = %q + %q + $env:PATH\n", dir, string(pathSep(shell)))
	case Nu:
		return formatNuMulti([]string{dir})
	default:
		return fmt.Sprintf("export PATH=%q:$PATH\n", dir)
	}
}

// FormatMultiPATH writes one environment update that prepends each of dirs
// (in order) to PATH.
func FormatMultiPATH(shell Name, dirs []string) string {
	if c := Canonical(string(shell)); c != "" {
		shell = c
	}
	switch shell {
	case Fish:
		return formatFishMulti(dirs)
	case Pwsh:
		return formatPwshMulti(dirs)
	case Nu:
		return formatNuMulti(dirs)
	default:
		return formatPosixMulti(dirs)
	}
}

func pathSep(shell Name) byte {
	if shell == Pwsh {
		return ';'
	}
	return ':'
}

// --- POSIX (sh, bash, zsh) ---

func formatPosixMulti(dirs []string) string {
	quoted := make([]string, len(dirs))
	for i, d := range dirs {
		quoted[i] = fmt.Sprintf("%q", d)
	}
	return fmt.Sprintf("export PATH=%s:$PATH\n", strings.Join(quoted, ":"))
}

// --- Fish ---

func formatFishMulti(dirs []string) string {
	quoted := make([]string, len(dirs))
	for i, d := range dirs {
		quoted[i] = fmt.Sprintf("%q", d)
	}
	return fmt.Sprintf("set -gx PATH %s $PATH;\n", strings.Join(quoted, " "))
}

// --- PowerShell ---

func formatPwshMulti(dirs []string) string {
	var b strings.Builder
	b.WriteString("$env:PATH = ")
	for i, d := range dirs {
		if i > 0 {
			b.WriteString(" + \";\" + ")
		}
		fmt.Fprintf(&b, "%q", d)
	}
	b.WriteString(" + \";\" + $env:PATH\n")
	return b.String()
}

// --- Nushell ---

// formatNuMulti returns a JSON object holding the complete new PATH as a list:
// dirs followed by the inherited PATH entries. Apply it with:
//
//	selever node 22.14.0 | from json | load-env
//
// Nushell matches PATH case-insensitively on Windows, so this also updates Path.
func formatNuMulti(dirs []string) string {
	path := append([]string{}, dirs...)
	path = append(path, filepath.SplitList(os.Getenv("PATH"))...)
	out, _ := json.Marshal(map[string][]string{"PATH": path})
	return string(out) + "\n"
}
