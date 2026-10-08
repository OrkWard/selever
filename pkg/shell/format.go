package shell

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Env is an environment update. List variables (PATH, INCLUDE, ...) get
// entries prepended to their current value; scalar variables are replaced.
type Env struct {
	lists []listVar
	vars  []scalarVar
}

type listVar struct {
	name    string
	entries []string
}

type scalarVar struct {
	name, value string
}

// Prepend adds entries in front of the current value of the list variable
// name. Entries from later calls follow those from earlier calls.
func (e *Env) Prepend(name string, entries ...string) {
	for i := range e.lists {
		if strings.EqualFold(e.lists[i].name, name) {
			e.lists[i].entries = append(e.lists[i].entries, entries...)
			return
		}
	}
	e.lists = append(e.lists, listVar{name, append([]string{}, entries...)})
}

// Set replaces the value of the variable name. A later Set wins.
func (e *Env) Set(name, value string) {
	for i := range e.vars {
		if strings.EqualFold(e.vars[i].name, name) {
			e.vars[i].value = value
			return
		}
	}
	e.vars = append(e.vars, scalarVar{name, value})
}

// Merge applies o after e.
func (e *Env) Merge(o *Env) {
	if o == nil {
		return
	}
	for _, l := range o.lists {
		e.Prepend(l.name, l.entries...)
	}
	for _, v := range o.vars {
		e.Set(v.name, v.value)
	}
}

// Empty reports whether e changes nothing.
func (e *Env) Empty() bool {
	for _, l := range e.lists {
		if len(l.entries) > 0 {
			return false
		}
	}
	return len(e.vars) == 0
}

// PathEnv returns an update that prepends dirs to PATH.
func PathEnv(dirs ...string) *Env {
	e := &Env{}
	e.Prepend("PATH", dirs...)
	return e
}

// FormatEnv returns the shell code that applies e.
// When shell is Auto it acts as Sh.
func FormatEnv(shell Name, e *Env) string {
	if c := Canonical(string(shell)); c != "" {
		shell = c
	}
	if shell == Nu {
		return formatNu(e)
	}

	var b strings.Builder
	for _, l := range e.lists {
		if len(l.entries) == 0 {
			continue
		}
		switch shell {
		case Fish:
			b.WriteString(formatFishList(l))
		case Pwsh:
			b.WriteString(formatPwshList(l))
		default:
			b.WriteString(formatPosixList(l))
		}
	}
	for _, v := range e.vars {
		switch shell {
		case Fish:
			fmt.Fprintf(&b, "set -gx %s %q;\n", v.name, v.value)
		case Pwsh:
			fmt.Fprintf(&b, "$env:%s = %s\n", v.name, pwshQuote(v.value))
		default:
			fmt.Fprintf(&b, "export %s=%q\n", v.name, v.value)
		}
	}
	return b.String()
}

func isPath(name string) bool {
	return strings.EqualFold(name, "PATH")
}

// listSep separates entries of list variables other than PATH, such as the
// MSVC INCLUDE and LIB variables.
var listSep = string(os.PathListSeparator)

func quoteAll(entries []string) []string {
	quoted := make([]string, len(entries))
	for i, d := range entries {
		quoted[i] = fmt.Sprintf("%q", d)
	}
	return quoted
}

// --- POSIX (sh, bash, zsh) ---

func formatPosixList(l listVar) string {
	if isPath(l.name) {
		return fmt.Sprintf("export PATH=%s:$PATH\n", strings.Join(quoteAll(l.entries), ":"))
	}
	return fmt.Sprintf("export %s=%q${%s:+%q$%s}\n",
		l.name, strings.Join(l.entries, listSep), l.name, listSep, l.name)
}

// --- Fish ---

func formatFishList(l listVar) string {
	if isPath(l.name) {
		return fmt.Sprintf("set -gx PATH %s $PATH;\n", strings.Join(quoteAll(l.entries), " "))
	}
	// Fish joins exported lists with spaces, so build one string.
	return fmt.Sprintf("set -gx %s %q\"$%s\";\n",
		l.name, strings.Join(l.entries, listSep)+listSep, l.name)
}

// --- PowerShell ---

func formatPwshList(l listVar) string {
	name := l.name
	if isPath(name) {
		name = "PATH"
	}
	quoted := make([]string, len(l.entries))
	for i, d := range l.entries {
		quoted[i] = pwshQuote(d)
	}
	return fmt.Sprintf("$env:%s = %s + ';' + $env:%s\n",
		name, strings.Join(quoted, " + ';' + "), name)
}

// pwshQuote returns s as a PowerShell verbatim string, in which backslashes
// and $ are literal.
func pwshQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// --- Nushell ---

// envName returns name as spelled in the current environment. Windows names
// are case-insensitive, and nushell's load-env given another spelling (PATH
// for Path) can hand externals either spelling.
func envName(name string) string {
	if runtime.GOOS != "windows" {
		return name
	}
	for _, kv := range os.Environ() {
		if k, _, ok := strings.Cut(kv, "="); ok && strings.EqualFold(k, name) {
			return k
		}
	}
	return name
}

// formatNu returns a JSON object to apply with load-env:
//
//	selever node 22.14.0 | from json | load-env
//
// PATH is the complete new search path as a list: the new entries followed by
// the inherited ones. Other list variables are strings that hold the new
// entries followed by the inherited value. Keys use the spelling of the
// inherited variable (Path on Windows).
func formatNu(e *Env) string {
	out := map[string]any{}
	for _, l := range e.lists {
		if len(l.entries) == 0 {
			continue
		}
		if isPath(l.name) {
			path := append([]string{}, l.entries...)
			out[envName("PATH")] = append(path, filepath.SplitList(os.Getenv("PATH"))...)
			continue
		}
		v := strings.Join(l.entries, listSep)
		if cur := os.Getenv(l.name); cur != "" {
			v += listSep + cur
		}
		out[envName(l.name)] = v
	}
	for _, v := range e.vars {
		out[envName(v.name)] = v.value
	}
	data, _ := json.Marshal(out)
	return string(data) + "\n"
}
