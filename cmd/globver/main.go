package main

import (
	"os"

	"github.com/orkward/selever/pkg/globver"
)

func main() {
	if err := globver.Execute(); err != nil {
		os.Exit(1)
	}
}
