package shell

import (
	"encoding/json"
	"fmt"
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
		return nuJSON(dir)
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

// nuJSON returns a JSON object for load-env with only the directory to prepend.
// Nushell users pipe through: … | from json | each {|x| load-env {PATH: $"($x.PATH):($env.PATH)"}}
func nuJSON(dir string) string {
	obj := map[string]string{"PATH": dir}
	out, _ := json.Marshal(obj)
	return string(out) + "\n"
}

func formatNuMulti(dirs []string) string {
	obj := map[string]string{"PATH": strings.Join(dirs, ":")}
	out, _ := json.Marshal(obj)
	return string(out) + "\n"
}
