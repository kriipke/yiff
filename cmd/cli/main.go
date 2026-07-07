package main

import (
	"github.com/kriipke/driftmap/internal/adapters/cli"
	"os"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
