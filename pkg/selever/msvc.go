package selever

import (
	"fmt"
	"os"

	"github.com/orkward/selever/pkg/install"

	"github.com/spf13/cobra"
)

var (
	msvcHost     string
	msvcTarget   string
	winsdkHost   string
	winsdkTarget string
)

var msvcCmd = &cobra.Command{
	Use:   "msvc <version> [--host=<arch>] [--target=<arch>]",
	Short: "Install an exact MSVC toolset (Windows only)",
	Long: `Install an exact MSVC toolset and print shell environment code.

The version is the toolset version of the Visual Studio package, such as
14.44.17.14 or 14.51. The host defaults to the native architecture and the
target to the host. The Windows SDK is installed separately with "winsdk".`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		env, err := install.InstallMSVC(cmd.Context(), args[0], msvcHost, msvcTarget)
		if err != nil {
			fmt.Fprintf(os.Stderr, "selever msvc: %v\n", err)
			os.Exit(1)
		}
		printEnv(env)
	},
}

var winsdkCmd = &cobra.Command{
	Use:   "winsdk <build> [--host=<arch>] [--target=<arch>]",
	Short: "Install an exact Windows SDK (Windows only)",
	Long: `Install an exact Windows SDK and print shell environment code.

The build is the SDK build number, such as 26100. The host selects the SDK
tools (rc, mt, ...) and the target the import libraries. The host defaults to
the native architecture and the target to the host.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		env, err := install.InstallWinSDK(cmd.Context(), args[0], winsdkHost, winsdkTarget)
		if err != nil {
			fmt.Fprintf(os.Stderr, "selever winsdk: %v\n", err)
			os.Exit(1)
		}
		printEnv(env)
	},
}

func init() {
	msvcCmd.Flags().StringVar(&msvcHost, "host", "", "host architecture: x64, x86, arm64 (default native)")
	msvcCmd.Flags().StringVar(&msvcTarget, "target", "", "target architecture: x64, x86, arm, arm64 (default host)")
	winsdkCmd.Flags().StringVar(&winsdkHost, "host", "", "host architecture: x64, x86, arm64 (default native)")
	winsdkCmd.Flags().StringVar(&winsdkTarget, "target", "", "target architecture: x64, x86, arm, arm64 (default host)")
}
