package main

import (
	"os"

	"agent-collab/src/interface/cli"
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
