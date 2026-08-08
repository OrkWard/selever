package shell

import (
	"os"
	"path/filepath"
	"strings"
)

// Name identifies a shell dialect.
type Name string

const (
	Sh        Name = "sh"
	Bash      Name = "bash"
	Zsh       Name = "zsh"
	Fish      Name = "fish"
	Pwsh      Name = "pwsh"
	Powershell Name = "powershell"
	Nu        Name = "nu"
	Nushell   Name = "nushell"
	Auto      Name = "auto"
)

// valid maps canonical shell names to themselves and aliases to their canonical form.
var valid = map[Name]Name{
	Sh:        Sh,
	Bash:      Bash,
	Zsh:       Zsh,
	Fish:      Fish,
	Pwsh:      Pwsh,
	Powershell: Pwsh,
	Nu:        Nu,
	Nushell:   Nu,
	Auto:      Auto,
}

// Canonical returns the canonical shell name for s, or an empty string when
// s is not a recognised shell.
func Canonical(s string) Name {
	c, _ := valid[Name(s)]
	return c
}

// DetectShell returns the canonical name of the invoking shell.
//
// On Linux it reads /proc/<ppid>/comm. On macOS and other Unix systems it
// falls back to the SHELL environment variable, taking the basename.
func DetectShell() Name {
	name := detect()
	if c := Canonical(name); c != "" {
		return c
	}
	return Sh
}

// detect reads the parent process comm or falls back to $SHELL.
func detect() string {
	if n := detectProc(); n != "" {
		return n
	}
	return shellFromEnv()
}

// shellFromEnv extracts the basename of $SHELL.
func shellFromEnv() string {
	s := os.Getenv("SHELL")
	if s == "" {
		return ""
	}
	return strings.ToLower(filepath.Base(s))
}
