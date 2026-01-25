// Package main is the entry point for feather-cli.
package main

import (
	"os"

	"github.com/feather-store/feather/cmd/feather-cli/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
