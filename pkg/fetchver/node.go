package fetchver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

var nodeCmd = &cobra.Command{
	Use:   "node <version>",
	Short: "Resolve a Node.js version query",
	Long: `Resolve a Node.js release version.

<version> may be "latest", a 1/2/3-part numeric prefix, or an LTS codename.
A leading "v" is accepted. LTS codenames are matched without regard to case.`,
	Args: cobra.ExactArgs(1),
	Run:  runFetchNode,
}

// nodeRelease represents a release from nodejs.org/dist/index.json.
type nodeRelease struct {
	Version string `json:"version"` // "v22.14.0"
	LTS     any    `json:"lts"`     // string codename, false, or true
}

func runFetchNode(cmd *cobra.Command, args []string) {
	query := strings.TrimPrefix(args[0], "v")

	result, err := resolveNode(query)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fetchver node: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(result)
}

func resolveNode(query string) (string, error) {
	releases, err := fetchNodeIndex()
	if err != nil {
		return "", err
	}

	query = strings.ToLower(query)

	// "latest" → highest stable version.
	if query == "latest" {
		for _, r := range releases {
			if r.LTS != nil && r.LTS != false {
				return strings.TrimPrefix(r.Version, "v"), nil
			}
		}
		// Fallback: first stable.
		return strings.TrimPrefix(releases[0].Version, "v"), nil
	}

	// Try LTS codename match.
	for _, r := range releases {
		ltsName, ok := r.LTS.(string)
		if ok && strings.EqualFold(ltsName, query) {
			return strings.TrimPrefix(r.Version, "v"), nil
		}
	}

	// Try numeric prefix match.
	query = "v" + query
	for _, r := range releases {
		if strings.HasPrefix(r.Version, query) {
			return strings.TrimPrefix(r.Version, "v"), nil
		}
	}

	return "", fmt.Errorf("no matching Node.js release for %q", query)
}

// fetchNodeIndex fetches and parses the Node.js release index.
func fetchNodeIndex() ([]nodeRelease, error) {
	resp, err := http.Get("https://nodejs.org/dist/index.json")
	if err != nil {
		return nil, fmt.Errorf("fetch node index: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("node index returned %s", resp.Status)
	}

	var releases []nodeRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("parse node index: %w", err)
	}

	// Sort descending by version (already sorted, but defensive).
	sort.Slice(releases, func(i, j int) bool {
		return versionOrder(releases[i].Version) > versionOrder(releases[j].Version)
	})

	return releases, nil
}

// versionOrder converts "v22.14.0" into a sortable string (zero-padded parts).
func versionOrder(v string) string {
	parts := strings.Split(strings.TrimPrefix(v, "v"), ".")
	for len(parts) < 4 {
		parts = append(parts, "0")
	}
	return fmt.Sprintf("%06s.%06s.%06s.%06s", parts[0], parts[1], parts[2], parts[3])
}
