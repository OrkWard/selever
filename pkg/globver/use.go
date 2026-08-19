package globver

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/orkward/selever/pkg/fetchver"
	"github.com/orkward/selever/pkg/install"

	"github.com/spf13/cobra"
)

var useCmd = &cobra.Command{
	Use:   "use",
	Short: "Install and expose executables",
	Long:  `Resolve, install, and create launchers for a selection.`,
}

var (
	useNodeCmd = &cobra.Command{
		Use:   "node <version>",
		Short: "Install a Node.js release and expose its executables",
		Args:  cobra.ExactArgs(1),
		Run:   runUseNode,
	}

	useGoCmd = &cobra.Command{
		Use:   "go <version>",
		Short: "Install a Go release and expose its executables",
		Args:  cobra.ExactArgs(1),
		Run:   runUseGo,
	}

	useNpmCmd = &cobra.Command{
		Use:   "npm <package[@version]>",
		Short: "Install an npm package and expose its executables",
		Args:  cobra.ExactArgs(1),
		Run:   runUseNpm,
	}

	useGopkgCmd = &cobra.Command{
		Use:   "gopkg <package[@version]>",
		Short: "Install a Go package and expose its executables",
		Args:  cobra.ExactArgs(1),
		Run:   runUseGopkg,
	}

	useNpmNode string
	useGopkgGo string
)

func init() {
	useNpmCmd.Flags().StringVar(&useNpmNode, "node", "", "Node.js version")
	useGopkgCmd.Flags().StringVar(&useGopkgGo, "go", "", "Go version")

	useCmd.AddCommand(useNodeCmd)
	useCmd.AddCommand(useGoCmd)
	useCmd.AddCommand(useNpmCmd)
	useCmd.AddCommand(useGopkgCmd)
}

func runUseNode(cmd *cobra.Command, args []string) {
	query := strings.TrimPrefix(args[0], "v")
	version := query
	exact := false

	// If the version looks exact (3-part numeric), use it directly.
	if isExactVersion(query) {
		exact = true
	} else {
		var err error
		version, err = fetchver.ResolveNode(query)
		if err != nil {
			fmt.Fprintf(os.Stderr, "globver use node: %v\n", err)
			os.Exit(1)
		}
	}

	dir, err := install.InstallNode(context.Background(), version)
	if err != nil {
		fmt.Fprintf(os.Stderr, "globver use node: %v\n", err)
		os.Exit(1)
	}

	// Discover executables.
	exes, err := listBinDir(filepath.Join(dir, "bin"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "globver use node: %v\n", err)
		os.Exit(1)
	}

	selector := []string{"node", version}
	if err := registerAndShim(exes, selector, []string{filepath.Join(dir, "bin")}); err != nil {
		fmt.Fprintf(os.Stderr, "globver use node: %v\n", err)
		os.Exit(1)
	}

	printUseResult(selector, version, exes, exact)
}

func runUseGo(cmd *cobra.Command, args []string) {
	version := strings.TrimPrefix(args[0], "go")

	dir, err := install.InstallGo(context.Background(), version)
	if err != nil {
		fmt.Fprintf(os.Stderr, "globver use go: %v\n", err)
		os.Exit(1)
	}

	exes, err := listBinDir(filepath.Join(dir, "bin"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "globver use go: %v\n", err)
		os.Exit(1)
	}

	selector := []string{"go", version}
	if err := registerAndShim(exes, selector, []string{filepath.Join(dir, "bin")}); err != nil {
		fmt.Fprintf(os.Stderr, "globver use go: %v\n", err)
		os.Exit(1)
	}

	printUseResult(selector, version, exes, true)
}

func runUseNpm(cmd *cobra.Command, args []string) {
	spec := args[0]
	pkg, query := splitSpec(spec)

	if useNpmNode == "" {
		useNpmNode = resolveParentNode()
	}

	pkgVersion, err := fetchver.ResolveNpmVersion(pkg, query)
	if err != nil {
		fmt.Fprintf(os.Stderr, "globver use npm: %v\n", err)
		os.Exit(1)
	}

	_, binDir, err := install.InstallNpm(context.Background(), pkg, pkgVersion, useNpmNode)
	if err != nil {
		fmt.Fprintf(os.Stderr, "globver use npm: %v\n", err)
		os.Exit(1)
	}

	exes, err := listBinDir(binDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "globver use npm: %v\n", err)
		os.Exit(1)
	}

	// Record the resolved version, not the raw query (e.g. @latest).
	selector := []string{"npm", pkg + "@" + pkgVersion, "--node", useNpmNode}
	if err := registerAndShim(exes, selector, []string{binDir}); err != nil {
		fmt.Fprintf(os.Stderr, "globver use npm: %v\n", err)
		os.Exit(1)
	}

	printUseResult(selector, pkgVersion, exes, false)
}

func runUseGopkg(cmd *cobra.Command, args []string) {
	spec := args[0]
	pkg, query := splitSpec(spec)

	if useGopkgGo == "" {
		useGopkgGo = resolveParentGo()
	}

	pkgVersion, err := fetchver.ResolveGopkgVersion(pkg, query)
	if err != nil {
		fmt.Fprintf(os.Stderr, "globver use gopkg: %v\n", err)
		os.Exit(1)
	}

	_, binDir, err := install.InstallGopkg(context.Background(), pkg, pkgVersion, useGopkgGo)
	if err != nil {
		fmt.Fprintf(os.Stderr, "globver use gopkg: %v\n", err)
		os.Exit(1)
	}

	exes, err := listBinDir(binDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "globver use gopkg: %v\n", err)
		os.Exit(1)
	}

	// Record the resolved version, not the raw query (e.g. @latest).
	selector := []string{"gopkg", pkg + "@" + pkgVersion, "--go", useGopkgGo}
	if err := registerAndShim(exes, selector, []string{binDir}); err != nil {
		fmt.Fprintf(os.Stderr, "globver use gopkg: %v\n", err)
		os.Exit(1)
	}

	printUseResult(selector, pkgVersion, exes, false)
}

// --- helpers ---

// listBinDir returns sorted names of regular files in a directory.
func listBinDir(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.Type().IsRegular() || e.Type()&os.ModeSymlink != 0 {
			names = append(names, e.Name())
		}
	}
	// Sort for deterministic output.
	// (no sort needed for small lists, but good practice)
	return names, nil
}

// registerAndShim updates the config and creates shims for executables.
func registerAndShim(exes []string, selector []string, binDirs []string) error {
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}

	for _, exe := range exes {
		if err := CreateShim(exe, binDirs); err != nil {
			return err
		}
		cfg[exe] = selector
	}

	// Remove config entries for executables no longer present.
	for exe, sel := range cfg {
		if slicesEqual(sel, selector) && slices.Contains(exes, exe) {
			if err := RemoveShim(exe); err != nil {
				return err
			}
			delete(cfg, exe)
		}
	}

	return SaveConfig(cfg)
}

// resolveParentNode returns a Node.js version from interactive selection (when
// stdin is a terminal and local installations exist), or from the recorded
// global selection, or exits with an error.
func resolveParentNode() string {
	// If on a terminal, try interactive selection from installed versions first.
	if isTerminal(os.Stdin) {
		versions := listInstalledVersions("node")
		if len(versions) > 0 {
			return promptSelectVersion("Node.js", versions)
		}
	}

	// Fall back to recorded global selection.
	cfg, _ := LoadConfig()
	for _, sel := range cfg {
		if len(sel) >= 2 && sel[0] == "node" {
			return sel[1]
		}
	}
	fmt.Fprintln(os.Stderr, "globver: no local or recorded Node.js selection; use --node")
	os.Exit(1)
	return ""
}

// resolveParentGo returns a Go version from interactive selection (when stdin
// is a terminal and local installations exist), or from the recorded global
// selection, or exits with an error.
func resolveParentGo() string {
	if isTerminal(os.Stdin) {
		versions := listInstalledVersions("go")
		if len(versions) > 0 {
			return promptSelectVersion("Go", versions)
		}
	}

	cfg, _ := LoadConfig()
	for _, sel := range cfg {
		if len(sel) >= 2 && sel[0] == "go" {
			return sel[1]
		}
	}
	fmt.Fprintln(os.Stderr, "globver: no local or recorded Go selection; use --go")
	os.Exit(1)
	return ""
}

func printUseResult(selector []string, resolvedVersion string, exes []string, exact bool) {
	selStr := strings.Join(selector, " ")
	if !exact {
		// Replace the version in the selector with the resolved version.
		selStr = strings.Join(resolveSelector(selector, resolvedVersion), " ")
	}
	fmt.Fprintf(os.Stderr, "selection: %s\n", selStr)
	fmt.Fprintf(os.Stderr, "executables: %s\n", strings.Join(exes, ", "))
}

func resolveSelector(sel []string, resolved string) []string {
	out := make([]string, len(sel))
	copy(out, sel)
	if len(out) < 2 {
		return out
	}

	switch sel[0] {
	case "node", "go":
		// sel[1] is the version directly.
		out[1] = resolved
	case "npm", "gopkg":
		// sel[1] is "pkg@query" — replace only the version part.
		if i := strings.LastIndex(out[1], "@"); i >= 0 {
			out[1] = out[1][:i+1] + resolved
		}
	}
	return out
}

func isExactVersion(v string) bool {
	parts := strings.Split(v, ".")
	return len(parts) == 3 && allNumeric(parts)
}

func allNumeric(parts []string) bool {
	for _, p := range parts {
		for _, c := range p {
			if c < '0' || c > '9' {
				return false
			}
		}
	}
	return true
}

func splitSpec(spec string) (pkg, version string) {
	// Skip a leading "@" that starts a scope (e.g. @scope/pkg has no version).
	if i := strings.LastIndex(spec, "@"); i > 0 {
		return spec[:i], spec[i+1:]
	}
	return spec, "latest"
}

func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// isTerminal reports whether f refers to a terminal.
func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// listInstalledVersions returns locally installed versions of a tool,
// sorted newest first.
func listInstalledVersions(tool string) []string {
	p, err := install.ResolvePaths()
	if err != nil {
		return nil
	}
	toolDir := filepath.Join(p.Data, tool)
	entries, err := os.ReadDir(toolDir)
	if err != nil {
		return nil
	}

	var versions []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		binDir := filepath.Join(toolDir, e.Name(), "bin")
		if info, err := os.Stat(binDir); err == nil && info.IsDir() {
			versions = append(versions, e.Name())
		}
	}

	sort.Slice(versions, func(i, j int) bool {
		return compareVersions(versions[i], versions[j]) > 0
	})

	return versions
}

// compareVersions compares two dotted version strings.
// Returns negative if a < b, 0 if equal, positive if a > b.
func compareVersions(a, b string) int {
	pa := strings.Split(a, ".")
	pb := strings.Split(b, ".")
	for i := 0; i < len(pa) || i < len(pb); i++ {
		var na, nb int
		if i < len(pa) {
			na, _ = strconv.Atoi(pa[i])
		}
		if i < len(pb) {
			nb, _ = strconv.Atoi(pb[i])
		}
		if na != nb {
			return na - nb
		}
	}
	return 0
}

// promptSelectVersion prints a numbered list and reads the user's choice
// from stdin. It re-prompts on invalid input.
func promptSelectVersion(tool string, versions []string) string {
	fmt.Fprintf(os.Stderr, "Available %s versions:\n", tool)
	for i, v := range versions {
		fmt.Fprintf(os.Stderr, "  [%d] %s\n", i+1, v)
	}

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Fprintf(os.Stderr, "Choose a version [1-%d]: ", len(versions))
		if !scanner.Scan() {
			fmt.Fprintln(os.Stderr)
			os.Exit(1)
		}
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}
		n, err := strconv.Atoi(input)
		if err != nil || n < 1 || n > len(versions) {
			fmt.Fprintf(os.Stderr, "Invalid selection: %s\n", input)
			continue
		}
		return versions[n-1]
	}
}
