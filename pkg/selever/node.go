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

var nodeCmd = &cobra.Command{
	Use:   "node <version>",
	Short: "Install an exact Node.js release",
	Long: `Install an exact Node.js release and print shell environment code.

The version must be exact and without a leading "v".`,
	Args: cobra.ExactArgs(1),
	Run:  runNode,
}

func runNode(cmd *cobra.Command, args []string) {
	version := strings.TrimPrefix(args[0], "v")
	ctx := context.Background()

	dir, err := install.InstallNode(ctx, version)
	if err != nil {
		fmt.Fprintf(os.Stderr, "selever node: %v\n", err)
		os.Exit(1)
	}

	fmt.Print(shell.FormatPATH(resolvedShell(), dir))
}
