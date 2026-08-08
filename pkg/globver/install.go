package globver

import (
	"context"
	"fmt"
	"os"

	"git.lan/selever/pkg/install"

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
		binDirs, err := installSelection(ctx, j.selector)
		if err != nil {
			fmt.Fprintf(os.Stderr, "globver install: %v\n", err)
			os.Exit(1)
		}

		for _, exe := range j.exes {
			if err := CreateShim(exe, binDirs); err != nil {
				fmt.Fprintf(os.Stderr, "globver install: %v\n", err)
				os.Exit(1)
			}
		}
	}
}

// installSelection installs a recorded selection and returns the bin directories
// to add to PATH.
func installSelection(ctx context.Context, sel []string) ([]string, error) {
	if len(sel) < 2 {
		return nil, fmt.Errorf("invalid selector: %v", sel)
	}

	tool := sel[0]

	switch tool {
	case "node":
		dir, err := install.InstallNode(ctx, sel[1])
		return []string{dir}, err

	case "go":
		dir, err := install.InstallGo(ctx, sel[1])
		return []string{dir}, err

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
		return []string{binDir}, err

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
		return []string{binDir}, err

	default:
		return nil, fmt.Errorf("unknown tool %q", tool)
	}
}
