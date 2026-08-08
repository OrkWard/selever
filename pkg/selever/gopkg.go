package selever

import (
	"context"
	"fmt"
	"os"

	"github.com/orkward/selever/pkg/install"
	"github.com/orkward/selever/pkg/shell"

	"github.com/spf13/cobra"
)

var gopkgGoVersion string

var gopkgCmd = &cobra.Command{
	Use:   "gopkg <package@version> --go=<version>",
	Short: "Install an exact Go package",
	Long: `Build and install an exact Go package using the specified Go release
and print shell environment code.

A leading "v" on the module version is optional.`,
	Args: cobra.ExactArgs(1),
	Run:  runGopkg,
}

func init() {
	gopkgCmd.Flags().StringVar(&gopkgGoVersion, "go", "", "Go version (required)")
	gopkgCmd.MarkFlagRequired("go")
}

func runGopkg(cmd *cobra.Command, args []string) {
	spec := args[0]
	pkg, version := parsePackageSpec(spec)
	if pkg == "" || version == "" {
		fmt.Fprintf(os.Stderr, "selever gopkg: invalid package spec %q (expected package@version)\n", spec)
		os.Exit(1)
	}

	ctx := context.Background()
	_, binDir, err := install.InstallGopkg(ctx, pkg, version, gopkgGoVersion)
	if err != nil {
		fmt.Fprintf(os.Stderr, "selever gopkg: %v\n", err)
		os.Exit(1)
	}

	fmt.Print(shell.FormatPATH(resolvedShell(), binDir))
}
