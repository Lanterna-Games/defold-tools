package cli

import (
	"runtime/debug"
	"testing"
)

func TestResolveVersion(t *testing.T) {
	buildInfo := func(moduleVersion string) func() (*debug.BuildInfo, bool) {
		return func() (*debug.BuildInfo, bool) {
			return &debug.BuildInfo{Main: debug.Module{Version: moduleVersion}}, true
		}
	}
	noBuildInfo := func() (*debug.BuildInfo, bool) { return nil, false }

	tests := []struct {
		name          string
		setByLinker   string
		readBuildInfo func() (*debug.BuildInfo, bool)
		want          string
	}{
		// A release build stamps the tag through -ldflags; that wins over everything.
		{name: "release build", setByLinker: "1.2.3", readBuildInfo: buildInfo("v9.9.9"), want: "1.2.3"},
		// `go install module@v1.2.3` has no -ldflags, but the module version is recorded.
		{name: "go install of a tagged version", readBuildInfo: buildInfo("v1.2.3"), want: "v1.2.3"},
		// Without version control information, as under `go test`, Go records "(devel)".
		{name: "build without version control information", readBuildInfo: buildInfo("(devel)"), want: "dev"},
		{name: "binary without build info", readBuildInfo: noBuildInfo, want: "dev"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveVersion(tt.setByLinker, tt.readBuildInfo); got != tt.want {
				t.Errorf("resolveVersion() = %q, want %q", got, tt.want)
			}
		})
	}
}
