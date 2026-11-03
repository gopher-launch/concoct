package main

import (
	"runtime/debug"
	"strings"
)

const (
	baseVersion         = "0.6.0-dev"
	revisionShortLength = 12
)

func currentVersion() string {
	info, ok := debug.ReadBuildInfo()
	return formatVersion(baseVersion, info, ok)
}

func formatVersion(base string, info *debug.BuildInfo, buildInfoAvailable bool) string {
	if !strings.HasSuffix(base, "-dev") {
		return base
	}

	revision := "unknown"
	dirty := false
	if buildInfoAvailable && info != nil {
		for _, setting := range info.Settings {
			switch setting.Key {
			case "vcs.revision":
				if setting.Value != "" {
					revision = setting.Value
					if len(revision) > revisionShortLength {
						revision = revision[:revisionShortLength]
					}
				}
			case "vcs.modified":
				dirty = setting.Value == "true"
			}
		}
	}

	version := base + "+" + revision
	if dirty {
		version += ".dirty"
	}
	return version
}
