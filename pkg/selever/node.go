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

var nodeCmd = &cobra.Command{
	Use:   "node <version>",
	Short: "Install an exact Node.js release",
	Long: `Install an exact Node.js release and print shell environment code.

The version must be exact and without a leading "v".`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		env, err := selectNode(cmd.Context(), args[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "selever node: %v\n", err)
			os.Exit(1)
		}
		printEnv(env)
	},
}

func selectNode(ctx context.Context, version string) (*shell.Env, error) {
	dir, err := install.InstallNode(ctx, strings.TrimPrefix(version, "v"))
	if err != nil {
		return nil, err
	}
	return shell.PathEnv(install.ToolBinDir("node", dir)), nil
}
