package main

import (
	"os"

	"github.com/mmmnt/flmnt-cli/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
