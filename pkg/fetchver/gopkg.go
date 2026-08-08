package fetchver

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

var gopkgCmd = &cobra.Command{
	Use:   "gopkg <package[@version]>",
	Short: "Resolve a Go module version",
	Long: `Resolve a Go module by running "go list -m -json".

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

	queryStr := pkg + "@" + modQuery

	goBin, err := exec.LookPath("go")
	if err != nil {
		return "", fmt.Errorf("go not found in PATH")
	}

	out, err := exec.Command(goBin, "list", "-m", "-json", queryStr).Output()
	if err != nil {
		return "", fmt.Errorf("go list: %w", err)
	}

	var result goListOutput
	if err := json.Unmarshal(out, &result); err != nil {
		return "", fmt.Errorf("parse go list output: %w", err)
	}

	return result.Path + "@" + result.Version, nil
}

// isNumericQuery returns true if the query looks like a numeric version prefix.
func isNumericQuery(s string) bool {
	if s == "" || s == "latest" || s == "upgrade" || s == "patch" || s == "master" || s == "main" {
		return false
	}
	// A numeric prefix starts with a digit.
	return len(s) > 0 && s[0] >= '0' && s[0] <= '9'
}
