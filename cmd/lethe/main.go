package main

import (
	"os"

	"github.com/lethe/lethe/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
