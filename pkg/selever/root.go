package selever

import (
	"fmt"
	"os"

	"git.lan/selever/pkg/shell"

	"github.com/spf13/cobra"
)

var shellName string

// rootCmd is the top-level selever command.
var rootCmd = &cobra.Command{
	Use:   "selever",
	Short: "Install exact toolchains and packages",
	Long: `selever installs exact Node.js, Go, npm package, and Go package versions
in immutable directories and prints shell environment updates.`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVar(&shellName, "shell", "auto",
		"shell format: auto, sh, bash, zsh, fish, pwsh, powershell, nu, nushell")
	rootCmd.AddCommand(nodeCmd)
	rootCmd.AddCommand(npmCmd)
	rootCmd.AddCommand(goCmd)
	rootCmd.AddCommand(gopkgCmd)
	rootCmd.AddCommand(batchCmd)
}

// resolvedShell returns the canonical shell name or exits on invalid input.
func resolvedShell() shell.Name {
	s := shell.Canonical(shellName)
	if s == "" {
		fmt.Fprintf(os.Stderr, "selever: unsupported shell %q\n", shellName)
		os.Exit(1)
	}
	if s == shell.Auto {
		s = shell.DetectShell()
	}
	return s
}
