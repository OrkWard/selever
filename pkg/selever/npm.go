package selever

import (
	"context"
	"fmt"
	"os"
	"strings"

	"git.lan/selever/pkg/install"
	"git.lan/selever/pkg/shell"

	"github.com/spf13/cobra"
)

var npmNodeVersion string

var npmCmd = &cobra.Command{
	Use:   "npm <package@version> --node=<version>",
	Short: "Install an exact npm package",
	Long: `Install an exact npm package using the specified Node.js release
and print shell environment code.`,
	Args: cobra.ExactArgs(1),
	Run:  runNpm,
}

func init() {
	npmCmd.Flags().StringVar(&npmNodeVersion, "node", "", "Node.js version (required)")
	npmCmd.MarkFlagRequired("node")
}

func runNpm(cmd *cobra.Command, args []string) {
	spec := args[0]
	pkg, version := parsePackageSpec(spec)
	if pkg == "" || version == "" {
		fmt.Fprintf(os.Stderr, "selever npm: invalid package spec %q (expected package@version)\n", spec)
		os.Exit(1)
	}

	ctx := context.Background()
	_, binDir, err := install.InstallNpm(ctx, pkg, version, npmNodeVersion)
	if err != nil {
		fmt.Fprintf(os.Stderr, "selever npm: %v\n", err)
		os.Exit(1)
	}

	fmt.Print(shell.FormatPATH(resolvedShell(), binDir))
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
