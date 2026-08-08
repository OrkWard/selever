package globver

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "globver",
	Short: "Manage exact tool versions with global launchers",
	Long: `globver installs selections through Selever and creates launchers in
~/.local/bin. Each launcher uses an executable's native name and
adds the directories required by its recorded selection to PATH.`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(useCmd)
	rootCmd.AddCommand(removeCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(installCmd)
}
