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
	Short: "Resolve a Go version query",
	Long: `Resolve a Go release version.

<version> may be "latest", a 1/2/3-part numeric prefix, or an exact version.
A leading "go" is accepted.`,
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
	fullQuery := "go" + query

	if query == "latest" {
		for _, r := range releases {
			if r.Stable {
				return strings.TrimPrefix(r.Version, "go"), nil
			}
		}
		return "", fmt.Errorf("no stable Go release found")
	}

	// Prefix match: prefer the latest stable release.
	var best string
	for _, r := range releases {
		if !strings.HasPrefix(r.Version, fullQuery) || !r.Stable {
			continue
		}
		// Rest must be empty (exact match) or start with '.' (segment boundary).
		rest := strings.TrimPrefix(r.Version, fullQuery)
		if rest != "" && rest[0] != '.' {
			continue
		}
		v := strings.TrimPrefix(r.Version, "go")
		if best == "" || versionCmp(v, best) > 0 {
			best = v
		}
	}

	if best != "" {
		return best, nil
	}

	return "", fmt.Errorf("no matching Go release for %q", query)
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
