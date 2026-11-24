// Package buildinfo exposes public artifact identity without environment values.
package buildinfo

import (
	"runtime"
	"runtime/debug"
)

// Set by the image builder. Local builds remain explicitly distinguishable.
var Version = "development"
var Revision = "unknown"

const MetadataContract = 1

type Info struct {
	Version          string `json:"version"`
	Revision         string `json:"revision"`
	MetadataContract int    `json:"metadata_contract"`
	OS               string `json:"os"`
	Architecture     string `json:"architecture"`
	GoVersion        string `json:"go_version"`
}

func Current() Info {
	revision := Revision
	if revision == "unknown" {
		if b, ok := debug.ReadBuildInfo(); ok {
			for _, setting := range b.Settings {
				if setting.Key == "vcs.revision" {
					revision = setting.Value
				}
			}
		}
	}
	return Info{Version, revision, MetadataContract, runtime.GOOS, runtime.GOARCH, runtime.Version()}
}
