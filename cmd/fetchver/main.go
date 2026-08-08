package main

import (
	"os"

	"git.lan/selever/pkg/fetchver"
)

func main() {
	if err := fetchver.Execute(); err != nil {
		os.Exit(1)
	}
}
