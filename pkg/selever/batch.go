package selever

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/orkward/selever/pkg/install"
	"github.com/orkward/selever/pkg/shell"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var batchCmd = &cobra.Command{
	Use:   "batch",
	Short: "Install multiple selectors from stdin",
	Long: `Read selectors from standard input and emit one combined environment update.

Each non-empty line is a selector with whitespace-separated fields.
Processing stops at the first invalid selector or failed installation.`,
	Args: cobra.NoArgs,
	Run:  runBatch,
}

func runBatch(cmd *cobra.Command, args []string) {
	env := &shell.Env{}

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		e, err := processBatchLine(cmd.Context(), line)
		if err != nil {
			fmt.Fprintf(os.Stderr, "selever batch: %v\n", err)
			os.Exit(1)
		}
		env.Merge(e)
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "selever batch: read stdin: %v\n", err)
		os.Exit(1)
	}

	if !env.Empty() {
		printEnv(env)
	}
}

// processBatchLine parses a batch input line and installs the selection.
func processBatchLine(ctx context.Context, line string) (*shell.Env, error) {
	fields := strings.Fields(line)
	tool := fields[0]
	flags := pflag.NewFlagSet(tool, pflag.ContinueOnError)
	flags.SetOutput(io.Discard)

	switch tool {
	case "node":
		if err := flags.Parse(fields[1:]); err != nil || flags.NArg() != 1 {
			return nil, fmt.Errorf("node selector requires exactly a version")
		}
		return selectNode(ctx, flags.Arg(0))

	case "go":
		if err := flags.Parse(fields[1:]); err != nil || flags.NArg() != 1 {
			return nil, fmt.Errorf("go selector requires exactly a version")
		}
		return selectGo(ctx, flags.Arg(0))

	case "npm":
		node := flags.String("node", "", "")
		if err := flags.Parse(fields[1:]); err != nil || flags.NArg() != 1 || *node == "" {
			return nil, fmt.Errorf("npm selector requires --node=<version> and <package@version>")
		}
		return selectNpm(ctx, flags.Arg(0), *node)

	case "gopkg":
		goVersion := flags.String("go", "", "")
		if err := flags.Parse(fields[1:]); err != nil || flags.NArg() != 1 || *goVersion == "" {
			return nil, fmt.Errorf("gopkg selector requires --go=<version> and <package@version>")
		}
		return selectGopkg(ctx, flags.Arg(0), *goVersion)

	case "msvc":
		host := flags.String("host", "", "")
		target := flags.String("target", "", "")
		if err := flags.Parse(fields[1:]); err != nil || flags.NArg() != 1 {
			return nil, fmt.Errorf("msvc selector requires a version and optional --host=<arch> --target=<arch>")
		}
		return install.InstallMSVC(ctx, flags.Arg(0), *host, *target)

	case "winsdk":
		host := flags.String("host", "", "")
		target := flags.String("target", "", "")
		if err := flags.Parse(fields[1:]); err != nil || flags.NArg() != 1 {
			return nil, fmt.Errorf("winsdk selector requires a build and optional --host=<arch> --target=<arch>")
		}
		return install.InstallWinSDK(ctx, flags.Arg(0), *host, *target)

	default:
		return nil, fmt.Errorf("unknown tool %q", tool)
	}
}
