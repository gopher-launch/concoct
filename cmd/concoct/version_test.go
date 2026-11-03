package main

import (
	"runtime/debug"
	"testing"
)

func TestFormatVersion(t *testing.T) {
	tests := []struct {
		name      string
		base      string
		info      *debug.BuildInfo
		available bool
		want      string
	}{
		{
			name: "clean development build",
			base: "0.6.0-dev",
			info: &debug.BuildInfo{Settings: []debug.BuildSetting{
				{Key: "vcs.revision", Value: "819a7b1234567890abcdef"},
				{Key: "vcs.modified", Value: "false"},
			}},
			available: true,
			want:      "0.6.0-dev+819a7b123456",
		},
		{
			name: "modified development build",
			base: "0.6.0-dev",
			info: &debug.BuildInfo{Settings: []debug.BuildSetting{
				{Key: "vcs.revision", Value: "819a7b1234567890abcdef"},
				{Key: "vcs.modified", Value: "true"},
			}},
			available: true,
			want:      "0.6.0-dev+819a7b123456.dirty",
		},
		{
			name:      "development build without metadata",
			base:      "0.6.0-dev",
			available: false,
			want:      "0.6.0-dev+unknown",
		},
		{
			name:      "development build without revision",
			base:      "0.6.0-dev",
			info:      &debug.BuildInfo{},
			available: true,
			want:      "0.6.0-dev+unknown",
		},
		{
			name: "release build",
			base: "0.6.0",
			info: &debug.BuildInfo{Settings: []debug.BuildSetting{
				{Key: "vcs.revision", Value: "819a7b1234567890abcdef"},
				{Key: "vcs.modified", Value: "true"},
			}},
			available: true,
			want:      "0.6.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatVersion(tt.base, tt.info, tt.available); got != tt.want {
				t.Fatalf("formatVersion() = %q, want %q", got, tt.want)
			}
		})
	}
}
