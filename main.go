package main

import (
	"os"

	"github.com/devriles/xpctl/cmd"
)

var version = "dev" // injected via -ldflags

func main() {
	cmd.SetVersion(version)
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
