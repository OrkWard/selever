package fetchver

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

var gopkgCmd = &cobra.Command{
	Use:   "gopkg <package[@version]>",
	Short: "Resolve a Go module version",
	Long: `Resolve a Go package or module by running "go list -m -json".

A package inside a module resolves to the version of its enclosing module.

The optional version is a Go module version query. Omitting it is equivalent
to @latest. A numeric query is passed to Go with a leading "v".`,
	Args: cobra.ExactArgs(1),
	Run:  runFetchGopkg,
}

type goListOutput struct {
	Path    string `json:"Path"`
	Version string `json:"Version"`
}

func runFetchGopkg(cmd *cobra.Command, args []string) {
	spec := args[0]
	pkg, query := parseSpec(spec)

	result, err := resolveGopkg(pkg, query)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fetchver gopkg: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(result)
}

func resolveGopkg(pkg, query string) (string, error) {
	modQuery := query
	if modQuery == "" {
		modQuery = "latest"
	} else if isNumericQuery(modQuery) {
		// Add leading "v" for numeric queries.
		if !strings.HasPrefix(modQuery, "v") {
			modQuery = "v" + modQuery
		}
	}

	goBin, err := exec.LookPath("go")
	if err != nil {
		return "", fmt.Errorf("go not found in PATH")
	}

	// "go list -m" needs a module path, but the argument may be a package
	// inside a module (github.com/go-delve/delve/cmd/dlv lives in
	// github.com/go-delve/delve). Go has no query form for package paths, so
	// try the path itself and then each shorter prefix until one is a module.
	// Report the failure of the requested path, not of some short prefix.
	var firstErr error
	for modPath := pkg; modPath != ""; modPath = parentPath(modPath) {
		out, err := exec.Command(goBin, "list", "-m", "-json", modPath+"@"+modQuery).Output()
		if err != nil {
			if firstErr == nil {
				firstErr = goListError(err)
			}
			continue
		}

		var result goListOutput
		if err := json.Unmarshal(out, &result); err != nil {
			return "", fmt.Errorf("parse go list output: %w", err)
		}

		// Report the requested package path with the resolved module version.
		return pkg + "@" + result.Version, nil
	}

	return "", fmt.Errorf("resolve %s@%s: %w", pkg, modQuery, firstErr)
}

// parentPath drops the last element of a slash separated path, returning ""
// once nothing is left. "gioui.org/cmd/gogio" → "gioui.org/cmd" → "gioui.org".
func parentPath(p string) string {
	i := strings.LastIndex(p, "/")
	if i < 0 {
		return ""
	}
	return p[:i]
}

// goListError keeps the stderr of a failed "go list" in the error message.
func goListError(err error) error {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && len(exitErr.Stderr) > 0 {
		return fmt.Errorf("go list: %s", strings.TrimSpace(string(exitErr.Stderr)))
	}
	return fmt.Errorf("go list: %w", err)
}

// isNumericQuery returns true if the query looks like a numeric version prefix.
func isNumericQuery(s string) bool {
	if s == "" || s == "latest" || s == "upgrade" || s == "patch" || s == "master" || s == "main" {
		return false
	}
	// A numeric prefix starts with a digit.
	return len(s) > 0 && s[0] >= '0' && s[0] <= '9'
}
