package main

import (
	"os"

	"github.com/zyrophix/lethe/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
