package cli

import "runtime/debug"

const (
	versionOfLocalBuild       = "dev"
	moduleVersionOfLocalBuild = "(devel)"
)

var versionSetByLinker string

func resolveVersion(setByLinker string, readBuildInfo func() (*debug.BuildInfo, bool)) string {
	if setByLinker != "" {
		return setByLinker
	}
	if info, ok := readBuildInfo(); ok && isReleasedModuleVersion(info.Main.Version) {
		return info.Main.Version
	}
	return versionOfLocalBuild
}

func isReleasedModuleVersion(version string) bool {
	return version != "" && version != moduleVersionOfLocalBuild
}
