package main

import (
	"os"

	"github.com/MrWong99/zhi/internal/cli"
)

// version, commit, and date are set at build time via ldflags.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	cli.SetVersionInfo(version, commit, date)
	if err := cli.Execute(); err != nil {
		if code := cli.ExitCode(err); code != 0 {
			os.Exit(code)
		}
		os.Exit(1)
	}
}
