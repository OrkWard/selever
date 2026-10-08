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

var goCmd = &cobra.Command{
	Use:   "go <version>",
	Short: "Install an exact Go release",
	Long: `Install an exact Go release and print shell environment code.

The version must be exact and without a leading "go".`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		env, err := selectGo(cmd.Context(), args[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "selever go: %v\n", err)
			os.Exit(1)
		}
		printEnv(env)
	},
}

func selectGo(ctx context.Context, version string) (*shell.Env, error) {
	dir, err := install.InstallGo(ctx, strings.TrimPrefix(version, "go"))
	if err != nil {
		return nil, err
	}
	return shell.PathEnv(install.ToolBinDir("go", dir)), nil
}
