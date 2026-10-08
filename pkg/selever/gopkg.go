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
	Run: func(cmd *cobra.Command, args []string) {
		env, err := selectGopkg(cmd.Context(), args[0], gopkgGoVersion)
		if err != nil {
			fmt.Fprintf(os.Stderr, "selever gopkg: %v\n", err)
			os.Exit(1)
		}
		printEnv(env)
	},
}

func init() {
	gopkgCmd.Flags().StringVar(&gopkgGoVersion, "go", "", "Go version (required)")
	gopkgCmd.MarkFlagRequired("go")
}

func selectGopkg(ctx context.Context, spec, goVersion string) (*shell.Env, error) {
	pkg, version := parsePackageSpec(spec)
	if pkg == "" || version == "" {
		return nil, fmt.Errorf("invalid package spec %q (expected package@version)", spec)
	}
	_, binDir, err := install.InstallGopkg(ctx, pkg, version, goVersion)
	if err != nil {
		return nil, err
	}
	return shell.PathEnv(binDir), nil
}
