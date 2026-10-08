package fetchver

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/orkward/selever/pkg/install"

	"github.com/spf13/cobra"
)

// vsManifestMaxAge is how long fetchver trusts a cached Visual Studio channel.
const vsManifestMaxAge = time.Hour

var (
	msvcHost   string
	msvcTarget string
)

var msvcCmd = &cobra.Command{
	Use:   "msvc <version> [--host=<arch>] [--target=<arch>]",
	Short: "Resolve an MSVC toolset version",
	Long: `Resolve an MSVC toolset through the Visual Studio manifest.

<version> may be "latest", a prefix of the toolset version on a dot boundary
(14, 14.44, 14.44.17.14), or a build version such as 14.44.35229. Only
toolsets with a compiler for the host and target are considered; the host
defaults to the native architecture and the target to the host.`,
	Args: cobra.ExactArgs(1),
	Run:  runFetchMSVC,
}

var winsdkCmd = &cobra.Command{
	Use:   "winsdk <version>",
	Short: "Resolve a Windows SDK build number",
	Long: `Resolve a Windows SDK through the Visual Studio manifest.

<version> may be "latest", a build number such as 26100, or a full version
such as 10.0.26100.0. The result is the build number.`,
	Args: cobra.ExactArgs(1),
	Run:  runFetchWinSDK,
}

func init() {
	msvcCmd.Flags().StringVar(&msvcHost, "host", "", "host architecture: x64, x86, arm64 (default native)")
	msvcCmd.Flags().StringVar(&msvcTarget, "target", "", "target architecture: x64, x86, arm, arm64 (default host)")
	rootCmd.AddCommand(msvcCmd)
	rootCmd.AddCommand(winsdkCmd)
}

func runFetchMSVC(cmd *cobra.Command, args []string) {
	result, err := resolveMSVC(args[0], msvcHost, msvcTarget)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fetchver msvc: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(result)
}

func runFetchWinSDK(cmd *cobra.Command, args []string) {
	result, err := resolveWinSDK(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "fetchver winsdk: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(result)
}

func resolveMSVC(query, host, target string) (string, error) {
	host, target, err := install.CheckVSArch(host, target)
	if err != nil {
		return "", err
	}
	m, err := install.LoadVSManifest(vsManifestMaxAge)
	if err != nil {
		return "", err
	}
	versions := m.MSVCVersions(host, target)
	if len(versions) == 0 {
		return "", fmt.Errorf("no MSVC toolset for host %s, target %s", host, target)
	}

	query = strings.ToLower(query)
	if query == "latest" {
		return versions[0].Version, nil
	}
	for _, v := range versions {
		if dotPrefix(v.Version, query) || dotPrefix(v.Build, query) {
			return v.Version, nil
		}
	}
	return "", fmt.Errorf("no MSVC toolset matches %q for host %s, target %s", query, host, target)
}

func resolveWinSDK(query string) (string, error) {
	m, err := install.LoadVSManifest(vsManifestMaxAge)
	if err != nil {
		return "", err
	}
	sdks := m.WinSDKs()
	if len(sdks) == 0 {
		return "", fmt.Errorf("no Windows SDK in the Visual Studio manifest")
	}

	query = strings.ToLower(query)
	if query == "latest" {
		return sdks[0].Build, nil
	}
	// 10.0.26100.0 -> 26100
	build := query
	if parts := strings.Split(query, "."); len(parts) >= 3 && parts[0] == "10" && parts[1] == "0" {
		build = parts[2]
	}
	for _, s := range sdks {
		if s.Build == build {
			return s.Build, nil
		}
	}
	return "", fmt.Errorf("no Windows SDK matches %q (available: %s)", query, sdkBuilds(sdks))
}

func sdkBuilds(sdks []install.WinSDK) string {
	builds := make([]string, len(sdks))
	for i, s := range sdks {
		builds[i] = s.Build
	}
	return strings.Join(builds, ", ")
}

// dotPrefix reports whether q equals v or is a prefix of v ending at a dot.
func dotPrefix(v, q string) bool {
	return v == q || strings.HasPrefix(v, q+".")
}
