package main

import (
	"os"

	"github.com/orkward/selever/pkg/fetchver"
)

func main() {
	if err := fetchver.Execute(); err != nil {
		os.Exit(1)
	}
}
