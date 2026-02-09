package main

import (
	"os"

	"agent-collab/internal/interface/cli"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
	builtBy = "unknown"
)

func main() {
	cli.SetVersionInfo(version, commit, date, builtBy)
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}

