package selever

import (
	"context"
	"fmt"
	"os"

	"github.com/orkward/selever/pkg/install"
	"github.com/orkward/selever/pkg/shell"

	"github.com/spf13/cobra"
)

var zigCmd = &cobra.Command{
	Use:   "zig <version>",
	Short: "Install an exact Zig release",
	Long: `Install an exact Zig release and print shell environment code.

The version must be a release in https://ziglang.org/download/index.json
or the current master build version.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		env, err := selectZig(cmd.Context(), args[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "selever zig: %v\n", err)
			os.Exit(1)
		}
		printEnv(env)
	},
}

var zlsCmd = &cobra.Command{
	Use:   "zls <version>",
	Short: "Install an exact ZLS release",
	Long: `Install an exact tagged ZLS release and print shell environment code.

A ZLS release supports the Zig releases with the same major and minor
version.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		env, err := selectZls(cmd.Context(), args[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "selever zls: %v\n", err)
			os.Exit(1)
		}
		printEnv(env)
	},
}

func selectZig(ctx context.Context, version string) (*shell.Env, error) {
	dir, err := install.InstallZig(ctx, version)
	if err != nil {
		return nil, err
	}
	return shell.PathEnv(install.ToolBinDir("zig", dir)), nil
}

func selectZls(ctx context.Context, version string) (*shell.Env, error) {
	dir, err := install.InstallZls(ctx, version)
	if err != nil {
		return nil, err
	}
	return shell.PathEnv(install.ToolBinDir("zls", dir)), nil
}
