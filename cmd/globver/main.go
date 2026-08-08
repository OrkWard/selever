package main

import (
	"os"

	"git.lan/selever/pkg/globver"
)

func main() {
	if err := globver.Execute(); err != nil {
		os.Exit(1)
	}
}
