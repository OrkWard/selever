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

var dotnetCmd = &cobra.Command{
	Use:   "dotnet <package[@version]>",
	Short: "Resolve a dotnet tool version",
	Long: `Resolve a dotnet tool package through NuGet.

The optional version may be "latest" or a 1/2/3-part stable version prefix.
Omitting it is equivalent to @latest. Package names are matched without regard
to case, and the registry's spelling is used in the result.`,
	Args: cobra.ExactArgs(1),
	Run:  runFetchDotnet,
}

const nuGetServiceIndex = "https://api.nuget.org/v3/index.json"

func runFetchDotnet(cmd *cobra.Command, args []string) {
	spec := args[0]
	pkg, query := parseSpec(spec)

	result, err := resolveDotnet(pkg, query)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fetchver dotnet: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(result)
}

func resolveDotnet(pkg, query string) (string, error) {
	if query == "" {
		query = "latest"
	}

	// Get the registration base URL from the service index.
	regBase, err := nuGetRegistrationBase()
	if err != nil {
		return "", err
	}

	// Normalize package name (case-insensitive search).
	versions, canonicalName, err := fetchNuGetVersions(regBase, pkg)
	if err != nil {
		return "", err
	}

	if query == "latest" {
		return canonicalName + "@" + versions[0], nil
	}

	// Prefix match (versions are sorted descending).
	for _, v := range versions {
		if strings.HasPrefix(v, query) {
			return canonicalName + "@" + v, nil
		}
	}

	return "", fmt.Errorf("no matching version for %s@%s", pkg, query)
}

// fetchNuGetVersions fetches all versions for a package from the NuGet registration API.
func fetchNuGetVersions(regBase, pkg string) ([]string, string, error) {
	// NuGet registration URL: <base>/<lowercase-pkg>/index.json
	url := fmt.Sprintf("%s/%s/index.json", strings.TrimRight(regBase, "/"), strings.ToLower(pkg))

	resp, err := http.Get(url)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("GET %s: %s", url, resp.Status)
	}

	// NuGet registration response structure.
	var reg struct {
		Items []struct {
			Items []struct {
				CatalogEntry struct {
					Version string `json:"version"`
				} `json:"catalogEntry"`
			} `json:"items"`
		} `json:"items"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&reg); err != nil {
		return nil, "", fmt.Errorf("parse NuGet response: %w", err)
	}

	var versions []string
	canonical := pkg
	for _, page := range reg.Items {
		for _, item := range page.Items {
			v := item.CatalogEntry.Version
			if v != "" {
				versions = append(versions, v)
			}
		}
	}

	if len(versions) == 0 {
		return nil, "", fmt.Errorf("no versions found for %s", pkg)
	}

	// Sort descending.
	sort.Slice(versions, func(i, j int) bool {
		return versionCmp(versions[i], versions[j]) > 0
	})

	return versions, canonical, nil
}

// versionCmp compares two dotted version strings.
func versionCmp(a, b string) int {
	pa := strings.Split(a, ".")
	pb := strings.Split(b, ".")
	for i := 0; i < len(pa) || i < len(pb); i++ {
		var va, vb int
		if i < len(pa) {
			fmt.Sscanf(pa[i], "%d", &va)
		}
		if i < len(pb) {
			fmt.Sscanf(pb[i], "%d", &vb)
		}
		if va != vb {
			return va - vb
		}
	}
	return 0
}

// nuGetRegistrationBase fetches the RegistrationBaseUrl from the NuGet service index.
func nuGetRegistrationBase() (string, error) {
	resp, err := http.Get(nuGetServiceIndex)
	if err != nil {
		return "", fmt.Errorf("fetch NuGet service index: %w", err)
	}
	defer resp.Body.Close()

	var idx struct {
		Resources []struct {
			Type    string `json:"@type"`
			ID      string `json:"@id"`
		} `json:"resources"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&idx); err != nil {
		return "", fmt.Errorf("parse NuGet service index: %w", err)
	}

	for _, r := range idx.Resources {
		if strings.Contains(r.Type, "RegistrationsBaseUrl") {
			return r.ID, nil
		}
	}

	return "", fmt.Errorf("RegistrationsBaseUrl not found in NuGet service index")
}
