package fetchver

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(goCmd)
}

var goCmd = &cobra.Command{
	Use:   "go <version>",
	Short: "Resolve the latest Go release",
	Long: `Resolve a Go release version.

<version> must be "latest" to resolve the latest stable Go release.
Go versions are irregular (1-, 2-, or 3-part); use exact versions directly.`,
	Args: cobra.ExactArgs(1),
	Run:  runFetchGo,
}

func runFetchGo(cmd *cobra.Command, args []string) {
	query := strings.TrimPrefix(args[0], "go")

	result, err := resolveGo(query)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fetchver go: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(result)
}

type goRelease struct {
	Version string `json:"version"` // "go1.26.5"
	Stable  bool   `json:"stable"`
}

func resolveGo(query string) (string, error) {
	releases, err := fetchGoReleases()
	if err != nil {
		return "", err
	}

	query = strings.ToLower(query)

	if query == "latest" {
		for _, r := range releases {
			if r.Stable {
				return strings.TrimPrefix(r.Version, "go"), nil
			}
		}
		return "", fmt.Errorf("no stable Go release found")
	}

	return "", fmt.Errorf("Go releases require an exact version; use %q to resolve the latest stable release", "latest")
}

func fetchGoReleases() ([]goRelease, error) {
	resp, err := http.Get("https://go.dev/dl/?mode=json&include=all")
	if err != nil {
		return nil, fmt.Errorf("fetch go index: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("go index returned %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var releases []goRelease
	if err := json.Unmarshal(body, &releases); err != nil {
		return nil, fmt.Errorf("parse go index: %w", err)
	}

	// Sort descending by version.
	sort.Slice(releases, func(i, j int) bool {
		return versionCmp(releases[i].Version, releases[j].Version) > 0
	})

	return releases, nil
}
