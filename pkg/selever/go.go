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

var goCmd = &cobra.Command{
	Use:   "go <version>",
	Short: "Install an exact Go release",
	Long: `Install an exact Go release and print shell environment code.

The version must be exact and without a leading "go".`,
	Args: cobra.ExactArgs(1),
	Run:  runGo,
}

func runGo(cmd *cobra.Command, args []string) {
	version := strings.TrimPrefix(args[0], "go")
	ctx := context.Background()

	dir, err := install.InstallGo(ctx, version)
	if err != nil {
		fmt.Fprintf(os.Stderr, "selever go: %v\n", err)
		os.Exit(1)
	}

	fmt.Print(shell.FormatPATH(resolvedShell(), dir))
}
