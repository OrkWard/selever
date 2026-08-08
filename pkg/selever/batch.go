package selever

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"git.lan/selever/pkg/install"
	"git.lan/selever/pkg/shell"

	"github.com/spf13/cobra"
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
	ctx := context.Background()
	var dirs []string

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		d, err := processBatchLine(ctx, line)
		if err != nil {
			fmt.Fprintf(os.Stderr, "selever batch: %v\n", err)
			os.Exit(1)
		}
		dirs = append(dirs, d)
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "selever batch: read stdin: %v\n", err)
		os.Exit(1)
	}

	if len(dirs) > 0 {
		fmt.Print(shell.FormatMultiPATH(resolvedShell(), dirs))
	}
}

// processBatchLine parses a batch input line and installs the selection.
// Returns the bin directory to add to PATH.
func processBatchLine(ctx context.Context, line string) (string, error) {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return "", fmt.Errorf("invalid batch line: %q", line)
	}

	tool := fields[0]

	switch tool {
	case "node":
		if len(fields) != 2 {
			return "", fmt.Errorf("node selector requires exactly a version")
		}
		version := strings.TrimPrefix(fields[1], "v")
		dir, err := install.InstallNode(ctx, version)
		if err != nil {
			return "", err
		}
		return dir, nil

	case "go":
		if len(fields) != 2 {
			return "", fmt.Errorf("go selector requires exactly a version")
		}
		version := strings.TrimPrefix(fields[1], "go")
		dir, err := install.InstallGo(ctx, version)
		if err != nil {
			return "", err
		}
		return dir, nil

	case "npm":
		return processBatchNpm(ctx, fields)

	case "gopkg":
		return processBatchGopkg(ctx, fields)

	default:
		return "", fmt.Errorf("unknown tool %q", tool)
	}
}

func processBatchNpm(ctx context.Context, fields []string) (string, error) {
	// Format: npm [--node=<version>] <package@version>
	var nodeVersion string
	var spec string

	for _, f := range fields[1:] {
		if strings.HasPrefix(f, "--node=") {
			nodeVersion = strings.TrimPrefix(f, "--node=")
		} else {
			spec = f
		}
	}

	if spec == "" || nodeVersion == "" {
		return "", fmt.Errorf("npm batch selector requires --node=<version> and <package@version>")
	}

	pkg, version := parsePackageSpec(spec)
	if pkg == "" || version == "" {
		return "", fmt.Errorf("invalid npm package spec %q", spec)
	}

	_, binDir, err := install.InstallNpm(ctx, pkg, version, nodeVersion)
	return binDir, err
}

func processBatchGopkg(ctx context.Context, fields []string) (string, error) {
	// Format: gopkg [--go=<version>] <package@version>
	var goVersion string
	var spec string

	for _, f := range fields[1:] {
		if strings.HasPrefix(f, "--go=") {
			goVersion = strings.TrimPrefix(f, "--go=")
		} else {
			spec = f
		}
	}

	if spec == "" || goVersion == "" {
		return "", fmt.Errorf("gopkg batch selector requires --go=<version> and <package@version>")
	}

	pkg, version := parsePackageSpec(spec)
	if pkg == "" || version == "" {
		return "", fmt.Errorf("invalid gopkg spec %q", spec)
	}

	_, binDir, err := install.InstallGopkg(ctx, pkg, version, goVersion)
	return binDir, err
}
