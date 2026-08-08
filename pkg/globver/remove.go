package globver

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var removeCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove a selection and its launchers",
	Long:  `Delete every managed launcher owned by the named selection.`,
}

func init() {
	removeCmd.AddCommand(removeNodeCmd)
	removeCmd.AddCommand(removeNpmCmd)
	removeCmd.AddCommand(removeGoCmd)
	removeCmd.AddCommand(removeGopkgCmd)
	removeCmd.AddCommand(removeDotnetCmd)
}

var removeNodeCmd = &cobra.Command{
	Use:   "node",
	Short: "Remove the recorded Node.js selection",
	Args:  cobra.NoArgs,
	Run:   func(cmd *cobra.Command, args []string) { removeByTool("node", "") },
}

var removeNpmCmd = &cobra.Command{
	Use:   "npm <package>",
	Short: "Remove a recorded npm package selection",
	Args:  cobra.ExactArgs(1),
	Run:   func(cmd *cobra.Command, args []string) { removeByTool("npm", args[0]) },
}

var removeGoCmd = &cobra.Command{
	Use:   "go",
	Short: "Remove the recorded Go selection",
	Args:  cobra.NoArgs,
	Run:   func(cmd *cobra.Command, args []string) { removeByTool("go", "") },
}

var removeGopkgCmd = &cobra.Command{
	Use:   "gopkg <package>",
	Short: "Remove a recorded Go package selection",
	Args:  cobra.ExactArgs(1),
	Run:   func(cmd *cobra.Command, args []string) { removeByTool("gopkg", args[0]) },
}

var removeDotnetCmd = &cobra.Command{
	Use:   "dotnet <package>",
	Short: "Remove a recorded dotnet tool selection",
	Args:  cobra.ExactArgs(1),
	Run:   func(cmd *cobra.Command, args []string) { removeByTool("dotnet", args[0]) },
}

func removeByTool(tool, pkg string) {
	cfg, err := LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "globver remove: %v\n", err)
		os.Exit(1)
	}

	removed := 0
	for exe, sel := range cfg {
		if matchSelector(sel, tool, pkg) {
			if err := RemoveShim(exe); err != nil {
				fmt.Fprintf(os.Stderr, "globver remove: %v\n", err)
				os.Exit(1)
			}
			delete(cfg, exe)
			removed++
		}
	}

	if removed == 0 {
		fmt.Fprintf(os.Stderr, "globver remove: no selection found for %s", tool)
		if pkg != "" {
			fmt.Fprintf(os.Stderr, " %s", pkg)
		}
		fmt.Fprintln(os.Stderr)
		os.Exit(1)
	}

	if err := SaveConfig(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "globver remove: %v\n", err)
		os.Exit(1)
	}
}

// matchSelector checks if a recorded selector matches the tool and optional
// package name.
func matchSelector(sel []string, tool, pkg string) bool {
	if len(sel) == 0 || sel[0] != tool {
		return false
	}
	if pkg == "" {
		return true
	}
	// For package tools, check the package field.
	for _, s := range sel[1:] {
		if !strings.HasPrefix(s, "--") {
			if strings.HasPrefix(s, pkg+"@") || s == pkg {
				return true
			}
			break
		}
	}
	return false
}
