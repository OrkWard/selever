package selever

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/orkward/selever/pkg/install"
	"github.com/orkward/selever/pkg/shell"

	"github.com/spf13/cobra"
)

var npmNodeVersion string

var npmCmd = &cobra.Command{
	Use:   "npm <package@version> --node=<version>",
	Short: "Install an exact npm package",
	Long: `Install an exact npm package using the specified Node.js release
and print shell environment code.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		env, err := selectNpm(cmd.Context(), args[0], npmNodeVersion)
		if err != nil {
			fmt.Fprintf(os.Stderr, "selever npm: %v\n", err)
			os.Exit(1)
		}
		printEnv(env)
	},
}

func init() {
	npmCmd.Flags().StringVar(&npmNodeVersion, "node", "", "Node.js version (required)")
	npmCmd.MarkFlagRequired("node")
}

func selectNpm(ctx context.Context, spec, nodeVersion string) (*shell.Env, error) {
	pkg, version := parsePackageSpec(spec)
	if pkg == "" || version == "" {
		return nil, fmt.Errorf("invalid package spec %q (expected package@version)", spec)
	}
	_, binDir, err := install.InstallNpm(ctx, pkg, version, nodeVersion)
	if err != nil {
		return nil, err
	}
	return shell.PathEnv(binDir), nil
}

// parsePackageSpec splits "package@version" into (pkg, version).
func parsePackageSpec(spec string) (string, string) {
	// The last @ separates package from version.
	i := strings.LastIndex(spec, "@")
	if i < 0 {
		return "", ""
	}
	return spec[:i], spec[i+1:]
}
