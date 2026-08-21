package cli

import (
	godebug "runtime/debug"
	"strings"
	"sync"
)

var (
	versionOnce  sync.Once
	buildVersion string
)

// resolvedVersion returns the ldflags-injected version, falling back to
// the Go module build info so binaries installed via
// `go install ...@vX.Y.Z` report their real version instead of "dev".
func resolvedVersion() string {
	versionOnce.Do(func() {
		if version != "" && version != "dev" {
			buildVersion = version
			return
		}
		buildVersion = "dev"
		if info, ok := godebug.ReadBuildInfo(); ok {
			v := info.Main.Version
			if v != "" && v != "(devel)" {
				buildVersion = strings.TrimPrefix(v, "v")
			}
		}
	})
	return buildVersion
}
