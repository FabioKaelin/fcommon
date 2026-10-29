// Package buildinfo provides immutable release identity without editing source.
package buildinfo

import (
	"os"
	"runtime/debug"
)

var Version = "dev"
var Commit = "unknown"
var BuiltAt = "unknown"

type Info struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	BuiltAt string `json:"built_at"`
	Dirty   bool   `json:"dirty"`
}

func Current(fallback string) Info {
	info := Info{Version: Version, Commit: Commit, BuiltAt: BuiltAt}
	if info.Version == "dev" && fallback != "" {
		info.Version = fallback
	}
	if v := os.Getenv("F_VERSION"); v != "" && Version == "dev" {
		info.Version = v
	}
	if v := os.Getenv("F_COMMIT"); v != "" {
		info.Commit = v
	}
	if v := os.Getenv("F_BUILD_TIME"); v != "" {
		info.BuiltAt = v
	}
	if built, ok := debug.ReadBuildInfo(); ok {
		for _, s := range built.Settings {
			switch s.Key {
			case "vcs.revision":
				if info.Commit == "unknown" {
					info.Commit = s.Value
				}
			case "vcs.time":
				if info.BuiltAt == "unknown" {
					info.BuiltAt = s.Value
				}
			case "vcs.modified":
				info.Dirty = s.Value == "true"
			}
		}
	}
	return info
}
