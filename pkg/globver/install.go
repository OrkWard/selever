package globver

import (
	"context"
	"fmt"
	"os"

	"github.com/orkward/selever/pkg/install"

	"github.com/spf13/cobra"
)

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install all recorded selections and recreate launchers",
	Long: `Install every recorded selection and recreate its managed launchers.

Suitable for restoring the configured commands on another machine after
copying global.json.`,
	Args: cobra.NoArgs,
	Run:  runInstall,
}

func runInstall(cmd *cobra.Command, args []string) {
	cfg, err := LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "globver install: %v\n", err)
		os.Exit(1)
	}

	if len(cfg) == 0 {
		fmt.Fprintln(os.Stderr, "No selections recorded.")
		return
	}

	// Group by selector to avoid duplicate installs.
	type job struct {
		selector []string
		exes     []string
	}

	seen := make(map[string]*job)
	for exe, sel := range cfg {
		key := fmt.Sprintf("%v", sel)
		if j, ok := seen[key]; ok {
			j.exes = append(j.exes, exe)
		} else {
			seen[key] = &job{selector: sel, exes: []string{exe}}
		}
	}

	ctx := context.Background()
	for _, j := range seen {
		spec, err := installSelection(ctx, j.selector)
		if err != nil {
			fmt.Fprintf(os.Stderr, "globver install: %v\n", err)
			os.Exit(1)
		}

		for _, exe := range j.exes {
			if err := CreateShim(exe, spec(exe)); err != nil {
				fmt.Fprintf(os.Stderr, "globver install: %v\n", err)
				os.Exit(1)
			}
		}
	}
}

// installSelection installs a recorded selection and returns a builder for
// its launcher specs.
func installSelection(ctx context.Context, sel []string) (specFunc, error) {
	if len(sel) < 2 {
		return nil, fmt.Errorf("invalid selector: %v", sel)
	}

	tool := sel[0]

	switch tool {
	case "node":
		dir, err := install.InstallNode(ctx, sel[1])
		return toolchainSpec(install.ToolBinDir("node", dir)), err

	case "go":
		dir, err := install.InstallGo(ctx, sel[1])
		return toolchainSpec(install.ToolBinDir("go", dir)), err

	case "npm":
		// sel = ["npm", "pkg@ver", "--node", "ver"]
		spec := sel[1]
		var nodeVersion string
		for i, f := range sel {
			if f == "--node" && i+1 < len(sel) {
				nodeVersion = sel[i+1]
			}
		}
		pkg, ver := splitSpec(spec)
		_, binDir, err := install.InstallNpm(ctx, pkg, ver, nodeVersion)
		if err != nil {
			return nil, err
		}
		return npmSpec(binDir, nodeVersion), nil

	case "gopkg":
		spec := sel[1]
		var goVersion string
		for i, f := range sel {
			if f == "--go" && i+1 < len(sel) {
				goVersion = sel[i+1]
			}
		}
		pkg, ver := splitSpec(spec)
		_, binDir, err := install.InstallGopkg(ctx, pkg, ver, goVersion)
		return toolchainSpec(binDir), err

	default:
		return nil, fmt.Errorf("unknown tool %q", tool)
	}
}
