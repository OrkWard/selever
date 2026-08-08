package main

import (
	"os"

	"git.lan/selever/pkg/selever"
)

func main() {
	if err := selever.Execute(); err != nil {
		os.Exit(1)
	}
}
