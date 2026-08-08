package globver

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List recorded selections",
	Long: `Write a table of exact recorded selections and their executable names
to standard output.`,
	Args: cobra.NoArgs,
	Run:  runList,
}

func runList(cmd *cobra.Command, args []string) {
	cfg, err := LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "globver list: %v\n", err)
		os.Exit(1)
	}

	if len(cfg) == 0 {
		fmt.Println("No selections recorded.")
		return
	}

	// Group executables by selector.
	type group struct {
		selector string
		exes     []string
	}

	seen := make(map[string]*group)
	for exe, sel := range cfg {
		key := strings.Join(sel, " ")
		if g, ok := seen[key]; ok {
			g.exes = append(g.exes, exe)
		} else {
			g := &group{selector: key, exes: []string{exe}}
			seen[key] = g
		}
	}

	// Sort groups by selector.
	var groups []*group
	for _, g := range seen {
		sort.Strings(g.exes)
		groups = append(groups, g)
	}
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].selector < groups[j].selector
	})

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "SELECTION\tEXECUTABLES")
	for _, g := range groups {
		fmt.Fprintf(w, "%s\t%s\n", g.selector, strings.Join(g.exes, ", "))
	}
	w.Flush()
}
