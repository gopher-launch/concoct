// Package buildinfo describes the executable that is currently running.
package buildinfo

import (
	"fmt"
	"regexp"
	"runtime/debug"
)

// These values are set only by the release build. Ordinary builds deliberately
// retain their development identity.
var (
	Version  = "development"
	Revision = "unknown"
	Dirty    = "unknown"
	Release  = "false"
)

type Info struct {
	Version, Revision, Dirty string
	Official                 bool
}

var semver = regexp.MustCompile(`^v(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)(?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$`)
var revision = regexp.MustCompile(`^[0-9a-f]{7,64}$`)

func Current() Info {
	i := Info{Version: Version, Revision: Revision, Dirty: Dirty}
	if i.Revision == "unknown" {
		if b, ok := debug.ReadBuildInfo(); ok {
			for _, s := range b.Settings {
				switch s.Key {
				case "vcs.revision":
					i.Revision = s.Value
				case "vcs.modified":
					i.Dirty = s.Value
				}
			}
		}
	}
	i.Official = Release == "true" && semver.MatchString(i.Version) && revision.MatchString(i.Revision) && i.Dirty == "false"
	if !i.Official {
		i.Version = "development"
	}
	return i
}

func (i Info) Provenance() string {
	return fmt.Sprintf("%s (%s; embedded Concoct defaults)", i.Version, i.Revision)
}
func (i Info) String() string {
	return fmt.Sprintf("Concoct %s\nRevision: %s\nModified: %s\nClassification: %s\n", i.Version, i.Revision, i.Dirty, map[bool]string{true: "official release", false: "development build"}[i.Official])
}
