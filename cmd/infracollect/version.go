package main

import (
	"context"
	"fmt"
	"runtime/debug"

	"github.com/urfave/cli/v3"
)

// Build information populated at init() from debug.ReadBuildInfo().
var (
	Version   = "unknown"
	GoVersion = "unknown"
	Commit    = "unknown"
	BuildTime = "unknown"
	Modified  bool
)

func init() {
	parseBuildInfo()
}

func parseBuildInfo() {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}

	Version = info.Main.Version
	GoVersion = info.GoVersion

	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			Commit = setting.Value
		case "vcs.time":
			BuildTime = setting.Value
		case "vcs.modified":
			Modified = setting.Value == "true"
		}
	}
}

var versionCommand = &cli.Command{
	Name:  "version",
	Usage: "Print version information",
	Action: func(ctx context.Context, command *cli.Command) error {
		fmt.Printf("version: %s\n", Version)
		fmt.Printf("go: %s\n", GoVersion)
		if Commit != "unknown" {
			if Modified {
				fmt.Printf("commit: %s (dirty)\n", Commit)
			} else {
				fmt.Printf("commit: %s\n", Commit)
			}
		}
		if BuildTime != "unknown" {
			fmt.Printf("built: %s\n", BuildTime)
		}
		return nil
	},
}
