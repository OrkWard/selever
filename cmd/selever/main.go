package main

import (
	"os"

	"github.com/orkward/selever/pkg/selever"
)

func main() {
	if err := selever.Execute(); err != nil {
		os.Exit(1)
	}
}
