package fetchver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

var npmCmd = &cobra.Command{
	Use:   "npm <package[@version]>",
	Short: "Resolve an npm package version",
	Long: `Resolve an npm package from its configured registry.

The optional version may be a 1/2/3-part numeric prefix or a dist-tag.
Omitting it is equivalent to @latest.`,
	Args: cobra.ExactArgs(1),
	Run:  runFetchNpm,
}

func runFetchNpm(cmd *cobra.Command, args []string) {
	spec := args[0]
	pkg, query := parseSpec(spec)

	result, err := resolveNpm(pkg, query)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fetchver npm: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(result)
}

// parseSpec splits "pkg@version" or returns pkg with empty version.
func parseSpec(spec string) (pkg, version string) {
	if i := strings.LastIndex(spec, "@"); i >= 0 {
		return spec[:i], spec[i+1:]
	}
	return spec, "latest"
}

// resolveNpm resolves an npm package version from the registry.
func resolveNpm(pkg, query string) (string, error) {
	registry, err := npmRegistry(pkg)
	if err != nil {
		return "", err
	}

	// Try dist-tags first.
	dt, err := fetchDistTags(registry, pkg)
	if err != nil {
		return "", fmt.Errorf("fetch dist-tags: %w", err)
	}

	// Check if query is a dist-tag.
	if dt != nil {
		if exact, ok := dt[query]; ok {
			return pkg + "@" + exact, nil
		}
	}

	// Try prefix match against all versions.
	versions, err := fetchPackageVersions(registry, pkg)
	if err != nil {
		return "", fmt.Errorf("fetch versions: %w", err)
	}

	// Sort descending.
	sort.Slice(versions, func(i, j int) bool {
		return versionCmp(versions[i], versions[j]) > 0
	})

	// Prefix match.
	for _, v := range versions {
		if strings.HasPrefix(v, query) {
			return pkg + "@" + v, nil
		}
	}

	return "", fmt.Errorf("no matching version for %s@%s", pkg, query)
}

// npmRegistry returns the registry URL for a package.
func npmRegistry(pkg string) (string, error) {
	// Check scoped registry.
	if strings.HasPrefix(pkg, "@") {
		scope := strings.SplitN(pkg, "/", 2)[0]
		rc := loadNpmrc()
		if r, ok := rc[scope+":registry"]; ok {
			return r, nil
		}
	}

	// Env var.
	if r := os.Getenv("npm_config_registry"); r != "" {
		return r, nil
	}
	if r := os.Getenv("NPM_CONFIG_REGISTRY"); r != "" {
		return r, nil
	}

	// npmrc registry setting.
	rc := loadNpmrc()
	if r, ok := rc["registry"]; ok {
		return r, nil
	}

	return "https://registry.npmjs.org", nil
}

// loadNpmrc loads npm configuration from user and project .npmrc files.
func loadNpmrc() map[string]string {
	config := make(map[string]string)

	// User config.
	userRc := npmUserConfig()
	if data, err := os.ReadFile(userRc); err == nil {
		parseIniLike(config, string(data))
	}

	// Project config.
	if data, err := os.ReadFile(".npmrc"); err == nil {
		parseIniLike(config, string(data))
	}

	return config
}

// npmUserConfig returns the path to the user's npm config.
func npmUserConfig() string {
	if p := os.Getenv("npm_config_userconfig"); p != "" {
		return p
	}
	if p := os.Getenv("NPM_CONFIG_USERCONFIG"); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".npmrc")
}

// parseIniLike parses simple key=value lines into m.
func parseIniLike(m map[string]string, data string) {
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		m[key] = os.ExpandEnv(val)
	}
}

// distTagsResponse is the npm registry dist-tags response.
type distTagsResponse map[string]string

// packageResponse is the abbreviated npm package metadata response.
type packageResponse struct {
	Versions map[string]any `json:"versions"`
}

func fetchDistTags(registry, pkg string) (distTagsResponse, error) {
	url := fmt.Sprintf("%s/-/package/%s/dist-tags", strings.TrimRight(registry, "/"), pkg)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil // no dist-tags, fall through to version list
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}

	var dt distTagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&dt); err != nil {
		return nil, err
	}
	return dt, nil
}

func fetchPackageVersions(registry, pkg string) ([]string, error) {
	url := fmt.Sprintf("%s/%s", strings.TrimRight(registry, "/"), pkg)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}

	var p packageResponse
	if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
		return nil, err
	}

	var versions []string
	for v := range p.Versions {
		versions = append(versions, v)
	}
	return versions, nil
}
