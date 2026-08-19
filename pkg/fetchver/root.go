package fetchver

import (
	"github.com/orkward/selever/pkg/config"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "fetchver",
	Short: "Resolve version queries to exact versions",
	Long: `fetchver resolves version prefixes, tags, codenames, or package queries
and writes the matching exact version to standard output.`,
	SilenceUsage: true,
	Version:      config.Version,
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(nodeCmd)
	rootCmd.AddCommand(npmCmd)
	rootCmd.AddCommand(gopkgCmd)
	rootCmd.AddCommand(dotnetCmd)
}
